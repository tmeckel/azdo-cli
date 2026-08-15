package add

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/relation/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type addOptions struct {
	targetArg string

	relationType string   // --relation-type
	targetIDs    []string // --target-id (repeatable; comma-separated also accepted)
	targetURLs   []string // --target-url (repeatable; comma-separated also accepted)

	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &addOptions{}

	cmd := &cobra.Command{
		Use:     "add [ORG:]PROJECT/ID",
		Aliases: []string{"a"},
		Short:   "Add a relation(s) to a work item.",
		Long: heredoc.Doc(`
			Attach one or more relations to an existing work item. The relation type
			must be one of the friendly names returned by 'list-type'. Targets can
			be other work items (by ID) or arbitrary artifact URLs.
		`),
		Example: heredoc.Doc(`
			# Add a parent relation to another work item
			azdo boards work-item relation add Fabrikam/1234 --relation-type parent --target-id 5678

			# Add a relation to multiple work items
			azdo boards work-item relation add Fabrikam/1234 --relation-type related --target-id 5678,5679

			# Add an artifact relation
			azdo boards work-item relation add Fabrikam/1234 --relation-type artifact --target-url https://example.com/release
		`),
		Args: util.ExactArgs(1, "project/source work item target required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runAdd(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.relationType, "relation-type", "", "Relation type (friendly name, e.g. parent, child, related).")
	cmd.Flags().StringArrayVar(&opts.targetIDs, "target-id", nil, "Target work item ID (repeatable; comma-separated values accepted).")
	cmd.Flags().StringArrayVar(&opts.targetURLs, "target-url", nil, "Target artifact URL (repeatable; comma-separated values accepted).")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"id", "rev", "fields", "url", "_links", "relations", "commentVersionRef"})

	return cmd
}

func runAdd(cmdCtx util.CmdContext, opts *addOptions) error {
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
	targetURLs, err := shared.SplitAndTrimCSV(opts.targetURLs)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	if len(targetIDs) == 0 && len(targetURLs) == 0 {
		return util.FlagErrorf("--target-id or --target-url must be provided")
	}
	if len(targetIDs) > 0 && len(targetURLs) > 0 {
		return util.FlagErrorf("--target-id and --target-url are mutually exclusive; supply only one")
	}
	for _, tid := range targetIDs {
		n, err := strconv.Atoi(tid)
		if err != nil || n <= 0 {
			return util.FlagErrorf("target work item ID must be a positive integer; got %q", tid)
		}
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
	targetURLsResolved := []string{}
	for _, tid := range targetIDs {
		n, _ := strconv.Atoi(tid)
		target, err := wit.GetWorkItem(cmdCtx.Context(), workitemtracking.GetWorkItemArgs{
			Project: types.ToPtr(scope.Project),
			Id:      &n,
		})
		if err != nil {
			return fmt.Errorf("failed to resolve target work item %d: %w", n, err)
		}
		if target == nil || target.Url == nil || *target.Url == "" {
			return fmt.Errorf("target work item %d has no URL; cannot create relation", n)
		}
		targetURLsResolved = append(targetURLsResolved, *target.Url)
	}
	targetURLsResolved = append(targetURLsResolved, targetURLs...)

	add := webapi.OperationValues.Add
	doc := []webapi.JsonPatchOperation{}
	for _, u := range targetURLsResolved {
		p := "/relations/-"
		doc = append(doc, webapi.JsonPatchOperation{
			Op:    &add,
			Path:  &p,
			Value: map[string]any{"rel": relRefName, "url": u},
		})
	}

	_, err = wit.UpdateWorkItem(cmdCtx.Context(), workitemtracking.UpdateWorkItemArgs{
		Project:  types.ToPtr(scope.Project),
		Document: &doc,
		Id:       &id,
	})
	if err != nil {
		return fmt.Errorf("failed to update work item %d: %w", id, err)
	}

	// Re-fetch with expand=All to populate relations.
	expand := workitemtracking.WorkItemExpandValues.All
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
