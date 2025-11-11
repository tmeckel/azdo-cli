package iteration

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/iteration/project"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NewCmd constructs the 'iteration' command group under 'boards'.
func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "iteration <command>",
		Short: "Work with iteration/classification nodes.",
		Example: heredoc.Doc(`
			# List iterations for a project
			azdo boards iteration project list Fabrikam
		`),
		Aliases: []string{
			"it",
			"i",
		},
	}

	// Add subgroups
	cmd.AddCommand(project.NewCmd(ctx))

	return cmd
}
