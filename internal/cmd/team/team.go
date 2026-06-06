package team

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/team/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/team/delete"
	"github.com/tmeckel/azdo-cli/internal/cmd/team/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/team/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/team/update"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "team <command>",
		Short:   "Manage Azure DevOps teams.",
		GroupID: "core",
		Aliases: []string{
			"t",
		},
	}

	cmd.AddCommand(create.NewCmd(ctx))
	cmd.AddCommand(delete.NewCmd(ctx))
	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(show.NewCmd(ctx))
	cmd.AddCommand(update.NewCmd(ctx))

	return cmd
}
