package edit

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type editOptions struct {
	selectorArg string

	addRequiredReviewer []string
	addOptionalReviewer []string
	removeReviewer      []string
	base                string
	body                string
	bodyFile            string
	title               string
	addLabel            []string
	removeLabel         []string
}

type reviewerIntent struct {
	ref      git.IdentityRefWithVote
	required bool
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

			The command can:
			- Add reviewers as optional or required, promoting/demoting existing reviewers when needed.
			- Remove reviewers regardless of their current required/optional state.
			- Add or remove labels

			Examples:
			  %[1]sazdo pr edit --add-required-reviewer alice@example.com bob@example.com%[1]s
			  %[1]sazdo pr edit --add-optional-reviewer alice@example.com --remove-reviewer bob@example.com%[1]s
			  %[1]sazdo pr edit --add-label bug --remove-label needs-review%[1]s
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

	cmd.Flags().StringSliceVarP(&opts.addRequiredReviewer, "add-required-reviewer", "", nil, "Add or promote required reviewers (comma-separated)")
	cmd.Flags().StringSliceVarP(&opts.addOptionalReviewer, "add-optional-reviewer", "", nil, "Add or demote optional reviewers (comma-separated)")
	cmd.Flags().StringSliceVarP(&opts.removeReviewer, "remove-reviewer", "", nil, "Remove reviewers (comma-separated, use * to remove all)")
	cmd.Flags().StringVarP(&opts.base, "base", "B", "", "Change the base branch for this pull request")
	cmd.Flags().StringVarP(&opts.body, "body", "b", "", "Set the new body.")
	cmd.Flags().StringVarP(&opts.bodyFile, "body-file", "F", "", "Read body text from file (use \"-\" to read from standard input)")
	cmd.Flags().StringVarP(&opts.title, "title", "t", "", "Set the new title.")
	cmd.Flags().StringSliceVarP(&opts.addLabel, "add-label", "", nil, "Add labels (comma-separated)")
	cmd.Flags().StringSliceVarP(&opts.removeLabel, "remove-label", "", nil, "Remove labels (comma-separated, use * to remove all)")

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

	if slices.Contains(opts.removeReviewer, "*") && len(opts.removeReviewer) > 1 {
		return util.FlagErrorf("--remove-reviewer cannot combine \"*\" with other values")
	}

	if slices.Contains(opts.removeLabel, "*") && len(opts.removeLabel) > 1 {
		return util.FlagErrorf("--remove-label cannot combine \"*\" with other values")
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
		return util.FlagErrorf("pull request not found")
	}

	gitClient, err := prRepo.GitClient(ctx.Context(), ctx.ConnectionFactory())
	if err != nil {
		return fmt.Errorf("failed to get Git REST client: %w", err)
	}

	repo, err := prRepo.GitRepository(ctx.Context(), gitClient)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	identityClient, err := ctx.ClientFactory().Identity(ctx.Context(), prRepo.Organization())
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
		base := strings.TrimPrefix(opts.base, "refs/heads/")
		updatePullRequest.TargetRefName = types.ToPtr("refs/heads/" + base)
	}

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

	currentReviewers := make(map[string]git.IdentityRefWithVote)
	if pr.Reviewers != nil {
		for _, reviewer := range *pr.Reviewers {
			if reviewer.Id == nil {
				continue
			}
			currentReviewers[*reviewer.Id] = reviewer
		}
	}

	var remove map[string]git.IdentityRefWithVote
	if slices.Contains(opts.removeReviewer, "*") {
		remove = map[string]git.IdentityRefWithVote{}
		for _, id := range types.MapSlicePtr(pr.Reviewers, func(ident git.IdentityRefWithVote) string {
			return *ident.Id
		}) {
			remove[id] = git.IdentityRefWithVote{
				Id: types.ToPtr(id),
			}
		}
	} else {
		remove, err = buildReviewerMap(ctx.Context(), identityClient, opts.removeReviewer)
		if err != nil {
			return fmt.Errorf("failed to resolve reviewers to remove: %w", err)
		}
	}

	for reviewerID := range remove {
		_, ok := currentReviewers[reviewerID]
		if !ok {
			continue
		}

		err := gitClient.DeletePullRequestReviewer(ctx.Context(), git.DeletePullRequestReviewerArgs{
			RepositoryId:  types.ToPtr(repo.Id.String()),
			PullRequestId: pr.PullRequestId,
			ReviewerId:    types.ToPtr(reviewerID),
			Project:       types.ToPtr(prRepo.Project()),
		})
		if err != nil {
			return fmt.Errorf("failed to remove reviewer %s: %w", reviewerID, err)
		}

		delete(currentReviewers, reviewerID)
	}

	intents := make(map[string]reviewerIntent)
	if err := addReviewerIntents(ctx.Context(), identityClient, opts.addOptionalReviewer, false, intents); err != nil {
		return fmt.Errorf("failed to resolve optional reviewers to add: %w", err)
	}
	if err := addReviewerIntents(ctx.Context(), identityClient, opts.addRequiredReviewer, true, intents); err != nil {
		return fmt.Errorf("failed to resolve required reviewers to add: %w", err)
	}

	var reviewersToAdd []webapi.IdentityRef
	for id, intent := range intents {
		if _, exists := currentReviewers[id]; exists {
			continue
		}
		reviewersToAdd = append(reviewersToAdd, webapi.IdentityRef{Id: intent.ref.Id})
	}

	if len(reviewersToAdd) > 0 {
		_, err = gitClient.CreatePullRequestReviewers(ctx.Context(), git.CreatePullRequestReviewersArgs{
			RepositoryId:  types.ToPtr(repo.Id.String()),
			PullRequestId: pr.PullRequestId,
			Project:       types.ToPtr(prRepo.Project()),
			Reviewers:     &reviewersToAdd,
		})
		if err != nil {
			return fmt.Errorf("failed to add reviewers: %w", err)
		}

		for _, reviewer := range reviewersToAdd {
			if reviewer.Id == nil {
				continue
			}
			currentReviewers[*reviewer.Id] = git.IdentityRefWithVote{
				Id: reviewer.Id,
			}
		}
	}

	for id, intent := range intents {
		reviewer, exists := currentReviewers[id]
		if !exists {
			continue
		}
		currentRequired := reviewer.IsRequired != nil && *reviewer.IsRequired
		if currentRequired == intent.required {
			continue
		}

		_, err := gitClient.CreatePullRequestReviewer(ctx.Context(), git.CreatePullRequestReviewerArgs{
			RepositoryId:  types.ToPtr(repo.Id.String()),
			PullRequestId: pr.PullRequestId,
			Project:       types.ToPtr(prRepo.Project()),
			ReviewerId:    types.ToPtr(id),
			Reviewer: &git.IdentityRefWithVote{
				Id:         types.ToPtr(id),
				IsRequired: types.ToPtr(intent.required),
			},
		})
		if err != nil {
			state := "required"
			if !intent.required {
				state = "optional"
			}
			return fmt.Errorf("failed to set reviewer %s %s: %w", id, state, err)
		}

		reviewer.IsRequired = types.ToPtr(intent.required)
		currentReviewers[id] = reviewer
	}

	// Track existing labels with lowercase keys so we can (a) match user input when
	// removing labels regardless of casing, and (b) skip redundant additions. The map
	// stores the canonical server casing so Delete/Create calls stay precise.
	labelLookup := make(map[string]string)
	if pr.Labels != nil {
		for _, lbl := range *pr.Labels {
			if lbl.Name == nil {
				continue
			}
			labelLookup[strings.ToLower(*lbl.Name)] = *lbl.Name
		}
	}

	if slices.Contains(opts.removeLabel, "*") {
		opts.removeLabel = types.MapSlicePtr(pr.Labels, func(i core.WebApiTagDefinition) string {
			return *i.Name
		})
	}
	for _, raw := range opts.removeLabel {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		if _, exists := labelLookup[lower]; !exists {
			continue
		}

		err := gitClient.DeletePullRequestLabels(ctx.Context(), git.DeletePullRequestLabelsArgs{
			RepositoryId:  types.ToPtr(repo.Id.String()),
			PullRequestId: pr.PullRequestId,
			LabelIdOrName: types.ToPtr(labelLookup[lower]),
			Project:       types.ToPtr(prRepo.Project()),
		})
		if err != nil {
			return fmt.Errorf("failed to remove label %s: %w", name, err)
		}

		delete(labelLookup, lower)
	}

	for _, raw := range opts.addLabel {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		if _, exists := labelLookup[lower]; exists {
			continue
		}

		_, err := gitClient.CreatePullRequestLabel(ctx.Context(), git.CreatePullRequestLabelArgs{
			RepositoryId:  types.ToPtr(repo.Id.String()),
			PullRequestId: pr.PullRequestId,
			Project:       types.ToPtr(prRepo.Project()),
			Label: &core.WebApiCreateTagRequestData{
				Name: types.ToPtr(name),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to add label %s: %w", name, err)
		}

		labelLookup[lower] = name
	}

	fmt.Fprintf(iostreams.Out, "Pull request #%d updated: %s\n", *updatedPr.PullRequestId, *updatedPr.Title)

	return nil
}

func buildReviewerMap(ctx context.Context, identityClient identity.Client, handles []string) (map[string]git.IdentityRefWithVote, error) {
	reviewers := make(map[string]git.IdentityRefWithVote)
	if len(handles) == 0 {
		return reviewers, nil
	}

	resolved, err := shared.GetReviewerIdentities(ctx, identityClient, handles)
	if err != nil {
		return nil, err
	}

	for _, identity := range resolved {
		id := identity.Id.String()
		reviewers[id] = git.IdentityRefWithVote{
			Id: types.ToPtr(id),
		}
	}

	return reviewers, nil
}

func addReviewerIntents(ctx context.Context, identityClient identity.Client, handles []string, required bool, intents map[string]reviewerIntent) error {
	resolved, err := shared.GetReviewerIdentities(ctx, identityClient, handles)
	if err != nil {
		return err
	}

	for _, identity := range resolved {
		id := identity.Id.String()
		intent := intents[id]
		intent.required = intent.required || required
		intent.ref = git.IdentityRefWithVote{
			Id: types.ToPtr(id),
		}

		intents[id] = intent
	}

	return nil
}

func readBodyFile(filename string) ([]byte, error) {
	if filename == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(filename) //nolint:gosec
}
