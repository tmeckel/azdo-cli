package status

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type statusOptions struct {
	exporter       util.Exporter
	conflictStatus bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &statusOptions{}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of relevant pull requests",
		Args:  util.NoArgsQuoteReminder,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.conflictStatus, "conflict-status", "c", false, "Display the merge conflict status of each pull request")
	util.AddJSONFlags(cmd, &opts.exporter, shared.PullRequestFields)

	return cmd
}

func runCmd(ctx util.CmdContext, opts *statusOptions) (err error) {
	return util.ErrNotImplemented
}
