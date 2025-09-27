package checkout

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/git"
)

type checkoutOptions struct {
	selectorArg       string
	recurseSubmodules bool
	force             bool
	detach            bool
	branchName        string
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &checkoutOptions{}

	cmd := &cobra.Command{
		Use:   "checkout <number>",
		Short: "Check out a pull request in git",
		Args:  util.ExactArgs(1, "argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.selectorArg = args[0]
			if len(opts.selectorArg) == 0 {
				return fmt.Errorf("invalid reference to pull request")
			}
			_, err := strconv.Atoi(opts.selectorArg)
			if err != nil {
				return fmt.Errorf("invalid pull request number")
			}
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.recurseSubmodules, "recurse-submodules", "", false, "Update all submodules after checkout")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Reset the existing local branch to the latest state of the pull request")
	cmd.Flags().BoolVarP(&opts.detach, "detach", "", false, "Checkout PR with a detached HEAD")
	cmd.Flags().StringVarP(&opts.branchName, "branch", "b", "", "Local branch name to use (default [the name of the head branch])")

	return cmd
}

func runCmd(ctx util.CmdContext, opts *checkoutOptions) (err error) {
	finder, err := shared.NewFinder(ctx)
	if err != nil {
		return err
	}
	pr, prRepo, err := finder.Find(shared.FindOptions{
		Selector: opts.selectorArg,
	})
	if err != nil {
		return err
	}

	if pr == nil {
		return fmt.Errorf("pr is nil")
	}

	// if we are in a Git Repository, ensure that the PR
	// points to a configured remote of the repo. If not
	// return an error
	repo, _ := ctx.RepoContext().Repo()
	if repo != nil && !repo.Equals(prRepo) {
		return fmt.Errorf("referenced pull request is not in current repository")
	}

	gitCmd, err := ctx.RepoContext().GitCommand()
	if err != nil {
		return fmt.Errorf("failed to get git command: %w", err)
	}

	remotes, err := ctx.RepoContext().Remotes()
	if err != nil {
		return fmt.Errorf("failed to get git remotes: %w", err)
	}

	remote, err := remotes.FindByRepo(repo)
	if err != nil {
		return fmt.Errorf("repository %q not configured as a remote: %w", repo.FullName(), err)
	}

	var cmds [][]string

	// Use full ref for fetch, but short name for remote-tracking reference paths
	shortSrc := strings.TrimPrefix(*pr.SourceRefName, "refs/heads/")
	remoteBranch := fmt.Sprintf("%s/%s", remote.Name, shortSrc)

	refSpec := fmt.Sprintf("+%s", *pr.SourceRefName)
	if !opts.detach {
		refSpec += fmt.Sprintf(":refs/remotes/%s", remoteBranch)
	}

	cmds = append(cmds, []string{"fetch", remote.Name, refSpec})

	localBranch := fmt.Sprintf("pull/%d", *pr.PullRequestId)
	if opts.branchName != "" {
		localBranch = opts.branchName
	}

	switch {
	case opts.detach:
		cmds = append(cmds, []string{"checkout", "--detach", "FETCH_HEAD"})
	case localBranchExists(gitCmd, localBranch):
		cmds = append(cmds, []string{"checkout", localBranch})
		if opts.force {
			cmds = append(cmds, []string{"reset", "--hard", fmt.Sprintf("refs/remotes/%s", remoteBranch)})
		} else {
			// TODO: check if non-fast-forward and suggest to use `--force`
			cmds = append(cmds, []string{"merge", "--ff-only", fmt.Sprintf("refs/remotes/%s", remoteBranch)})
		}
	default:
		cmds = append(cmds, []string{"checkout", "-b", localBranch, "--track", remoteBranch})
	}

	if opts.recurseSubmodules {
		cmds = append(cmds, [][]string{{"submodule", "sync", "--recursive"}, {"submodule", "update", "--init", "--recursive"}}...)
	}

	return executeCmds(gitCmd, cmds)
}

func localBranchExists(client git.GitCommand, b string) bool {
	_, err := client.ShowRefs(context.Background(), []string{"refs/heads/" + b})
	return err == nil
}

func executeCmds(client git.GitCommand, cmdQueue [][]string) error {
	for _, args := range cmdQueue {
		var err error
		var cmd *git.Command
		if args[0] == "fetch" || args[0] == "submodule" {
			cmd, err = client.AuthenticatedCommand(context.Background(), args...)
		} else {
			cmd, err = client.Command(context.Background(), args...)
		}
		if err != nil {
			return err
		}
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}
