package project

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/project/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmdProject(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <command> [flags]",
		Short: "Work with Azure DevOps Projects.",
		Example: heredoc.Doc(`
			$ azdo project create -o <organization> <project>
			$ azdo project list
			$ azdo project delete -o <organization> <project>
		`),
		GroupID: "core",
	}

	cmd.AddCommand(list.NewCmdProjectList(ctx))
	return cmd
}
