package variable

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
    "github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/variable/create"
    "github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/variable/delete"
    "github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/variable/list"
    "github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/variable/update"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variable",
		Short: "Manage variables in a variable group",
		Long: heredoc.Doc(`
			Work with variables within Azure Pipelines variable groups.
		`),
		Aliases: []string{"var"},
	}

    cmd.AddCommand(list.NewCmd(ctx))
    cmd.AddCommand(create.NewCmd(ctx))
    cmd.AddCommand(delete.NewCmd(ctx))
    cmd.AddCommand(update.NewCmd(ctx))

	return cmd
}
