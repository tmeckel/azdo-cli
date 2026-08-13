package delete

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	targetArg string
	yes       bool
	destroy   bool
	exporter  util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:     "delete [ORG:]PROJECT/ID",
		Short:   "Delete a work item.",
		Aliases: []string{"d", "del", "rm"},
		Long: heredoc.Doc(`
			Delete a work item by ID. By default the work item is moved to the
			Recycle Bin and can be restored via the Azure DevOps web UI.
			Use --destroy to permanently remove the work item; this cannot be
			undone.
		`),
		Example: heredoc.Doc(`
			# Delete a work item in the default organization
			azdo boards work-item delete Fabrikam/42 --yes

			# Permanently destroy a work item in a specific organization
			azdo boards work-item delete myorg:Fabrikam/42 --destroy --yes
		`),
		Args: util.ExactArgs(1, "project/work item target required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runDelete(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip the confirmation prompt.")
	cmd.Flags().BoolVar(&opts.destroy, "destroy", false, "Permanently delete the work item (bypasses Recycle Bin).")
	util.AddJSONFlags(cmd, &opts.exporter, []string{"id", "code", "deletedBy", "deletedDate", "message", "name", "project", "type", "url", "resource"})

	return cmd
}

func runDelete(cmdCtx util.CmdContext, opts *opts) error {
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

	zap.L().Debug(
		"resolved work item delete target",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.Int("workItemId", id),
	)

	client, err := cmdCtx.ClientFactory().WorkItemTracking(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create work item tracking client: %w", err)
	}

	item, err := client.GetWorkItem(cmdCtx.Context(), workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: types.ToPtr(scope.Project),
		Fields:  types.ToPtr([]string{shared.TeamProjectField}),
	})
	if err != nil {
		return fmt.Errorf("failed to fetch work item %d: %w", id, err)
	}
	if !shared.BelongsToProject(item, scope.Project) {
		return fmt.Errorf("work item %d does not belong to project %q", id, scope.Project)
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
		message := "Are you sure you want to delete this work item?"
		if opts.destroy {
			message = "Are you sure you want to permanently destroy this work item? This cannot be undone."
		}
		confirmed, err := prompter.Confirm(message, false)
		if err != nil {
			return err
		}
		if !confirmed {
			zap.L().Debug("work item deletion canceled by user", zap.Int("workItemId", id))
			return util.ErrCancel
		}
		ios.StartProgressIndicator()
	}

	res, err := client.DeleteWorkItem(cmdCtx.Context(), workitemtracking.DeleteWorkItemArgs{
		Project: types.ToPtr(scope.Project),
		Id:      &id,
		Destroy: &opts.destroy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete work item %d: %w", id, err)
	}

	zap.L().Debug(
		"work item deleted",
		zap.Int("workItemId", id),
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.Bool("destroy", opts.destroy),
	)

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, res)
	}

	if opts.destroy {
		fmt.Fprintf(ios.Out, "Permanently deleted work item %d\n", id)
		return nil
	}
	fmt.Fprintf(ios.Out, "Deleted work item %d\n", id)
	return nil
}
