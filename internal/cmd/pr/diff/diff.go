package diff

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type diffOptions struct {
	selectorArg string

	color    string
	nameOnly bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &diffOptions{}

	cmd := &cobra.Command{
		Use:   "diff [<number> | <branch> | <url>]",
		Short: "View changes in a pull request",
		Long: heredoc.Docf(`
			View changes in a pull request.
			The output displays a list of changed files and their change types.

			Without an argument, the pull request that belongs to the current branch is selected.
			If there are more than one pull request associated with the current branch, one pull request will be selected based on the shared finder logic.
		`, "`"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.selectorArg = args[0]
			}
			return runCmd(ctx, opts)
		},
	}

	util.StringEnumFlag(cmd, &opts.color, "color", "", "auto", []string{"always", "never", "auto"}, "Use color in diff output")
	cmd.Flags().BoolVar(&opts.nameOnly, "name-only", false, "Display only names of changed files")

	return cmd
}

func runCmd(ctx util.CmdContext, opts *diffOptions) (err error) {
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

	// Fetch pull request iterations to get the latest iteration ID
	iterations, err := gitClient.GetPullRequestIterations(ctx.Context(), git.GetPullRequestIterationsArgs{
		RepositoryId:  types.ToPtr(repo.Id.String()),
		PullRequestId: pr.PullRequestId,
		Project:       types.ToPtr(prRepo.Project()),
	})
	if err != nil {
		return fmt.Errorf("failed to get pull request iterations: %w", err)
	}

	if iterations == nil || len(*iterations) == 0 {
		return fmt.Errorf("no iterations found for pull request")
	}

	// Get the ID of the last iteration
	latestIterationID := (*iterations)[len(*iterations)-1].Id

	// Fetch the changes for the latest iteration
	diffs, err := gitClient.GetPullRequestIterationChanges(ctx.Context(), git.GetPullRequestIterationChangesArgs{
		RepositoryId:  types.ToPtr(repo.Id.String()),
		PullRequestId: pr.PullRequestId,
		IterationId:   latestIterationID,
		Project:       types.ToPtr(prRepo.Project()),
	})
	if err != nil {
		return fmt.Errorf("failed to get pull request diff: %w", err)
	}

	// Process and display the diff, handling both GitItem and map[string]interface{} types
	if opts.nameOnly {
		for _, change := range *diffs.ChangeEntries {
			if gitItem, ok := change.Item.(*git.GitItem); ok && gitItem.Path != nil {
				fmt.Fprintln(iostreams.Out, *gitItem.Path)
				continue
			}
			if m, ok := change.Item.(map[string]any); ok {
				if p, okp := m["path"].(string); okp {
					fmt.Fprintln(iostreams.Out, p)
				}
			}
		}
	} else {
		cs := iostreams.ColorScheme()
		useColor := iostreams.ColorEnabled()
		switch strings.ToLower(opts.color) {
		case "always":
			useColor = true
		case "never":
			useColor = false
		case "auto":
			// use default
		}
		for _, change := range *diffs.ChangeEntries {
			changeType := ""
			if change.ChangeType != nil {
				changeType = string(*change.ChangeType)
			}
			var path string
			if gi, ok := change.Item.(*git.GitItem); ok && gi.Path != nil {
				path = *gi.Path
			} else if m, ok := change.Item.(map[string]any); ok {
				if p, okp := m["path"].(string); okp {
					path = p
				}
			}
			if path == "" {
				continue
			}
			if useColor {
				switch changeType {
				case "add":
					fmt.Fprintf(iostreams.Out, "%s %s\n", cs.Green("+"), path)
				case "edit":
					fmt.Fprintf(iostreams.Out, "%s %s\n", cs.Yellow("~"), path)
				case "delete":
					fmt.Fprintf(iostreams.Out, "%s %s\n", cs.Red("-"), path)
				default:
					fmt.Fprintf(iostreams.Out, "%s %s\n", changeType, path)
				}
			} else {
				fmt.Fprintf(iostreams.Out, "%s %s\n", changeType, path)
			}
		}
	}
	return nil
}
