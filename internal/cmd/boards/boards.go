package boards

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/area"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/iteration"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NewCmd constructs the root command for Azure Boards functionality.
func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "boards <command>",
		Short:   "Work with Azure Boards resources.",
		GroupID: "core",
		Example: heredoc.Doc(`
			# List area paths for the default organization's Fabrikam project
			azdo boards area project list Fabrikam
		`),
		Aliases: []string{
			"b",
		},
	}

	cmd.AddCommand(area.NewCmd(ctx))
	cmd.AddCommand(iteration.NewCmd(ctx))

	return cmd
}
