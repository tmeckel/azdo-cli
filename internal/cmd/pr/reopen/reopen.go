package reopen

import (
	"fmt"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type reopenOptions struct {
	selectorArg string
	comment     string
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &reopenOptions{}

	cmd := &cobra.Command{
		Use:   "reopen <number> | <branch> | <url>",
		Short: "Reopen a pull request",
		Args:  util.ExactArgs(1, "cannot reopen pull request: number, url, or branch required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.selectorArg = args[0]
			}
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.comment, "comment", "c", "", "Add a reopening comment")

	return cmd
}

func runCmd(ctx util.CmdContext, opts *reopenOptions) (err error) {
	finder, err := shared.NewFinder(ctx)
	if err != nil {
		return fmt.Errorf("failed to create PR finder: %w", err)
	}

	findOptions := shared.FindOptions{
		Selector: opts.selectorArg,
		States:   []string{"abandoned"},
	}

	pr, prRepo, err := finder.Find(findOptions)
	if err != nil {
		return fmt.Errorf("failed to find abandoned pull request: %w", err)
	}
	if pr == nil {
		return fmt.Errorf("no abandoned pull request found with selector %q", opts.selectorArg)
	}

	gitClient, err := prRepo.GitClient(ctx.Context(), ctx.ConnectionFactory())
	if err != nil {
		return fmt.Errorf("failed to get Git client: %w", err)
	}

	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	// Reopen the pull request
	prStatusActive := git.PullRequestStatus("active")
	updatedPR, err := gitClient.UpdatePullRequest(ctx.Context(), git.UpdatePullRequestArgs{
		GitPullRequestToUpdate: &git.GitPullRequest{
			Status: &prStatusActive,
		},
		PullRequestId: pr.PullRequestId,
		RepositoryId:  types.ToPtr(pr.Repository.Id.String()), // Convert UUID to string and get pointer
		Project:       types.ToPtr(prRepo.Project()),
	})
	if err != nil {
		return fmt.Errorf("failed to reopen pull request %d: %w", *pr.PullRequestId, err)
	}

	// Add comment if provided
	if opts.comment != "" {
		_, err := gitClient.CreateComment(ctx.Context(), git.CreateCommentArgs{
			Comment: &git.Comment{ // Assuming CommentCreate is in models
				Content: types.ToPtr(opts.comment),
			},
			PullRequestId: updatedPR.PullRequestId,
			RepositoryId:  types.ToPtr(updatedPR.Repository.Id.String()), // Convert UUID to string and get pointer
			Project:       types.ToPtr(prRepo.Project()),
		})
		if err != nil {
			// Log the error but don't return, as the PR was reopened successfully
			fmt.Fprintf(iostreams.ErrOut, "Warning: Failed to add comment to pull request %d: %v\n", *updatedPR.PullRequestId, err)
		}
	}

	fmt.Fprintf(iostreams.Out, "Pull request %d reopened successfully.\n", *updatedPR.PullRequestId)

	return nil
}
