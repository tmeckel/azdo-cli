package update

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
	targetArg      string
	newPath        string
	newDescription string
	exporter       util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:     "update [ORG:]PROJECT/PATH",
		Short:   "Update a folder.",
		Aliases: []string{"u"},
		Long: heredoc.Doc(`
			Update the path or description of a build definition folder.

			Mirrors 'az pipelines folder update'. At least one of --new-path or
			--new-description must be specified. The full updated folder is sent
			to the server (full replace, not a partial patch).
		`),
		Example: heredoc.Doc(`
			# Rename a folder in the default organization
			azdo pipelines folder update Fabrikam/External/CI --new-path Fabrikam/External/Release

			# Change only the description
			azdo pipelines folder update myorg:Fabrikam/External/CI --new-description "Release pipeline folder"

			# Rename and re-describe, output as JSON
			azdo pipelines folder update Fabrikam/External/CI --new-path Fabrikam/External/Release --new-description "Release pipelines" --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runUpdate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.newPath, "new-path", "", "New full path for the folder.")
	cmd.Flags().StringVar(&opts.newDescription, "new-description", "", "New description for the folder.")
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

func runUpdate(cmdCtx util.CmdContext, opts *opts) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if opts.newPath == "" && opts.newDescription == "" {
		return util.FlagErrorf("specify at least one of --new-path or --new-description")
	}

	scope, err := util.ParseProjectPathTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}
	path := strings.Join(scope.Targets, "/")

	client, err := cmdCtx.ClientFactory().Build(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create build client: %w", err)
	}

	list, err := client.GetFolders(cmdCtx.Context(), build.GetFoldersArgs{
		Project: types.ToPtr(scope.Project),
		Path:    types.ToPtr(path),
	})
	if err != nil {
		return fmt.Errorf("failed to fetch folder %s: %w", path, err)
	}
	folders := *list
	if len(folders) == 0 {
		return fmt.Errorf("folder %s not found in project %s", path, scope.Project)
	}
	if len(folders) > 1 {
		return fmt.Errorf("path %s matched %d folders; expected exactly 1", path, len(folders))
	}
	current := folders[0]

	if opts.newPath != "" {
		current.Path = types.ToPtr(opts.newPath)
	}
	if opts.newDescription != "" {
		current.Description = types.ToPtr(opts.newDescription)
	}

	updated, err := client.UpdateFolder(cmdCtx.Context(), build.UpdateFolderArgs{
		Folder:  &current,
		Project: types.ToPtr(scope.Project),
		Path:    types.ToPtr(path),
	})
	if err != nil {
		return fmt.Errorf("failed to update folder %s: %w", path, err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, updated)
	}

	fmt.Fprintf(ios.Out, "Updated folder %s\n", types.GetValue(updated.Path, path))
	return nil
}
