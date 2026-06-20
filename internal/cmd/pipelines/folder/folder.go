package folder

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/folder/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folder",
		Short: "Manage Azure DevOps pipeline folders",
		Long: heredoc.Doc(`
			Manage Azure DevOps build definition folders. Folders are project-scoped
			and organize pipeline definitions.
		`),
		Aliases: []string{"folders"},
	}

	cmd.AddCommand(list.NewCmd(ctx))
	return cmd
}
