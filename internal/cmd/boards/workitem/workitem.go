package workitem

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NewCmd wires subcommands for working with Azure Boards work items.
func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work-item <command>",
		Short: "Work with Azure Boards work items.",
		Example: heredoc.Doc(`
			# List work items in a project
			azdo boards work-item list Fabrikam
		`),
	}

	cmd.AddCommand(list.NewCmd(ctx))

	return cmd
}

