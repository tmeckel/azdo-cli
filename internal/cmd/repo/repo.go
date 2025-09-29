package repo

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/repo/clone"
	"github.com/tmeckel/azdo-cli/internal/cmd/repo/create"
	"github.com/tmeckel/azdo-cli/internal/cmd/repo/delete"
	"github.com/tmeckel/azdo-cli/internal/cmd/repo/edit"
	"github.com/tmeckel/azdo-cli/internal/cmd/repo/list"
	"github.com/tmeckel/azdo-cli/internal/cmd/repo/restore"
	"github.com/tmeckel/azdo-cli/internal/cmd/repo/setdefault"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmdRepo(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo <command>",
		Short: "Manage repositories",
		Long:  `Work with Azure DevOps Git repositories.`,
		Example: heredoc.Doc(`
			$ azdo repo create
			$ azdo repo list
			$ azdo repo clone cli/cli
			$ azdo repo view --web
		`),
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				A repository can be supplied as an argument in any of the following formats:
				- "{organization}/{repo}"
				- by URL, e.g. "https://dev.azure.com/{organization}/{repo}"
			`),
		},
		GroupID: "core",
	}

	cmd.AddCommand(list.NewCmdRepoList(ctx))
	cmd.AddCommand(clone.NewCmdRepoClone(ctx))
	cmd.AddCommand(setdefault.NewCmdRepoSetDefault(ctx))
	cmd.AddCommand(restore.NewCmd(ctx))
	cmd.AddCommand(create.NewCmd(ctx))
	cmd.AddCommand(delete.NewCmd(ctx))
	cmd.AddCommand(edit.NewCmd(ctx))
	return cmd
}
