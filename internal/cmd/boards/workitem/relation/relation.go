package relation

import (
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/relation/add"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/relation/remove"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/relation/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NewCmd wires subcommands for working with Azure Boards work item relations.
func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relation <command>",
		Short: "Work with Azure Boards work item relations.",
	}

	cmd.AddCommand(add.NewCmd(ctx))
	cmd.AddCommand(remove.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))

	return cmd
}
