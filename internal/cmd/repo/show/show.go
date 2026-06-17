package show

import (
	_ "embed"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
)

type showOptions struct {
	targetArg string
	exporter  util.Exporter
}

//go:embed show.tpl
var showTpl string

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]PROJECT/REPO_ID_OR_NAME",
		Short: "Show repository details",
		Long: heredoc.Doc(`
			Display the details of a single Azure DevOps Git repository.

			The repository is identified by name or ID. The organization segment is optional when a
			default organization is configured.
		`),
		Example: heredoc.Doc(`
			# Show a repository by name
			azdo repo show Fabrikam/my-repo

			# Show a repository by ID
			azdo repo show myorg/Fabrikam/00000000-0000-0000-0000-000000000000
		`),
		Aliases: []string{"view", "status"},
		Args:    util.ExactArgs(1, "target argument is required and must be in the form [ORGANIZATION/]PROJECT/REPO_ID_OR_NAME"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runShow(ctx, opts)
		},
	}

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id",
		"name",
		"defaultBranch",
		"remoteUrl",
		"sshUrl",
		"webUrl",
		"url",
		"project",
		"parentRepository",
		"size",
		"isDisabled",
		"isFork",
		"isInMaintenance",
		"validRemoteUrls",
		"properties",
		"_links",
	})

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
	if len(scope.Targets) == 0 {
		return util.FlagErrorf("repository target is required")
	}
	repoIDOrName := scope.Targets[0]
	if repoIDOrName == "" {
		return util.FlagErrorf("repository target is required")
	}

	gitClient, err := cmdCtx.ClientFactory().Git(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create git client: %w", err)
	}

	zap.L().Debug(
		"fetching repository",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("repository", repoIDOrName),
	)

	repo, err := gitClient.GetRepository(cmdCtx.Context(), git.GetRepositoryArgs{
		RepositoryId: &repoIDOrName,
		Project:      &scope.Project,
	})
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	if repo == nil {
		return fmt.Errorf("repository %q not found", repoIDOrName)
	}

	if opts.exporter != nil {
		ios.StopProgressIndicator()
		return opts.exporter.Write(ios, repo)
	}

	ios.StopProgressIndicator()

	t := template.New(
		ios.Out,
		ios.TerminalWidth(),
		ios.ColorEnabled(),
	).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"hasBool": func(v *bool) bool { return v != nil },
			"hasText": template.HasText,
			"parent": func(repository *git.GitRepository) string {
				if repository == nil || repository.IsFork == nil || !*repository.IsFork || repository.ParentRepository == nil {
					return ""
				}
				parent := repository.ParentRepository
				name := template.StringOrEmpty(parent.Name)
				if parent.Id == nil {
					return name
				}
				if name == "" {
					return parent.Id.String()
				}
				return fmt.Sprintf("%s (%s)", name, parent.Id.String())
			},
			"s": template.StringOrEmpty,
			"b": template.BoolString,
			"u": template.UUIDString,
			"size": func(size *uint64) string {
				if size == nil {
					return ""
				}

				const unit = 1024
				value := float64(*size)
				units := []string{"B", "KB", "MB", "GB"}
				unitIndex := 0
				for value >= unit && unitIndex < len(units)-1 {
					value /= unit
					unitIndex++
				}
				if unitIndex == 0 {
					return fmt.Sprintf("%d B", *size)
				}
				return fmt.Sprintf("%.1f %s", value, units[unitIndex])
			},
		})

	if err := t.Parse(showTpl); err != nil {
		return err
	}

	return t.ExecuteData(repo)
}
