package team

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	listcmd "github.com/tmeckel/azdo-cli/internal/cmd/boards/area/team/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team <command>",
		Short: "Manage area paths scoped to a team.",
		Example: heredoc.Doc(`
			# List team area paths for a project in the default organization
			azdo boards area team list Fabrikam/"My Team"

			# List team area paths for a project in a specific organization
			azdo boards area team list myOrg/Fabrikam/"My Team"
		`),
		Aliases: []string{
			"t",
		},
	}

	cmd.AddCommand(listcmd.NewCmd(ctx))

	return cmd
}
