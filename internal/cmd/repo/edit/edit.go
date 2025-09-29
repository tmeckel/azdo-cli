package edit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type editOptions struct {
	repository    string
	defaultBranch string
	name          string
	disable       bool
	enable        bool
	exporter      util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &editOptions{}

	cmd := &cobra.Command{
		Short: "Edit or update an existing Git repository in a team project",
		Long: heredoc.Doc(`
			Modify properties of an Azure DevOps Git repository, including changing its default branch, renaming it, or toggling its disabled state.

			Constraints for disabled repositories:
			- When a repository is disabled, the only permitted action is to enable it using --enable.
			- Attempts to change the default branch, rename, or disable an already-disabled repository will be blocked with a clear error message.
			- Trying to re-disable a disabled repository or re-enable an enabled repository will also produce a specific "already disabled/enabled" error.
		`),
		Use: "edit [organization/]project/repository",
		Example: heredoc.Doc(`
        # Change the default branch (org from default config)
        azdo repo edit myproject/myrepo --default-branch live

        # Change the default branch with a full ref
        azdo repo edit myorg/myproject/myrepo --default-branch refs/heads/live

        # Rename a repository
        azdo repo edit myproject/myrepo --name NewRepoName

        # Disable a repository
        azdo repo edit myproject/myrepo --disable

        # Enable a previously disabled repository
        azdo repo edit myproject/myrepo --enable

        # Error: trying to disable an already disabled repo
        azdo repo edit myproject/myrepo --disable

        # Error: trying to make changes to a disabled repo (must enable first)
        azdo repo edit myproject/myrepo --default-branch main
    `),
		Args: util.ExactArgs(1, "cannot edit: repository argument required"),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			flagSet := false
			flagNames := []string{
				"default-branch",
				"name",
				"disable",
				"enable",
			}
			cmd.Flags().Visit(func(f *pflag.Flag) {
				if !flagSet {
					flagSet = slices.Contains(flagNames, f.Name)
				}
			})
			if !flagSet {
				return util.FlagErrorf("at least one of --name, --disable, --enable or --default-branch must be specified")
			}
			return nil
		},
		RunE: func(c *cobra.Command, args []string) error {
			opts.repository = args[0]
			return runEdit(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.defaultBranch, "default-branch", "", "Set the default branch for the repository")
	cmd.Flags().StringVar(&opts.name, "name", "", "Rename the repository")
	cmd.Flags().BoolVar(&opts.disable, "disable", false, "Disable the repository")
	cmd.Flags().BoolVar(&opts.enable, "enable", false, "Enable the repository")
	util.AddJSONFlags(cmd, &opts.exporter, []string{"ID", "Name", "Project", "DefaultBranch"})

	return cmd
}

func runEdit(ctx util.CmdContext, opts *editOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	r, err := azdo.RepositoryFromName(opts.repository)
	if err != nil {
		return err
	}

	organization := r.Organization()
	project := r.Project()

	gitClient, err := ctx.ClientFactory().Git(ctx.Context(), organization)
	if err != nil {
		return err
	}

	// retrieve repository to get the ID
	repo, err := r.GitRepository(ctx.Context(), gitClient)
	if err != nil {
		return err
	}

	// Disallow any changes if repository is disabled unless enabling it
	if repo.IsDisabled != nil && *repo.IsDisabled && (!opts.enable || opts.name != "" || opts.defaultBranch != "") {
		return fmt.Errorf("repository %s is disabled; only --enable can be used", r.FullName())
	}

	// Pre-check for enable/disable to avoid unsupported change error
	if opts.disable && repo.IsDisabled != nil && *repo.IsDisabled {
		return fmt.Errorf("repository %s is already disabled", r.FullName())
	}
	if opts.enable && repo.IsDisabled != nil && !*repo.IsDisabled {
		return fmt.Errorf("repository %s is already enabled", r.FullName())
	}

	args := git.UpdateRepositoryArgs{
		RepositoryId:      repo.Id,
		Project:           types.ToPtr(project),
		NewRepositoryInfo: &git.GitRepository{},
	}

	if opts.defaultBranch != "" {
		normalized := "refs/heads/" + strings.TrimPrefix(opts.defaultBranch, "refs/heads/")
		args.NewRepositoryInfo.DefaultBranch = types.ToPtr(normalized)
	}

	if opts.name != "" {
		args.NewRepositoryInfo.Name = types.ToPtr(opts.name)
	}
	if opts.disable {
		args.NewRepositoryInfo.IsDisabled = types.ToPtr(true)
	} else if opts.enable {
		args.NewRepositoryInfo.IsDisabled = types.ToPtr(false)
	}

	updated, err := gitClient.UpdateRepository(ctx.Context(), args)
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		jd := struct {
			ID            string `json:"ID"`
			Name          string `json:"Name"`
			Project       string `json:"Project"`
			DefaultBranch string `json:"DefaultBranch,omitempty"`
			IsDisabled    *bool  `json:"IsDisabled,omitempty"`
		}{
			ID:            updated.Id.String(),
			Name:          *updated.Name,
			Project:       project,
			DefaultBranch: types.GetValue(updated.DefaultBranch, ""),
			IsDisabled:    updated.IsDisabled,
		}
		return opts.exporter.Write(ios, jd)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "Name", "Project", "DefaultBranch", "IsDisabled")
	tp.AddField(updated.Id.String())
	tp.AddField(*updated.Name)
	tp.AddField(project)
	tp.AddField(types.GetValue(updated.DefaultBranch, ""))
	tp.AddField(fmt.Sprintf("%t", *updated.IsDisabled))
	tp.EndRow()
	return tp.Render()
}
