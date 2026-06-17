package build

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/build/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Manage classic builds",
		Long: heredoc.Doc(`
			Manage classic Azure Pipelines builds (Build v1).
			For modern Pipelines runs, see 'azdo pipelines runs'.
		`),
		Example: heredoc.Doc(`
			# List builds in a project
			azdo pipelines build list Fabrikam
		`),
	}

	cmd.AddCommand(list.NewCmd(ctx))
	return cmd
}
