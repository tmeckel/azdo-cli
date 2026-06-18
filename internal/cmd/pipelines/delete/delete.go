package delete

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deleteOptions struct {
	targetArg string
	yes       bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete [ORGANIZATION/]PROJECT/PIPELINE",
		Short: "Delete a pipeline definition",
		Long: heredoc.Doc(`
			Delete a pipeline definition by ID or name.

			The command prompts for confirmation unless --yes is supplied.
		`),
		Example: heredoc.Doc(`
			# Delete a pipeline by ID using the default organization
			azdo pipelines delete Fabrikam/42 --yes

			# Delete a pipeline by name
			azdo pipelines delete 'myorg/Fabrikam/My Pipeline'

			# Delete with confirmation
			azdo pipelines delete Fabrikam/MyPipeline
		`),
		Aliases: []string{
			"d",
			"del",
			"rm",
		},
		Args: util.ExactArgs(1, "pipeline target is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runDelete(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip the confirmation prompt.")

	return cmd
}

func runDelete(cmdCtx util.CmdContext, opts *deleteOptions) error {
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

	buildClient, err := cmdCtx.ClientFactory().Build(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Build client: %w", err)
	}

	pipelineID, err := shared.ResolvePipelineDefinition(cmdCtx, buildClient, scope.Project, scope.Targets[0])
	if err != nil {
		return err
	}

	zap.L().Debug(
		"resolved pipeline definition",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("input", scope.Targets[0]),
		zap.Int("pipelineId", pipelineID),
	)

	if !opts.yes {
		if !ios.CanPrompt() {
			return util.FlagErrorf("--yes required when not running interactively")
		}
		ios.StopProgressIndicator()
		prompter, err := cmdCtx.Prompter()
		if err != nil {
			return err
		}
		confirmed, err := prompter.Confirm("Are you sure you want to delete this pipeline?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			zap.L().Debug("pipeline deletion canceled by user", zap.Int("pipelineId", pipelineID))
			return util.ErrCancel
		}
		ios.StartProgressIndicator()
	}

	if err := buildClient.DeleteDefinition(cmdCtx.Context(), build.DeleteDefinitionArgs{
		Project:      types.ToPtr(scope.Project),
		DefinitionId: types.ToPtr(pipelineID),
	}); err != nil {
		return fmt.Errorf("failed to delete pipeline %d: %w", pipelineID, err)
	}

	zap.L().Debug(
		"pipeline definition deleted",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.Int("pipelineId", pipelineID),
	)

	ios.StopProgressIndicator()

	fmt.Fprintf(ios.Out, "Pipeline %d was deleted successfully.\n", pipelineID)
	return nil
}
