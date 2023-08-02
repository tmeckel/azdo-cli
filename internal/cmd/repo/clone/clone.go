package clone

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type cloneOptions struct {
	organizationName   string
	project            string
	repository         string
	gitArgs            []string
	upstreamName       string
	noCredentialHelper bool
}

func NewCmdRepoClone(ctx util.CmdContext) *cobra.Command {
	opts := &cloneOptions{}

	cmd := &cobra.Command{
		DisableFlagsInUseLine: true,

		Use:   "clone <repository> [<directory>] [-- <gitflags>...]",
		Args:  util.MinimumArgs(1, "cannot clone: repository argument required"),
		Short: "Clone a repository locally",
		Long: heredoc.Docf(`
			Clone a GitHub repository locally. Pass additional %[1]sgit clone%[1]s flags by listing
			them after "--".
		`, "`"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.repository = args[0]
			opts.gitArgs = args[1:]

			return runClone(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.organizationName, "organization", "o", "", "Use organization")
	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Use project")
	cmd.Flags().StringVarP(&opts.upstreamName, "upstream-remote-name", "u", "upstream", "Upstream remote name when cloning a fork")
	cmd.Flags().BoolVar(&opts.noCredentialHelper, "no-credential-helper", false, "Don't configure azdo as credential helper for the cloned repository")
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		if err == pflag.ErrHelp {
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

	repoItems := strings.Split(opts.repository, "/")
	err = util.MutuallyExclusive("Either fully qualify the repository to clone ({PROJECT}/{REPOSITORY}) or specifiy the repository and the project via the --project argument", opts.project != "", len(repoItems) > 1)
	if err != nil {
		return
	}
	if len(repoItems) > 1 {
		if opts.project != "" {
			return err
		}
		opts.project = repoItems[0]
		opts.repository = repoItems[1]
	} else if opts.project == "" {
		return fmt.Errorf("no project specified")
	}
	var organizationName string
	if opts.organizationName != "" {
		organizationName = opts.organizationName
	} else {
		organizationName, _ = cfg.Authentication().GetDefaultOrganization()
	}
	if organizationName == "" {
		return util.FlagErrorf("no organization specified")
	}

	conn, err := ctx.Connection(organizationName)
	if err != nil {
		return
	}
	rctx, err := ctx.Context()
	if err != nil {
		return err
	}

	repoClient, err := git.NewClient(rctx, conn)
	if err != nil {
		return err
	}

	res, err := repoClient.GetRepositories(rctx, git.GetRepositoriesArgs{
		Project: &opts.project,
	})
	if err != nil {
		return
	}

	var repo *git.GitRepository = nil
	for _, r := range *res {
		if strings.EqualFold(*r.Name, opts.repository) {
			repo = &r
			break
		}
	}
	if repo == nil {
		return fmt.Errorf("repository %s not found in project %s and organization %s", opts.repository, opts.project, organizationName)
	}

	protocol, err := cfg.GetOrDefault([]string{config.Organizations, organizationName, "git_protocol"})
	if err != nil {
		return nil
	}

	var canonicalCloneURL string
	if strings.EqualFold(protocol, "ssh") {
		canonicalCloneURL = *repo.SshUrl
	} else {
		canonicalCloneURL = *repo.WebUrl
	}
	gitClient, err := ctx.GitClient()
	if err != nil {
		return
	}
	cloneDir, err := gitClient.Clone(rctx, canonicalCloneURL, opts.gitArgs)
	if err != nil {
		return err
	}
	gitClient.RepoDir = cloneDir

	if !opts.noCredentialHelper {
		authArgs, err := gitClient.GetAuthConfig(rctx)
		if err != nil {
			return err
		}
		err = gitClient.SetConfig(rctx, authArgs...)
		if err != nil {
			return err
		}
	}

	if repo.IsFork != nil && *repo.IsFork {
		repo, err = repoClient.GetRepositoryWithParent(rctx, git.GetRepositoryWithParentArgs{
			RepositoryId:  lo.ToPtr(repo.Id.String()),
			IncludeParent: lo.ToPtr(true),
		})
		if err != nil {
			return err
		}

		fork, err := repoClient.GetRepository(rctx, git.GetRepositoryArgs{
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

		_, err = gitClient.AddRemote(rctx, opts.upstreamName, upstreamURL, []string{strings.TrimPrefix(*fork.DefaultBranch, "refs/heads/")})
		if err != nil {
			return err
		}

		if err := gitClient.Fetch(rctx, opts.upstreamName, ""); err != nil {
			return err
		}

		if err := gitClient.SetRemoteBranches(rctx, opts.upstreamName, `*`); err != nil {
			return err
		}

	}
	return
}
