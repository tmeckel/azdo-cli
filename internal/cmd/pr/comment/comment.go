package comment

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type commentOptions struct {
	selectorArg string
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

	return cmd
}

func runCmd(ctx util.CmdContext, opts *commentOptions) (err error) {
	return util.ErrNotImplemented
}
