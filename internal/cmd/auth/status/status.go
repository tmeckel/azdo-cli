package status

import (
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type statusOptions struct {
	ctx util.CmdContext

	OrganizationName string
	ShowToken        bool
}

func NewCmdStatus(ctx util.CmdContext) *cobra.Command {
	opts := &statusOptions{
		ctx: ctx,
	}

	cmd := &cobra.Command{
		Use:   "status",
		Args:  cobra.ExactArgs(0),
		Short: "View authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return statusRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.OrganizationName, "organization", "o", "", "Check a specific oragnizations's auth status")
	cmd.Flags().BoolVarP(&opts.ShowToken, "show-token", "t", false, "Display the auth token")

	return cmd
}

func statusRun(opts *statusOptions) error {
	return nil
}
