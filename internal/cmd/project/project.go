package project

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/project/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/project/delete"
	"github.com/tmeckel/azdo-cli/internal/cmd/project/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/project/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmdProject(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <command> [flags]",
		Short: "Work with Azure DevOps Projects.",
		Example: heredoc.Doc(`
			# Creatign a new project in the default organization
			$ azdo project create <project>

			# Listing existing project in the default organization
			$ azdo project list

			# Delete a project in an organization
			$ azdo project delete myorg/myproject
		`),
		Aliases: []string{
			"p",
		},
		GroupID: "core",
	}

	cmd.AddCommand(list.NewCmdProjectList(ctx))
	cmd.AddCommand(create.NewCmd(ctx))
	cmd.AddCommand(delete.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))
	return cmd
}
