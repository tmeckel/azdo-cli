package add

import (
	"fmt"
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
			be other work items (by ID, optionally prefixed with their project) or
			arbitrary artifact URLs. Work items in other projects of the same
			organization are resolved via 'PROJECT/ID'. Cross-organization links
			are not possible by ID; use --target-url with a remote link type such
			as 'Remote Related', 'Consumes From' or 'Produces For'.
		`),
		Example: heredoc.Doc(`
			# Add a parent relation to another work item
			azdo boards work-item relation add Fabrikam/1234 --relation-type parent --target-id 5678

			# Add a parent relation to a work item in another project of the same organization
			azdo boards work-item relation add Fabrikam/1234 --relation-type parent --target-id Contoso/77

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
	cmd.Flags().StringArrayVarP(&opts.targetIDs, "target-id", "T", nil, "Target work item ID (repeatable; comma-separated; each entry is [PROJECT/]ID; ID-only targets resolve in the current project).")
	cmd.Flags().StringArrayVarP(&opts.targetURLs, "target-url", "u", nil, "Target artifact URL (repeatable; comma-separated values accepted).")

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
	type target struct {
		id      int
		project string
	}
	parsedTargets := make([]target, 0, len(targetIDs))
	for _, tid := range targetIDs {
		p, err := util.Parse(nil, tid, util.ParseOptions{
			DisallowOrganization: true,
			AllowBareTargets:     true,
			MinTargets:           1,
			MaxTargets:           1,
		})
		if err != nil {
			return util.FlagErrorWrap(err)
		}
		targetProject := scope.Project
		if p.Project != "" {
			targetProject = p.Project
		}
		n, err := strconv.Atoi(p.Targets[0])
		if err != nil || n <= 0 {
			return util.FlagErrorf("target work item ID must be a positive integer; got %q", p.Targets[0])
		}
		parsedTargets = append(parsedTargets, target{id: n, project: targetProject})
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

	source, err := wit.GetWorkItem(cmdCtx.Context(), workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: types.ToPtr(scope.Project),
		Fields:  types.ToPtr([]string{wishared.TeamProjectField}),
	})
	if err != nil {
		return fmt.Errorf("failed to fetch work item %d: %w", id, err)
	}
	if !wishared.BelongsToProject(source, scope.Project) {
		return fmt.Errorf("work item %d does not belong to project %q", id, scope.Project)
	}

	// Resolve target IDs to URLs.
	targetURLsResolved := []string{}
	for _, tgt := range parsedTargets {
		target, err := wit.GetWorkItem(cmdCtx.Context(), workitemtracking.GetWorkItemArgs{
			Project: types.ToPtr(tgt.project),
			Id:      &tgt.id,
		})
		if err != nil {
			return fmt.Errorf("failed to resolve target work item %d: %w", tgt.id, err)
		}
		if !wishared.BelongsToProject(target, tgt.project) {
			return fmt.Errorf("target work item %d does not belong to project %q", tgt.id, tgt.project)
		}
		if target == nil || target.Url == nil || *target.Url == "" {
			return fmt.Errorf("target work item %d has no URL; cannot create relation", tgt.id)
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
