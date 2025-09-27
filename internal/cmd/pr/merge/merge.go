package merge

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type mergeOptions struct {
	selectorArg string

	completionMessage          string
	deleteSourceBranch         bool
	mergeStrategy              string
	transitionWorkItemStatuses bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &mergeOptions{}

	cmd := &cobra.Command{
		Use:   "merge <number> | <branch> | <url>",
		Short: "Merge a pull request",
		Long: heredoc.Docf(`
			Merge a pull request on Azure DevOps.

			Without an argument, the pull request that belongs to the current branch
			is selected.

			If required checks have not yet passed, auto-complete will be enabled.
		`, "`"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.selectorArg = args[0]
			}
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.completionMessage, "message", "m", "", "Message to include when completing the pull request")
	cmd.Flags().BoolVarP(&opts.deleteSourceBranch, "delete-source-branch", "d", false, "Delete the source branch after merging")
	util.StringEnumFlag(cmd, &opts.mergeStrategy, "merge-strategy", "", "NoFastForward", []string{"noFastForward", "squash", "rebase", "rebaseMerge"}, "Merge strategy to use")
	cmd.Flags().BoolVar(&opts.transitionWorkItemStatuses, "transition-work-items", true, "Transition linked work item statuses upon merging")

	return cmd
}

func runCmd(ctx util.CmdContext, opts *mergeOptions) (err error) {
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	finder, err := shared.NewFinder(ctx)
	if err != nil {
		return err
	}

	pr, prRepo, err := finder.Find(shared.FindOptions{
		Selector: opts.selectorArg,
		States: []string{
			string(git.PullRequestStatusValues.Active),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to find pull request: %w", err)
	}

	if pr == nil {
		return util.NewNoResultsError("pull request not found")
	}

	gitClient, err := prRepo.GitClient(ctx.Context(), ctx.ConnectionFactory())
	if err != nil {
		return fmt.Errorf("failed to get Git REST client: %w", err)
	}

	repo, err := prRepo.GitRepository(ctx.Context(), gitClient)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	// Determine merge strategy
	var mergeStrategy git.GitPullRequestMergeStrategy
	switch opts.mergeStrategy {
	case "noFastForward":
		mergeStrategy = git.GitPullRequestMergeStrategyValues.NoFastForward
	case "squash":
		mergeStrategy = git.GitPullRequestMergeStrategyValues.Squash
	case "rebase":
		mergeStrategy = git.GitPullRequestMergeStrategyValues.Rebase
	case "rebaseMerge":
		mergeStrategy = git.GitPullRequestMergeStrategyValues.RebaseMerge
	default:
		// This should not happen due to StringEnumFlag, but as a fallback
		return fmt.Errorf("invalid merge strategy: %s", opts.mergeStrategy)
	}

	// Complete the pull request
	_, err = gitClient.UpdatePullRequest(ctx.Context(),
		git.UpdatePullRequestArgs{
			GitPullRequestToUpdate: &git.GitPullRequest{
				CompletionOptions: &git.GitPullRequestCompletionOptions{
					DeleteSourceBranch:  types.ToPtr(opts.deleteSourceBranch),
					MergeCommitMessage:  types.ToPtr(opts.completionMessage),
					MergeStrategy:       &mergeStrategy,
					TransitionWorkItems: types.ToPtr(opts.transitionWorkItemStatuses),
				},
			},
			RepositoryId:  types.ToPtr(repo.Id.String()),
			PullRequestId: pr.PullRequestId,
			Project:       types.ToPtr(prRepo.Project()),
		})
	if err != nil {
		return fmt.Errorf("failed to merge pull request: %w", err)
	}

	fmt.Fprintf(iostreams.Out, "%s Merged pull request %s#%d\n",
		iostreams.ColorScheme().SuccessIcon(),
		prRepo.FullName(),
		*pr.PullRequestId,
	)

	return nil
}
