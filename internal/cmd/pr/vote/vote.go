package vote

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type voteOptions struct {
	selectorArg string
	vote        string
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &voteOptions{}

	cmd := &cobra.Command{
		Use:   "vote [<number> | <branch> | <url>]",
		Short: "Vote on a pull request",
		Long: heredoc.Doc(`
            Cast or reset your reviewer vote on an Azure DevOps pull request.

            Without an argument, the pull request associated with the current branch is selected.
        `),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.selectorArg = args[0]
			}
			return runCmd(ctx, opts)
		},
	}

	util.StringEnumFlag(cmd, &opts.vote, "vote", "", "approve", []string{"approve", "approve-with-suggestions", "reject", "reset", "wait-for-author"}, "Vote value to set")
	_ = cmd.MarkFlagRequired("vote")

	return cmd
}

func runCmd(ctx util.CmdContext, opts *voteOptions) error {
	io, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	finder, err := shared.NewFinder(ctx)
	if err != nil {
		return err
	}
	pr, prRepo, err := finder.Find(shared.FindOptions{Selector: opts.selectorArg})
	if err != nil {
		return err
	}
	if pr == nil {
		return util.NewNoResultsError("pull request not found")
	}

	voteVal, err := mapVote(opts.vote)
	if err != nil {
		return err
	}

	gitClient, err := prRepo.GitClient(ctx.Context(), ctx.ConnectionFactory())
	if err != nil {
		return fmt.Errorf("failed to get Git client: %w", err)
	}

	gitRepo, err := prRepo.GitRepository(ctx.Context(), gitClient)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	extensionsClient, err := ctx.ClientFactory().Extensions(ctx.Context(), prRepo.Organization())
	if err != nil {
		return err
	}

	reviewerID, err := extensionsClient.GetSelfID(ctx.Context())
	if err != nil {
		return fmt.Errorf("failed to determine current user id: %w", err)
	}

	// Cast or reset the vote for the current user
	_, err = gitClient.CreatePullRequestReviewer(ctx.Context(), git.CreatePullRequestReviewerArgs{
		Reviewer: &git.IdentityRefWithVote{
			Vote: &voteVal,
		},
		RepositoryId:  types.ToPtr(gitRepo.Id.String()),
		PullRequestId: pr.PullRequestId,
		ReviewerId:    types.ToPtr(reviewerID.String()),
		Project:       types.ToPtr(prRepo.Project()),
	})
	if err != nil {
		return fmt.Errorf("failed to set vote: %w", err)
	}

	// Feedback
	fmt.Fprintf(io.Out, "%s Set vote to '%s' for %s#%d\n",
		io.ColorScheme().SuccessIcon(),
		opts.vote,
		prRepo.FullName(),
		*pr.PullRequestId,
	)

	return nil
}

func mapVote(v string) (int, error) {
	switch v {
	case "approve":
		return 10, nil
	case "approve-with-suggestions":
		return 5, nil
	case "reset":
		return 0, nil
	case "wait-for-author":
		return -5, nil
	case "reject":
		return -10, nil
	default:
		return 0, fmt.Errorf("invalid vote: %s", v)
	}
}
