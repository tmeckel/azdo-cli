package restore

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type restoreOptions struct {
	repository string
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &restoreOptions{}

	cmd := &cobra.Command{
		Short: "Restore a deleted repository",
		Use:   "restore [organization/]project/repository",
		Example: heredoc.Doc(`
			# restore a deleted repository in the default organization
			azdo repo list myproject/myrepo

			# restore a deleted repository using specified organization
			azdo repo list myorg/myproject/myrepo
		`),
		Args:    util.ExactArgs(1, "cannot restore: repository argument required"),
		Aliases: []string{"ls"},
		RunE: func(c *cobra.Command, args []string) error {
			opts.repository = args[0]

			return runRestore(ctx, opts)
		},
	}

	return cmd
}

func runRestore(ctx util.CmdContext, opts *restoreOptions) (err error) {
	return util.ErrNotImplemented
}
