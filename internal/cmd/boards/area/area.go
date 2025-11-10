package area

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	projectcmd "github.com/tmeckel/azdo-cli/internal/cmd/boards/area/project"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NewCmd wires subcommands for managing area paths.
func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "area <command>",
		Short: "Manage area paths used by Azure Boards.",
		Example: heredoc.Doc(`
			# List the area paths for a project
			azdo boards area project list Fabrikam
		`),
		Aliases: []string{
			"a",
		},
	}

	cmd.AddCommand(projectcmd.NewCmd(ctx))

	return cmd
}
