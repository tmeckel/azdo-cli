package team

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/team/create"
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

	return cmd
}
