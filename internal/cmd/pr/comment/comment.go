package comment

import (
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type commentOptions struct {
	selectorArg string
	comment     string
	threadID    int
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &commentOptions{}

	cmd := &cobra.Command{
		Use:   "comment [<number> | <branch> | <url>]",
		Short: "Comment a pull request",
		Long: heredoc.Docf(`
			Comment an existing pull request.

			Without an argument, the pull request that belongs to the current branch is updated.
			If there are more than one pull request associated with the current branch, one pull request must be selected explicitly.
		`, "`"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.selectorArg = args[0]
			}
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.comment, "comment", "c", "", "Comment to add to the pull request. Use '-' to read from stdin.")
	cmd.Flags().IntVarP(&opts.threadID, "thread", "t", 0, "ID of the thread to reply to.")

	return cmd
}

func runCmd(ctx util.CmdContext, opts *commentOptions) (err error) {
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	finder, err := shared.NewFinder(ctx)
	if err != nil {
		return err
	}
	pr, repo, err := finder.Find(shared.FindOptions{
		Selector: opts.selectorArg,
	})
	if err != nil {
		return err
	}
	if pr == nil {
		return fmt.Errorf("pull request not found")
	}
	var comment string
	if opts.comment != "" {
		if opts.comment == "-" {
			iostreams, err := ctx.IOStreams()
			if err != nil {
				return err
			}
			b, err := io.ReadAll(iostreams.In)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			comment = string(b)
		} else {
			comment = opts.comment
		}
	} else {
		prompter, err := ctx.Prompter()
		if err != nil {
			return err
		}
		comment, err = prompter.Input("Comment:", "")
		if err != nil {
			return err
		}
	}

	gitClient, err := repo.GitClient(ctx.Context(), ctx.ConnectionFactory())
	if err != nil {
		return fmt.Errorf("failed to get Git client: %w", err)
	}

	if opts.threadID > 0 {
		// Create a reply to a thread
		c, err := gitClient.CreateComment(ctx.Context(), git.CreateCommentArgs{
			Comment: &git.Comment{
				Content: &comment,
			},
			RepositoryId:  types.ToPtr(pr.Repository.Id.String()),
			PullRequestId: pr.PullRequestId,
			ThreadId:      &opts.threadID,
			Project:       types.ToPtr(pr.Repository.Project.Id.String()),
		})
		if err != nil {
			return fmt.Errorf("failed to create comment: %w", err)
		}
		fmt.Fprintf(iostreams.Out, "Created comment: %d", *c.Id)
	} else {
		// Create a new thread
		thread := &git.GitPullRequestCommentThread{
			Comments: &[]git.Comment{
				{
					Content: &comment,
				},
			},
		}

		createdThread, err := gitClient.CreateThread(ctx.Context(), git.CreateThreadArgs{
			CommentThread: thread,
			RepositoryId:  types.ToPtr(pr.Repository.Id.String()),
			PullRequestId: pr.PullRequestId,
			Project:       types.ToPtr(pr.Repository.Project.Id.String()),
		})
		if err != nil {
			return fmt.Errorf("failed to create comment thread: %w", err)
		}

		fmt.Fprintf(iostreams.Out, "Created comment: %d", *(*createdThread.Comments)[0].Id)
	}

	return nil
}
