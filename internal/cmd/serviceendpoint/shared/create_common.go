package shared

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

// CreateCommonOptions contains flags/args that apply to all typed create commands.
type createCommonOptions struct {
	Name        string
	Description string

	WaitReady          bool
	ValidateSchema     bool
	ValidateConnection bool
	GrantAllPipelines  bool

	Timeout  time.Duration
	Exporter util.Exporter
}

// AddCreateCommonFlags registers the common flags on a create command.
func AddCreateCommonFlags(cmd *cobra.Command) *cobra.Command {
	common := createCommonOptions{}
	cmd.Flags().StringVar(&common.Name, "name", "", "Name of the service endpoint")
	cmd.Flags().StringVar(&common.Description, "description", "", "Description for the service endpoint")
	cmd.Flags().BoolVar(&common.WaitReady, "wait", false, "Wait until the endpoint reports ready/failed")
	cmd.Flags().DurationVar(&common.Timeout, "timeout", 2*time.Minute, "Maximum time to wait when --wait or --validate-connection is enabled")
	cmd.Flags().BoolVar(&common.ValidateSchema, "validate-schema", false, "Validate auth scheme/params against endpoint type metadata (opt-in)")
	cmd.Flags().BoolVar(&common.ValidateConnection, "validate-connection", false, "Run TestConnection after creation (opt-in)")
	cmd.Flags().BoolVar(&common.GrantAllPipelines, "grant-permission-to-all-pipelines", false, "Grant access permission to all pipelines to use the service connection")
	util.AddJSONFlags(cmd, &common.Exporter, ServiceEndpointJSONFields)

	_ = cmd.MarkFlagRequired("name")

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, "createCommonOptions", &common))

	return cmd
}
