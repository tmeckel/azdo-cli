package project

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/iteration/project/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NewCmd constructs the 'project' subgroup under 'boards iteration'.
func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <command>",
		Short: "Project-scoped iteration commands.",
		Example: heredoc.Doc(`
			# List iterations for a project
			azdo boards iteration project list Fabrikam
		`),
		Aliases: []string{
			"prj",
			"p",
		},
	}

	cmd.AddCommand(list.NewCmd(ctx))

	return cmd
}
