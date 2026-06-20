package project

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/iteration/project/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/iteration/project/delete"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/iteration/project/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/iteration/project/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/iteration/project/update"
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

			# Show a specific iteration
			azdo boards iteration project show Fabrikam --path "Sprint 1"
		`),
		Aliases: []string{
			"prj",
			"p",
		},
	}

	cmd.AddCommand(create.NewCmd(ctx))
	cmd.AddCommand(delete.NewCmd(ctx))
	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))
	cmd.AddCommand(update.NewCmd(ctx))

	return cmd
}
