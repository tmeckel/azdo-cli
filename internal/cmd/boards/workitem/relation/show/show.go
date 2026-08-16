package show

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/relation/shared"
	wishared "github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type showOptions struct {
	targetArg string

	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:     "show [ORG:]PROJECT/ID",
		Aliases: []string{"s"},
		Short:   "List the relations of a work item.",
		Long: heredoc.Doc(`
			List all relations of an existing work item. Relation types are
			displayed by their friendly name.
		`),
		Example: heredoc.Doc(`
			# List the relations of a work item
			azdo boards work-item relation show Fabrikam/1234
		`),
		Args: util.ExactArgs(1, "project/work item target required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runShow(ctx, opts)
		},
	}

	util.AddJSONFlags(cmd, &opts.exporter, []string{"id", "rev", "fields", "url", "_links", "relations", "commentVersionRef"})

	return cmd
}

func runShow(cmdCtx util.CmdContext, opts *showOptions) error {
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

	wit, err := cmdCtx.ClientFactory().WorkItemTracking(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create work item tracking client: %w", err)
	}

	expand := workitemtracking.WorkItemExpandValues.All
	wi, err := wit.GetWorkItem(cmdCtx.Context(), workitemtracking.GetWorkItemArgs{
		Project: types.ToPtr(scope.Project),
		Id:      &id,
		Expand:  &expand,
	})
	if err != nil {
		return fmt.Errorf("failed to get work item %d: %w", id, err)
	}
	if !wishared.BelongsToProject(wi, scope.Project) {
		return fmt.Errorf("work item %d does not belong to project %q", id, scope.Project)
	}

	relTypes, err := wit.GetRelationTypes(cmdCtx.Context(), workitemtracking.GetRelationTypesArgs{})
	if err != nil {
		return fmt.Errorf("failed to get relation types: %w", err)
	}
	if err := shared.PopulateFriendlyNames(relTypes, wi); err != nil {
		return err
	}

	if opts.exporter != nil {
		return opts.exporter.Write(ios, wi)
	}
	tp, err := cmdCtx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("TYPE", "ORGANIZATION", "PROJECT", "ID", "TITLE")
	if wi.Relations != nil {
		resolved := map[int]shared.RelationTarget{}
		for _, rel := range *wi.Relations {
			tp.AddField(types.GetValue(rel.Rel, ""))
			target := shared.ResolveRelationTarget(cmdCtx.Context(), wit, scope, resolved, types.GetValue(rel.Url, ""))
			tp.AddField(target.Organization)
			tp.AddField(target.Project)
			idField := ""
			if target.ID > 0 {
				idField = strconv.Itoa(target.ID)
			}
			tp.AddField(idField)
			tp.AddField(target.Title)
			tp.EndRow()
		}
	}
	return tp.Render()
}
