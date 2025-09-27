package close

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type closeOptions struct {
	selectorArg  string
	comment      string
	deleteBranch bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &closeOptions{}

	cmd := &cobra.Command{
		Use:   "close <number> | <branch> | <url>",
		Short: "Close a pull request",
		Args:  util.ExactArgs(1, "cannot close pull request: number, url, or branch required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.selectorArg = args[0]
			}
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.comment, "comment", "c", "", "Leave a closing comment")
	cmd.Flags().BoolVarP(&opts.deleteBranch, "delete-branch", "d", false, "Delete the local and remote branch after close")

	return cmd
}

func runCmd(ctx util.CmdContext, opts *closeOptions) (err error) {
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
		return fmt.Errorf("pr not found")
	}

	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	if *pr.Status != git.PullRequestStatusValues.Active {
		fmt.Fprintf(iostreams.ErrOut,
			"%s Unable to close pull request %s#%d (%s) because it is not active\n",
			iostreams.ColorScheme().WarningIcon(),
			prRepo.FullName(),
			*pr.PullRequestId,
			*pr.Title)
		return nil
	}

	gitClient, err := prRepo.GitClient(ctx.Context(), ctx.ConnectionFactory())
	if err != nil {
		return fmt.Errorf("failed to get Git REST client: %w", err)
	}

	repo, err := prRepo.GitRepository(ctx.Context(), gitClient)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}
	pr, err = gitClient.UpdatePullRequest(ctx.Context(), git.UpdatePullRequestArgs{
		GitPullRequestToUpdate: &git.GitPullRequest{
			Status: &git.PullRequestStatusValues.Abandoned,
		},
		RepositoryId:  types.ToPtr(repo.Id.String()),
		PullRequestId: pr.PullRequestId,
		Project:       types.ToPtr(prRepo.Project()),
	})
	if err != nil {
		return fmt.Errorf("failed to close pull request: %w", err)
	}

	_, err = gitClient.CreateThread(ctx.Context(), git.CreateThreadArgs{
		CommentThread: &git.GitPullRequestCommentThread{
			Comments: &[]git.Comment{
				{
					Content: &opts.comment,
				},
			},
			Status: &git.CommentThreadStatusValues.Closed,
		},
		RepositoryId:  types.ToPtr(repo.Id.String()),
		PullRequestId: pr.PullRequestId,
	})
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	if opts.deleteBranch {
		gitCmd, err := ctx.RepoContext().GitCommand()
		if err != nil {
			return fmt.Errorf("failed to get Git command: %w", err)
		}
		branchSwitchString := ""
		localBranchExists := gitCmd.HasLocalBranch(ctx.Context(), *pr.SourceRefName)
		if !localBranchExists {
			fmt.Fprintf(iostreams.ErrOut,
				"%s Skipped deleting the local branch since current directory is not a git repository or it does not contain the source branch of the PR\n",
				iostreams.ColorScheme().WarningIcon())
		} else {
			currentBranch, err := gitCmd.CurrentBranch(ctx.Context())
			if err != nil {
				return err
			}

			var branchToSwitchTo string
			if currentBranch == strings.TrimPrefix(*pr.SourceRefName, "refs/heads/") {
				if repo.DefaultBranch == nil || len(*repo.DefaultBranch) == 0 {
					return fmt.Errorf("repository %s does not have a default branch", *repo.Name)
				}
				err = gitCmd.CheckoutBranch(ctx.Context(), *repo.DefaultBranch)
				if err != nil {
					return fmt.Errorf("failed to switch to branch %q: %w", *repo.DefaultBranch, err)
				}
			}

			if err := gitCmd.DeleteLocalBranch(ctx.Context(), *pr.SourceRefName); err != nil {
				return fmt.Errorf("failed to delete local branch %s: %w", *pr.SourceRefName, err)
			}

			if branchToSwitchTo != "" {
				branchSwitchString = fmt.Sprintf(" and switched to branch %s", iostreams.ColorScheme().Cyan(branchToSwitchTo))
			}
		}

		if pr.ForkSource == nil {
			// Fetch current object ID of the source ref to provide a correct oldObjectId
			refs, err := gitClient.GetRefs(ctx.Context(), git.GetRefsArgs{
				RepositoryId: types.ToPtr(repo.Id.String()),
				Project:      types.ToPtr(prRepo.Project()),
				Filter:       pr.SourceRefName,
			})
			if err != nil {
				return fmt.Errorf("failed to get current ref for deletion: %w", err)
			}

			var oldObjectId string
			if refs != nil {
				for _, r := range refs.Value {
					if r.Name != nil && *r.Name == *pr.SourceRefName {
						if r.ObjectId != nil {
							oldObjectId = *r.ObjectId
							break
						}
					}
				}
			}
			if len(oldObjectId) == 0 {
				return fmt.Errorf("failed to resolve current object ID for %s", *pr.SourceRefName)
			}

			_, err = gitClient.UpdateRefs(ctx.Context(), git.UpdateRefsArgs{
				RepositoryId: types.ToPtr(repo.Id.String()),
				RefUpdates: &[]git.GitRefUpdate{
					{
						Name:        pr.SourceRefName,
						OldObjectId: types.ToPtr(oldObjectId),
						NewObjectId: types.ToPtr("0000000000000000000000000000000000000000"),
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to delete remote branch: %w", err)
			}
		} else {
			fmt.Fprintf(iostreams.ErrOut, "%s Skipped deleting the remote branch of a pull request from fork\n", iostreams.ColorScheme().WarningIcon())
		}
		fmt.Fprintf(iostreams.ErrOut, "%s Deleted branch %s%s\n", iostreams.ColorScheme().SuccessIconWithColor(iostreams.ColorScheme().Red), iostreams.ColorScheme().Cyan(*pr.SourceRefName), branchSwitchString)
	}
	return nil
}
