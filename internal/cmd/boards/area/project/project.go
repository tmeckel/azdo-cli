package project

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	listcmd "github.com/tmeckel/azdo-cli/internal/cmd/boards/area/project/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NewCmd registers project-scoped area commands.
func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <command>",
		Short: "Manage area paths scoped to a project.",
		Example: heredoc.Doc(`
			# List area paths under a project
			azdo boards area project list Fabrikam
		`),
		Aliases: []string{
			"prj",
			"p",
		},
	}

	cmd.AddCommand(listcmd.NewCmd(ctx))

	return cmd
}
