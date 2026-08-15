package workitem

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/delete"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/update"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// NewCmd wires subcommands for working with Azure Boards work items.
func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "work-item <command>",
		Short: "Work with Azure Boards work items.",
		Example: heredoc.Doc(`
			# List work items in a project
			azdo boards work-item list Fabrikam

			# Create a work item
			azdo boards work-item create Fabrikam --type Bug --title "Login is broken"

			# Show a work item's details
			azdo boards work-item show Fabrikam/42 --comments

			# Update a work item's title
			azdo boards work-item update Fabrikam/42 --title "New title"

			# Delete a work item
			azdo boards work-item delete Fabrikam/42 --yes
		`),
	}

	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(create.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))
	cmd.AddCommand(update.NewCmd(ctx))
	cmd.AddCommand(delete.NewCmd(ctx))

	return cmd
}
