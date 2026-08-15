package remove

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/relation/shared"
	wishared "github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type removeOptions struct {
	targetArg string

	relationType string   // --relation-type
	targetIDs    []string // --target-id (repeatable; comma-separated also accepted)
	yes          bool     // --yes

	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &removeOptions{}

	cmd := &cobra.Command{
		Use:     "remove [ORG:]PROJECT/ID",
		Aliases: []string{"r", "rm"},
		Short:   "Remove a relation(s) from a work item.",
		Long: heredoc.Doc(`
			Detach one or more relations from an existing work item. The relation
			type must be one of the friendly names returned by 'list-type'.
			Targets are specified by work item ID.
		`),
		Example: heredoc.Doc(`
			# Remove a parent relation to another work item
			azdo boards work-item relation remove Fabrikam/1234 --relation-type parent --target-id 5678 --yes

			# Remove relations to multiple work items
			azdo boards work-item relation remove Fabrikam/1234 --relation-type related --target-id 5678,5679 --yes
		`),
		Args: util.ExactArgs(1, "project/source work item target required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runRemove(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.relationType, "relation-type", "", "Relation type (friendly name, e.g. parent, child, related).")
	cmd.Flags().StringArrayVar(&opts.targetIDs, "target-id", nil, "Target work item ID (repeatable; comma-separated values accepted).")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip the confirmation prompt.")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"id", "rev", "fields", "url", "_links", "relations", "commentVersionRef"})

	return cmd
}

func runRemove(cmdCtx util.CmdContext, opts *removeOptions) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	id, err := strconv.Atoi(scope.Targets[0])
	if err != nil || id <= 0 {
		return util.FlagErrorf("work item ID must be a positive integer; got %q", scope.Targets[0])
	}

	targetIDs, err := shared.SplitAndTrimCSV(opts.targetIDs)
	if err != nil {
		return util.FlagErrorWrap(err)
	}
	if len(targetIDs) == 0 {
		return util.FlagErrorf("--target-id must be provided")
	}
	for _, tid := range targetIDs {
		n, err := strconv.Atoi(tid)
		if err != nil || n <= 0 {
			return util.FlagErrorf("target work item ID must be a positive integer; got %q", tid)
		}
	}

	if !opts.yes {
		if !ios.CanPrompt() {
			return util.FlagErrorf("--yes required when not running interactively")
		}
		ios.StopProgressIndicator()
		prompter, err := cmdCtx.Prompter()
		if err != nil {
			return err
		}
		confirmed, err := prompter.Confirm("Are you sure you want to remove this relation(s)?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
		ios.StartProgressIndicator()
	}

	wit, err := cmdCtx.ClientFactory().WorkItemTracking(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create work item tracking client: %w", err)
	}

	relTypes, err := wit.GetRelationTypes(cmdCtx.Context(), workitemtracking.GetRelationTypesArgs{})
	if err != nil {
		return fmt.Errorf("failed to get relation types: %w", err)
	}

	relRefName, err := shared.ResolveRelationType(relTypes, opts.relationType)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	// Resolve target IDs to URLs.
	targetURLs := make(map[string]struct{}, len(targetIDs))
	for _, tid := range targetIDs {
		n, _ := strconv.Atoi(tid)
		target, err := wit.GetWorkItem(cmdCtx.Context(), workitemtracking.GetWorkItemArgs{
			Project: types.ToPtr(scope.Project),
			Id:      &n,
		})
		if err != nil {
			return fmt.Errorf("failed to resolve target work item %d: %w", n, err)
		}
		if !wishared.BelongsToProject(target, scope.Project) {
			return fmt.Errorf("target work item %d does not belong to project %q", n, scope.Project)
		}
		if target.Url == nil || *target.Url == "" {
			return fmt.Errorf("target work item %d has no URL; cannot remove relation", n)
		}
		targetURLs[*target.Url] = struct{}{}
	}

	// Fetch the source work item to find matching relations.
	expand := workitemtracking.WorkItemExpandValues.All
	src, err := wit.GetWorkItem(cmdCtx.Context(), workitemtracking.GetWorkItemArgs{
		Project: types.ToPtr(scope.Project),
		Id:      &id,
		Expand:  &expand,
	})
	if err != nil {
		return fmt.Errorf("failed to get work item %d: %w", id, err)
	}
	if !wishared.BelongsToProject(src, scope.Project) {
		return fmt.Errorf("work item %d does not belong to project %q", id, scope.Project)
	}

	// Build a list of indices in reverse order (Decision 14).
	indices := []int{}
	if src.Relations != nil {
		for i, rel := range *src.Relations {
			if rel.Rel == nil || rel.Url == nil {
				continue
			}
			if *rel.Rel != relRefName {
				continue
			}
			if _, ok := targetURLs[*rel.Url]; !ok {
				continue
			}
			indices = append(indices, i)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(indices)))

	if len(indices) != len(targetIDs) {
		return util.FlagErrorf("Id(s) supplied in --target-id is not valid")
	}

	remove := webapi.OperationValues.Remove
	doc := []webapi.JsonPatchOperation{}
	for _, idx := range indices {
		p := fmt.Sprintf("/relations/%d", idx)
		doc = append(doc, webapi.JsonPatchOperation{Op: &remove, Path: &p})
	}

	_, err = wit.UpdateWorkItem(cmdCtx.Context(), workitemtracking.UpdateWorkItemArgs{
		Project:  types.ToPtr(scope.Project),
		Document: &doc,
		Id:       &id,
	})
	if err != nil {
		return fmt.Errorf("failed to update work item %d: %w", id, err)
	}

	populated, err := wit.GetWorkItem(cmdCtx.Context(), workitemtracking.GetWorkItemArgs{
		Project: types.ToPtr(scope.Project),
		Id:      &id,
		Expand:  &expand,
	})
	if err != nil {
		return fmt.Errorf("failed to get work item %d: %w", id, err)
	}

	if err := shared.PopulateFriendlyNames(relTypes, populated); err != nil {
		return err
	}

	if opts.exporter != nil {
		return opts.exporter.Write(ios, populated)
	}
	tp, err := cmdCtx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("TYPE", "URL")
	if populated.Relations != nil {
		for _, rel := range *populated.Relations {
			tp.AddField(types.GetValue(rel.Rel, ""))
			tp.AddField(types.GetValue(rel.Url, ""))
			tp.EndRow()
		}
	}
	return tp.Render()
}
