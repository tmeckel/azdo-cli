package clone

import (
	"errors"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type cloneOptions struct {
	repository         string
	gitArgs            []string
	upstreamName       string
	noCredentialHelper bool
	recurseSubmodules  bool
}

func NewCmdRepoClone(ctx util.CmdContext) *cobra.Command {
	opts := &cloneOptions{}

	cmd := &cobra.Command{
		DisableFlagsInUseLine: true,

		Use:   "clone [organization/]project/repository [<directory>] [-- <gitflags>...]",
		Args:  util.MinimumArgs(1, "cannot clone: repository argument required"),
		Short: "Clone a repository locally",
		Long: heredoc.Docf(`
			Clone a GitHub repository locally. Pass additional %[1]sgit clone%[1]s flags by listing
			them after "--".

			If the repository name does not specify an organization, the configured default orgnaization is used
			or the value from the AZDO_ORGANIZATION environment variable.
		`, "`"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.repository = args[0]
			opts.gitArgs = args[1:]

			return runClone(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.upstreamName, "upstream-remote-name", "u", "upstream", "Upstream remote name when cloning a fork")
	cmd.Flags().BoolVarP(&opts.recurseSubmodules, "recurse-submodules", "", false, "Update all submodules after checkout")
	cmd.Flags().BoolVar(&opts.noCredentialHelper, "no-credential-helper", false, "Don't configure azdo as credential helper for the cloned repository")
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		if errors.Is(err, pflag.ErrHelp) {
			return err
		}
		return util.FlagErrorf("%w\nSeparate git clone flags with '--'.", err)
	})

	return cmd
}

func runClone(ctx util.CmdContext, opts *cloneOptions) (err error) {
	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}

	r, err := azdo.RepositoryFromName(opts.repository)
	if err != nil {
		return
	}

	repoClient, err := r.GitClient(ctx.Context(), ctx.ConnectionFactory())
	if err != nil {
		return err
	}

	repo, err := r.GitRepository(ctx.Context(), repoClient)
	if err != nil {
		return
	}

	protocol, err := cfg.GetOrDefault([]string{config.Organizations, r.Organization(), "git_protocol"})
	if err != nil {
		return err
	}

	var canonicalCloneURL string
	if strings.EqualFold(protocol, "ssh") {
		canonicalCloneURL = *repo.SshUrl
	} else {
		canonicalCloneURL = *repo.WebUrl
	}
	gitCmd, err := ctx.RepoContext().GitCommand()
	if err != nil {
		return
	}
	cloneDir, err := gitCmd.Clone(ctx.Context(), canonicalCloneURL, opts.gitArgs)
	if err != nil {
		return err
	}

	err = gitCmd.SetRepoDir(cloneDir)
	if err != nil {
		return err
	}

	if !opts.noCredentialHelper {
		authArgs, err := gitCmd.GetAuthConfig(ctx.Context())
		if err != nil {
			return err
		}
		err = gitCmd.SetConfig(ctx.Context(), authArgs...)
		if err != nil {
			return err
		}
	}

	if opts.recurseSubmodules {
		// FIXME: use auth helper otherwise user is prompted for password
		for _, c := range [][]string{{"submodule", "sync", "--recursive"}, {"submodule", "update", "--init", "--recursive"}} {
			cmd, err := gitCmd.Command(ctx.Context(), c...)
			if err != nil {
				return err
			}
			if err := cmd.Run(); err != nil {
				return err
			}
		}
	}

	if repo.IsFork != nil && *repo.IsFork {
		repo, err = repoClient.GetRepositoryWithParent(ctx.Context(), git.GetRepositoryWithParentArgs{
			RepositoryId:  lo.ToPtr(repo.Id.String()),
			IncludeParent: lo.ToPtr(true),
		})
		if err != nil {
			return err
		}

		fork, err := repoClient.GetRepository(ctx.Context(), git.GetRepositoryArgs{
			Project:      lo.ToPtr(repo.ParentRepository.Project.Id.String()),
			RepositoryId: lo.ToPtr(repo.ParentRepository.Id.String()),
		})
		if err != nil {
			return err
		}

		var upstreamURL string
		if strings.EqualFold(protocol, "ssh") {
			upstreamURL = *fork.SshUrl
		} else {
			upstreamURL = *fork.WebUrl
		}

		_, err = gitCmd.AddRemote(ctx.Context(), opts.upstreamName, upstreamURL, []string{strings.TrimPrefix(*fork.DefaultBranch, "refs/heads/")})
		if err != nil {
			return err
		}

		if err := gitCmd.Fetch(ctx.Context(), opts.upstreamName, ""); err != nil {
			return err
		}

		if err := gitCmd.SetRemoteBranches(ctx.Context(), opts.upstreamName, `*`); err != nil {
			return err
		}

	}
	return
}
