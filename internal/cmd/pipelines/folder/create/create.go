package create

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	targetArg   string
	description string
	exporter    util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:     "create [ORG:]PROJECT/PATH",
		Short:   "Create a folder.",
		Aliases: []string{"c", "cr"},
		Long: heredoc.Doc(`
			Create a build definition folder at PATH under PROJECT.

			Mirrors 'az pipelines folder create'. PATH is the full path
			(e.g. "External/CI"). Azure DevOps stores folder paths with '/'.
		`),
		Example: heredoc.Doc(`
			# Create a folder in the default organization
			azdo pipelines folder create Fabrikam/External/CI

			# Create a folder in a specific organization
			azdo pipelines folder create myorg:Fabrikam/External/CI

			# Create a folder with a description
			azdo pipelines folder create Fabrikam/External/CI --description "CI folders"

			# Output as JSON
			azdo pipelines folder create Fabrikam/External/CI --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runCreate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.description, "description", "", "Description of the folder.")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"createdBy",
		"createdOn",
		"description",
		"lastChangedBy",
		"lastChangedDate",
		"path",
		"project",
	})

	return cmd
}

func runCreate(cmdCtx util.CmdContext, opts *opts) error {
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

	client, err := cmdCtx.ClientFactory().Build(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create build client: %w", err)
	}

	created, err := client.CreateFolder(cmdCtx.Context(), build.CreateFolderArgs{
		Folder: &build.Folder{
			Description: types.ToPtr(opts.description),
		},
		Project: types.ToPtr(scope.Project),
		Path:    types.ToPtr(path),
	})
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, created)
	}

	fmt.Fprintf(ios.Out, "Created folder %s\n", types.GetValue(created.Path, path))
	return nil
}
