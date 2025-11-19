package pr

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/checkout"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/close"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/comment"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/diff"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/edit"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/merge"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/reopen"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/view"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/vote"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmdPR(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr <command>",
		Short: "Manage pull requests",
		Long:  "Work with Azure DevOps pull requests.",
		Example: heredoc.Doc(`
			$ azdo pr checkout 353
			$ azdo pr create --fill
		`),
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				A pull request can be supplied as argument in any of the following formats:
				- by number, e.g. "123";
				- by the name of its head branch, e.g. "patch-1" or "OWNER:patch-1".
			`),
		},
		GroupID: "core",
	}

	util.AddGroup(cmd, "General commands",
		list.NewCmd(ctx),
		create.NewCmd(ctx),
	)

	util.AddGroup(cmd, "Targeted commands",
		checkout.NewCmd(ctx),
		close.NewCmd(ctx),
		comment.NewCmd(ctx),
		diff.NewCmd(ctx),
		edit.NewCmd(ctx),
		merge.NewCmd(ctx),
		reopen.NewCmd(ctx),
		vote.NewCmd(ctx),
		view.NewCmd(ctx),
	)

	return cmd
}
