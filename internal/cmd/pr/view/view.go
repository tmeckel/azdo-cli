package view

import (
	_ "embed"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spewerspew/spew"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
)

type viewOptions struct {
	exporter     util.Exporter
	selectorArg  string
	showComments bool
	showCommits  bool
	showRaw      bool
}

//go:embed view.tpl
var vtempl string

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &viewOptions{}

	cmd := &cobra.Command{
		Use:   "view [<number> | <branch> | <url>]",
		Short: "View a pull request",
		Long: heredoc.Docf(`
			Display the title, body, and other information about a pull request.

			Without an argument, the pull request that belongs to the current branch
			is displayed.
		`, "`"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.selectorArg = args[0]
			}
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.showComments, "comments", "c", false, "View pull request comments")
	cmd.Flags().BoolVarP(&opts.showCommits, "commits", "C", false, "View pull request commits")
	cmd.Flags().BoolVarP(&opts.showRaw, "raw", "r", false, "View pull request raw")
	util.AddJSONFlags(cmd, &opts.exporter, shared.PullRequestFields)

	return cmd
}

func runCmd(ctx util.CmdContext, opts *viewOptions) (err error) {
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}
	finder, err := shared.NewFinder(ctx)
	if err != nil {
		return err
	}
	pr, _, err := finder.Find(shared.FindOptions{
		Selector: opts.selectorArg,
	})
	if err != nil {
		return
	}

	if pr == nil {
		return fmt.Errorf("pr is nil")
	}

	if opts.showRaw {
		spew.Dump(pr)
		return nil
	}
	t := template.New(
		iostreams.Out,
		iostreams.TerminalWidth(),
		iostreams.ColorEnabled()).
		WithTheme(iostreams.TerminalTheme()).
		WithFuncs(map[string]any{
			"userReviewers": func(reviewers *[]git.IdentityRefWithVote) (*[]git.IdentityRefWithVote, error) {
				rl := []git.IdentityRefWithVote{}
				if len(*reviewers) > 0 {
					for _, r := range *reviewers {
						if r.IsContainer != nil && *r.IsContainer {
							continue
						}
						rl = append(rl, r)
					}
				}
				return &rl, nil
			},
			"vote": func(vote *int) (string, error) {
				if vote == nil {
					return "unknown", nil
				}
				switch *vote {
				case 10:
					return "approved", nil
				case 5:
					return "approved with suggestions", nil
				case 0:
					return "no vote", nil
				case -5:
					return "waiting for author", nil
				case -10:
					return "rejected", nil
				}
				return "", fmt.Errorf("unknown vote %d", *vote)
			},
		})
	err = t.Parse(vtempl)
	if err != nil {
		return err
	}
	err = t.ExecuteData(pr)
	if err != nil {
		return err
	}
	return
}
