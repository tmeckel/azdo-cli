package list

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	targetArg  string
	path       string
	queryOrder string
	maxItems   int
	exporter   util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List folders.",
		Long: heredoc.Doc(`
			List build definition folders in PROJECT.

			Mirrors 'az pipelines folder list'. Use --path to limit the listing to
			a sub-folder. Use --query-order to sort by path ascending or descending.
		`),
		Example: heredoc.Doc(`
			# List top-level folders in a project
			azdo pipelines folder list Fabrikam

			# List folders at or under a sub-path
			azdo pipelines folder list Fabrikam --path /Shared

			# List folders sorted descending by path
			azdo pipelines folder list myorg/Fabrikam --query-order desc

			# Output as JSON
			azdo pipelines folder list Fabrikam --json
		`),
		Aliases: []string{
			"ls",
			"l",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runList(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.path, "path", "", "Limit the listing to folders at or under this path.")
	util.StringEnumFlag(cmd, &opts.queryOrder, "query-order", "", "", []string{"asc", "desc"}, "Sort folders by path")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Maximum number of folders to return (client-side; 0 = unlimited)")
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

func runList(cmdCtx util.CmdContext, opts *opts) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if opts.maxItems < 0 {
		return util.FlagErrorf("--max-items must be >= 0")
	}

	scope, err := util.ParseProjectScope(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorf("invalid project argument: %w", err)
	}

	ctx := cmdCtx.Context()

	client, err := cmdCtx.ClientFactory().Build(ctx, scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create build client: %w", err)
	}

	args := build.GetFoldersArgs{
		Project: types.ToPtr(scope.Project),
	}
	if opts.path != "" {
		args.Path = types.ToPtr(opts.path)
	}
	if opts.queryOrder != "" {
		q := build.FolderQueryOrderValues.FolderAscending
		if opts.queryOrder == "desc" {
			q = build.FolderQueryOrderValues.FolderDescending
		}
		args.QueryOrder = &q
	}

	zap.L().Debug(
		"listing folders",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("path", types.GetValue(args.Path, "")),
	)

	resp, err := client.GetFolders(ctx, args)
	if err != nil {
		return fmt.Errorf("failed to list folders: %w", err)
	}

	folders := []build.Folder{}
	if resp != nil {
		folders = *resp
	}

	if opts.maxItems > 0 && len(folders) > opts.maxItems {
		zap.L().Debug("truncating result set to max-items", zap.Int("maxItems", opts.maxItems))
		folders = folders[:opts.maxItems]
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, folders)
	}

	tp, err := cmdCtx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("PATH", "DESCRIPTION")
	tp.EndRow()
	for _, f := range folders {
		tp.AddField(types.GetValue(f.Path, ""))
		tp.AddField(types.GetValue(f.Description, ""))
		tp.EndRow()
	}

	return tp.Render()
}
