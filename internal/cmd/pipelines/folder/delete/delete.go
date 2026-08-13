package delete

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	targetArg string
	yes       bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:     "delete [ORG:]PROJECT/PATH",
		Short:   "Delete a folder.",
		Aliases: []string{"d", "del", "rm"},
		Long: heredoc.Doc(`
			Delete a build definition folder at PATH under PROJECT.

			Mirrors 'az pipelines folder delete'. The folder, all build definitions
			in it, and all builds for those definitions are deleted. This action is
			not reversible.
		`),
		Example: heredoc.Doc(`
			# Delete a folder in the default organization
			azdo pipelines folder delete Fabrikam/External/CI --yes

			# Delete a folder in a specific organization
			azdo pipelines folder delete myorg:Fabrikam/External/CI
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runDelete(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip the confirmation prompt.")

	return cmd
}

func runDelete(cmdCtx util.CmdContext, opts *opts) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectPathTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}
	path := strings.Join(scope.Targets, "/")

	zap.L().Debug(
		"resolved folder delete target",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("path", path),
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
		confirmed, err := prompter.Confirm("This will delete all pipelines in this folder. Are you sure you want to delete this folder?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			zap.L().Debug("folder deletion canceled by user", zap.String("path", path))
			return util.ErrCancel
		}
		ios.StartProgressIndicator()
	}

	client, err := cmdCtx.ClientFactory().Build(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create build client: %w", err)
	}

	if err := client.DeleteFolder(cmdCtx.Context(), build.DeleteFolderArgs{
		Project: types.ToPtr(scope.Project),
		Path:    types.ToPtr(path),
	}); err != nil {
		return fmt.Errorf("failed to delete folder %s: %w", path, err)
	}

	zap.L().Debug(
		"folder deleted",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("path", path),
	)

	ios.StopProgressIndicator()

	fmt.Fprintf(ios.Out, "Deleted folder %s/%s\n", scope.Project, path)
	return nil
}
