package serviceendpoint

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/delete"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/export"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service-endpoint <command> [flags]",
		Short: "Work with Azure DevOps service connections.",
		Long: heredoc.Doc(`
			Manage Azure DevOps service endpoints (service connections) for projects.
		`),
		Aliases: []string{
			"service-endpoints",
			"serviceendpoints",
			"se",
		},
		GroupID: "core",
	}

	cmd.AddCommand(list.NewCmd(ctx))
	cmd.AddCommand(create.NewCmd(ctx))
	cmd.AddCommand(delete.NewCmd(ctx))
	cmd.AddCommand(export.NewCmd(ctx))

	return cmd
}
