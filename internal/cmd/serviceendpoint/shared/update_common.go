package shared

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// updateCommonOptions contains flags/args that apply to all typed update commands.
type updateCommonOptions struct {
	Name        string
	Description string

	WaitReady          bool
	ValidateSchema     bool
	ValidateConnection bool
	GrantAllPipelines  bool

	Timeout  time.Duration
	Exporter util.Exporter
}

// AddUpdateCommonFlags registers the common flags on an update command.
func AddUpdateCommonFlags(cmd *cobra.Command) *cobra.Command {
	common := updateCommonOptions{}
	cmd.Flags().StringVar(&common.Name, "name", "", "New friendly name for the service endpoint")
	cmd.Flags().StringVar(&common.Description, "description", "", "New description for the service endpoint")
	cmd.Flags().BoolVar(&common.WaitReady, "wait", false, "Wait until the endpoint reports ready/failed")
	cmd.Flags().DurationVar(&common.Timeout, "timeout", 2*time.Minute, "Maximum time to wait when --wait or --validate-connection is enabled")
	cmd.Flags().BoolVar(&common.ValidateSchema, "validate-schema", false, "Validate auth scheme/params against endpoint type metadata (opt-in)")
	cmd.Flags().BoolVar(&common.ValidateConnection, "validate-connection", false, "Run TestConnection after update (opt-in)")
	cmd.Flags().BoolVar(&common.GrantAllPipelines, "grant-permission-to-all-pipelines", false, "Grant (true) or revoke (false) access permission to all pipelines")
	util.AddJSONFlags(cmd, &common.Exporter, ServiceEndpointJSONFields)

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, "updateCommonOptions", &common))

	return cmd
}
