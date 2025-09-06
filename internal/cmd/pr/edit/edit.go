package edit

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type editOptions struct {
	selectorArg string

	addRequiredReviewer    []string
	removeRequiredReviewer []string
	addOptionalReviewer    []string
	removeOptionalReviewer []string
	base                   string
	body                   string
	bodyFile               string
	title                  string
	addLabel               []string
	removeLabel            []string
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &editOptions{}

	cmd := &cobra.Command{
		Use:   "edit [<number> | <branch> | <url>]",
		Short: "Edit a pull request",
		Long: heredoc.Docf(`
			Edit an existing pull request.

			Without an argument, the pull request that belongs to the current branch is selected.
			If there are more than one pull request associated with the current branch, one pull request will be selected based on the shared finder logic.
		`, "`"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.selectorArg = args[0]
			}

			// Validate that body and body-file are not used together
			if opts.body != "" && opts.bodyFile != "" {
				return fmt.Errorf("cannot use both --body and --body-file")
			}

			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().StringSliceVarP(&opts.addRequiredReviewer, "add-required-reviewer", "", nil, "Add required reviewers (comma-separated)")
	cmd.Flags().StringSliceVarP(&opts.removeRequiredReviewer, "remove-required-reviewer", "", nil, "Remove required reviewers (comma-separated)")
	cmd.Flags().StringSliceVarP(&opts.addOptionalReviewer, "add-optional-reviewer", "", nil, "Add optional reviewers (comma-separated)")
	cmd.Flags().StringSliceVarP(&opts.removeOptionalReviewer, "remove-optional-reviewer", "", nil, "Remove optional reviewers (comma-separated)")
	cmd.Flags().StringVarP(&opts.base, "base", "B", "", "Change the base branch for this pull request")
	cmd.Flags().StringVarP(&opts.body, "body", "b", "", "Set the new body.")
	cmd.Flags().StringVarP(&opts.bodyFile, "body-file", "F", "", "Read body text from file (use \"-\" to read from standard input)")
	cmd.Flags().StringVarP(&opts.title, "title", "t", "", "Set the new title.")
	cmd.Flags().StringSliceVarP(&opts.addLabel, "add-label", "", nil, "Add labels (comma-separated)")
	cmd.Flags().StringSliceVarP(&opts.removeLabel, "remove-label", "", nil, "Remove labels (comma-separated)")

	// Register branch completion for the base flag
	// I will need to get the GitClient here or pass it to RegisterBranchCompletionFlags
	// Let's assume I can get it within NewCmd or pass it. I'll refine this during implementation.
	// For now, I'll add a placeholder comment.
	// err := util.RegisterBranchCompletionFlags(gitClient, cmd, "base")
	// if err != nil {
	// 	// Handle error
	// }

	return cmd
}

func runCmd(ctx util.CmdContext, opts *editOptions) (err error) {
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	finder, err := shared.NewFinder(ctx)
	if err != nil {
		return fmt.Errorf("failed to create PR finder: %w", err)
	}

	pr, prRepo, err := finder.Find(shared.FindOptions{
		Selector: opts.selectorArg,
	})
	if err != nil {
		return fmt.Errorf("failed to find pull request: %w", err)
	}

	if pr == nil {
		return fmt.Errorf("pull request not found")
	}

	gitClient, err := prRepo.GitClient(ctx.Context(), ctx.ConnectionFactory())
	if err != nil {
		return fmt.Errorf("failed to get Git REST client: %w", err)
	}

	repo, err := prRepo.GitRepository(ctx.Context(), gitClient)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	connection, err := ctx.ConnectionFactory().Connection(prRepo.Organization())
	if err != nil {
		return fmt.Errorf("failed to create Azure DevOps connection: %w", err)
	}

	identityClient, err := identity.NewClient(ctx.Context(), connection)
	if err != nil {
		return fmt.Errorf("failed to create Identity client: %w", err)
	}

	updatePullRequest := git.GitPullRequest{}

	// Update title if flag is set
	if opts.title != "" {
		updatePullRequest.Title = types.ToPtr(opts.title)
	}

	// Update body if flag is set
	if opts.body != "" {
		updatePullRequest.Description = types.ToPtr(opts.body)
	} else if opts.bodyFile != "" {
		bodyBytes, err := readBodyFile(opts.bodyFile)
		if err != nil {
			return fmt.Errorf("failed to read body file: %w", err)
		}
		updatePullRequest.Description = types.ToPtr(string(bodyBytes))
	}

	// Update base branch if flag is set
	if opts.base != "" {
		updatePullRequest.TargetRefName = types.ToPtr("refs/heads/" + opts.base)
	}

	// Handle reviewers
	var reviewers []git.IdentityRefWithVote
	if pr.Reviewers != nil {
		reviewers = *pr.Reviewers
	}

	// Batch process all reviewers to add
	allReviewersToAdd := append([]string{}, opts.addRequiredReviewer...)
	allReviewersToAdd = append(allReviewersToAdd, opts.addOptionalReviewer...)

	if len(allReviewersToAdd) > 0 {
		descriptors, err := shared.GetReviewerDescriptors(ctx.Context(), identityClient, allReviewersToAdd)
		if err != nil {
			return fmt.Errorf("failed to get reviewer descriptors: %w", err)
		}

		// Add required reviewers
		for i, r := range opts.addRequiredReviewer {
			reviewers = append(reviewers, git.IdentityRefWithVote{
				DisplayName: types.ToPtr(r),
				Descriptor:  types.ToPtr(descriptors[i]),
				IsRequired:  types.ToPtr(true),
			})
		}

		// Add optional reviewers
		offset := len(opts.addRequiredReviewer)
		for i, r := range opts.addOptionalReviewer {
			reviewers = append(reviewers, git.IdentityRefWithVote{
				DisplayName: types.ToPtr(r),
				Descriptor:  types.ToPtr(descriptors[offset+i]),
				IsRequired:  types.ToPtr(false),
			})
		}
	}

	// Remove required reviewers
	for _, r := range opts.removeRequiredReviewer {
		reviewers = removeReviewer(reviewers, r, true)
	}

	// Remove optional reviewers
	for _, r := range opts.removeOptionalReviewer {
		reviewers = removeReviewer(reviewers, r, false)
	}

	if len(reviewers) > 0 {
		updatePullRequest.Reviewers = &reviewers
	}

	// Handle labels
	var labels []core.WebApiTagDefinition
	if pr.Labels != nil {
		// Need to convert git.GitPullRequestLabel to core.WebApiTagDefinition
		for _, label := range *pr.Labels {
			labels = append(labels, core.WebApiTagDefinition{
				Id:   label.Id,
				Name: label.Name,
			})
		}
	}

	// Add labels
	for _, l := range opts.addLabel {
		labels = append(labels, core.WebApiTagDefinition{
			Name: types.ToPtr(l),
		})
	}

	// Remove labels
	for _, l := range opts.removeLabel {
		labels = removeLabel(labels, l)
	}
	updatePullRequest.Labels = &labels

	// Call the API to update the pull request
	updatedPr, err := gitClient.UpdatePullRequest(ctx.Context(), git.UpdatePullRequestArgs{
		GitPullRequestToUpdate: &updatePullRequest, // Corrected field name
		RepositoryId:           types.ToPtr(repo.Id.String()),
		PullRequestId:          pr.PullRequestId,
		Project:                types.ToPtr(prRepo.Project()),
	})
	if err != nil {
		return fmt.Errorf("failed to update pull request: %w", err)
	}

	fmt.Fprintf(iostreams.Out, "Pull request #%d updated: %s\n", *updatedPr.PullRequestId, *updatedPr.Title)

	return nil
}

func readBodyFile(filename string) ([]byte, error) {
	if filename == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(filename)
}

func removeReviewer(reviewers []git.IdentityRefWithVote, reviewerToRemove string, required bool) []git.IdentityRefWithVote {
	var updatedReviewers []git.IdentityRefWithVote
	for _, r := range reviewers {
		// Access DisplayName and UniqueName directly
		if !((strings.EqualFold(*r.DisplayName, reviewerToRemove) || (r.UniqueName != nil && strings.EqualFold(*r.UniqueName, reviewerToRemove))) && *r.IsRequired == required) {
			updatedReviewers = append(updatedReviewers, r)
		}
	}
	return updatedReviewers
}

func removeLabel(labels []core.WebApiTagDefinition, labelToRemove string) []core.WebApiTagDefinition {
	var updatedLabels []core.WebApiTagDefinition
	for _, l := range labels {
		if !strings.EqualFold(*l.Name, labelToRemove) {
			updatedLabels = append(updatedLabels, l)
		}
	}
	return updatedLabels
}
