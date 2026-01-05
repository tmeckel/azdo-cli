package shared

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestAddUpdateCommonFlags(t *testing.T) {
	t.Parallel()

	t.Run("Flags registration", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{
			Use: "test",
		}

		updatedCmd := AddUpdateCommonFlags(cmd)

		// Verify flags exist and have correct metadata
		flags := []struct {
			name      string
			usage     string
			defValue  string
			shortHand string
		}{
			{name: "name", usage: "New friendly name for the service endpoint", defValue: ""},
			{name: "description", usage: "New description for the service endpoint", defValue: ""},
			{name: "wait", usage: "Wait until the endpoint reports ready/failed", defValue: "false"},
			{name: "timeout", usage: "Maximum time to wait when --wait or --validate-connection is enabled", defValue: "2m0s"},
			{name: "validate-schema", usage: "Validate auth scheme/params against endpoint type metadata (opt-in)", defValue: "false"},
			{name: "validate-connection", usage: "Run TestConnection after update (opt-in)", defValue: "false"},
			{name: "grant-permission-to-all-pipelines", usage: "Grant (true) or revoke (false) access permission to all pipelines", defValue: "false"},
		}

		for _, f := range flags {
			flag := updatedCmd.Flag(f.name)
			require.NotNil(t, flag, "flag %s should exist", f.name)
			require.Equal(t, f.usage, flag.Usage, "usage for flag %s", f.name)
			require.Equal(t, f.defValue, flag.DefValue, "default value for flag %s", f.name)
		}

		// Verify JSON flags are added
		require.NotNil(t, updatedCmd.Flag("json"), "json flag should exist")

		// Verify context
		ctx := updatedCmd.Context()
		require.NotNil(t, ctx, "context should not be nil")
		opts := ctx.Value("updateCommonOptions")
		require.NotNil(t, opts, "updateCommonOptions should be in context")
		_, ok := opts.(*updateCommonOptions)
		require.True(t, ok, "context value should be of type *updateCommonOptions")
	})

	t.Run("Flag Parsing", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		AddUpdateCommonFlags(cmd)

		// Parse flags
		err := cmd.ParseFlags([]string{
			"--name", "my-endpoint",
			"--description", "desc",
			"--wait",
			"--timeout", "5m",
			"--validate-schema",
			"--validate-connection",
			"--grant-permission-to-all-pipelines",
		})
		require.NoError(t, err)

		// Retrieve options from context
		ctx := cmd.Context()
		opts := ctx.Value("updateCommonOptions").(*updateCommonOptions)

		require.Equal(t, "my-endpoint", opts.Name)
		require.Equal(t, "desc", opts.Description)
		require.True(t, opts.WaitReady)
		require.Equal(t, 5*time.Minute, opts.Timeout)
		require.True(t, opts.ValidateSchema)
		require.True(t, opts.ValidateConnection)
		require.True(t, opts.GrantAllPipelines)
	})

	t.Run("Invalid flag values", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		AddUpdateCommonFlags(cmd)

		// Parse invalid timeout
		err := cmd.ParseFlags([]string{"--name", "test", "--timeout", "invalid"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument \"invalid\" for \"--timeout\"")
	})

	t.Run("Existing context is preserved", func(t *testing.T) {
		t.Parallel()
		type key string
		ctx := context.WithValue(context.Background(), key("existing"), "value")
		cmd := &cobra.Command{
			Use: "test",
		}
		cmd.SetContext(ctx)

		AddUpdateCommonFlags(cmd)

		updatedCtx := cmd.Context()
		require.Equal(t, "value", updatedCtx.Value(key("existing")))
		require.NotNil(t, updatedCtx.Value("updateCommonOptions"))
	})

	t.Run("Optional name flag", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		AddUpdateCommonFlags(cmd)

		// Disable printing to avoid noise
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		cmd.SetArgs([]string{"--description", "desc"})
		err := cmd.Execute()
		require.NoError(t, err)

		ctx := cmd.Context()
		opts := ctx.Value("updateCommonOptions").(*updateCommonOptions)
		require.Equal(t, "desc", opts.Description)
		require.Equal(t, "", opts.Name)
	})
}
