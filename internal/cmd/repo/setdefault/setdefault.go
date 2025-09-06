package setdefault

import (
	"errors"
	"fmt"
	"sort"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type setDefaultOptions struct {
	repo      azdo.Repository
	viewMode  bool
	unsetMode bool
}

func NewCmdRepoSetDefault(ctx util.CmdContext) *cobra.Command {
	opts := &setDefaultOptions{}

	cmd := &cobra.Command{
		Use:   "set-default [<repository>]",
		Short: "Configure default repository for this directory",
		Long: `
			This command sets the default remote repository to use when querying the
			Azure DevOps API for the locally cloned repository.

			azdo uses the default repository for things like:

			- viewing and creating pull requests
			- viewing and creating issues
			- viewing and creating releases
			- working with Azure DevOps Pipelines

			The command will only take configured remotes into account which point to a Azure DevOps organization.`,
		Example: heredoc.Doc(`
			Interactively select a default repository:
			$ azdo repo set-default

			Set a repository explicitly:
			$ azdo repo set-default [organization/]project/repo

			View the current default repository:
			$ azdo repo set-default --view

			Show more repository options in the interactive picker:
			$ git remote add newrepo https://dev.azure.com/myorg/myrepo/_git/myrepo
			$ azdo repo set-default
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				var err error
				opts.repo, err = azdo.RepositoryFromName(args[0])
				if err != nil {
					return err
				}
			}

			iostreams, err := ctx.IOStreams()
			if err != nil {
				return util.FlagErrorf("error getting io streams: %w", err)
			}

			if !opts.viewMode && !iostreams.CanPrompt() && opts.repo == nil {
				return util.FlagErrorf("repository required when not running interactively")
			}

			gitClient, err := ctx.RepoContext().GitCommand()
			if err != nil {
				return err
			}
			if isLocal, err := gitClient.IsLocalGitRepo(cmd.Context()); err != nil {
				return err
			} else if !isLocal {
				return errors.New("must be run from inside a git repository")
			}

			return setDefaultRun(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.viewMode, "view", "v", false, "view the current default repository")
	cmd.Flags().BoolVarP(&opts.unsetMode, "unset", "u", false, "unset the current default repository")

	return cmd
}

func setDefaultRun(ctx util.CmdContext, opts *setDefaultOptions) error {
	remotes, err := ctx.RepoContext().Remotes()
	if err != nil {
		return err
	}
	if len(remotes) == 0 {
		return errors.New("none of the git remotes correspond to a valid Azure DevOps remote repository")
	}

	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	currentDefaultRemote, _ := remotes.DefaultRemote()
	if opts.viewMode {
		if currentDefaultRemote != nil {
			fmt.Fprintln(iostreams.Out, currentDefaultRemote)
		} else {
			fmt.Fprintln(iostreams.ErrOut, "no default repository has been set; use `azdo repo set-default` to select one")
		}
		return nil
	}
	cs := iostreams.ColorScheme()

	gitClient, err := ctx.RepoContext().GitCommand()
	if err != nil {
		return err
	}

	if opts.unsetMode {
		var msg string
		if currentDefaultRemote != nil {
			if err := gitClient.UnsetRemoteResolution(
				ctx.Context(), currentDefaultRemote.Name); err != nil {
				return err
			}
			msg = fmt.Sprintf("%s Unset %s as default repository",
				cs.SuccessIcon(),
				currentDefaultRemote.Name)
		} else {
			msg = "no default repository has been set"
		}

		if iostreams.IsStdoutTTY() {
			fmt.Fprintln(iostreams.Out, msg)
		}

		return nil
	}

	var selectedRemote *azdo.Remote
	if len(remotes) == 1 {
		selectedRemote = remotes[0]
	} else {
		var repoNames []string
		current := ""
		if currentDefaultRemote != nil {
			current = currentDefaultRemote.Name
		}

		for _, remote := range remotes {
			repoNames = append(repoNames, remote.Repository().FullName())
		}

		fmt.Fprintln(iostreams.Out)

		p, err := ctx.Prompter()
		if err != nil {
			return util.FlagErrorf("error getting io propter: %w", err)
		}
		selected, err := p.Select("Which repository should be the default?", current, repoNames)
		if err != nil {
			return fmt.Errorf("could not prompt: %w", err)
		}
		selectedName := repoNames[selected]
		for _, remote := range remotes {
			if selectedName == remote.Repository().FullName() {
				selectedRemote = remote
			}
		}
	}

	resolution := "default"
	if selectedRemote == nil {
		sort.Stable(remotes)
		selectedRemote = remotes[0]
	}

	if currentDefaultRemote != nil {
		if err := gitClient.UnsetRemoteResolution(
			ctx.Context(),
			currentDefaultRemote.Name); err != nil {
			return err
		}
	}
	if err = gitClient.SetRemoteResolution(
		ctx.Context(),
		selectedRemote.Name,
		resolution); err != nil {
		return err
	}

	if iostreams.IsStdoutTTY() {
		cs := iostreams.ColorScheme()
		fmt.Fprintf(iostreams.Out,
			"%s Set %s as the default repository for the current directory\n",
			cs.SuccessIcon(),
			selectedRemote.Repository().FullName())
	}

	return nil
}
