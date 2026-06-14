package area

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/area/project"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/area/team"
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

	cmd.AddCommand(project.NewCmd(ctx))
	cmd.AddCommand(team.NewCmd(ctx))

	return cmd
}
