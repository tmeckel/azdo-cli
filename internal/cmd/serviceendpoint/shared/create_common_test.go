package shared

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestAddCreateCommonFlags(t *testing.T) {
	t.Parallel()

	t.Run("Flags registration", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{
			Use: "test",
		}

		updatedCmd := AddCreateCommonFlags(cmd)

		// Verify flags exist and have correct metadata
		flags := []struct {
			name      string
			usage     string
			defValue  string
			shortHand string
		}{
			{name: "name", usage: "Name of the service endpoint", defValue: ""},
			{name: "description", usage: "Description for the service endpoint", defValue: ""},
			{name: "wait", usage: "Wait until the endpoint reports ready/failed", defValue: "false"},
			{name: "timeout", usage: "Maximum time to wait when --wait or --validate-connection is enabled", defValue: "2m0s"},
			{name: "validate-schema", usage: "Validate auth scheme/params against endpoint type metadata (opt-in)", defValue: "false"},
			{name: "validate-connection", usage: "Run TestConnection after creation (opt-in)", defValue: "false"},
			{name: "grant-permission-to-all-pipelines", usage: "Grant access permission to all pipelines to use the service connection", defValue: "false"},
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
		opts := ctx.Value("createCommonOptions")
		require.NotNil(t, opts, "createCommonOptions should be in context")
		_, ok := opts.(*createCommonOptions)
		require.True(t, ok, "context value should be of type *createCommonOptions")
	})

	t.Run("Flag Parsing", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		AddCreateCommonFlags(cmd)

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
		opts := ctx.Value("createCommonOptions").(*createCommonOptions)

		require.Equal(t, "my-endpoint", opts.Name)
		require.Equal(t, "desc", opts.Description)
		require.True(t, opts.WaitReady)
		require.Equal(t, 5*time.Minute, opts.Timeout)
		require.True(t, opts.ValidateSchema)
		require.True(t, opts.ValidateConnection)
		require.True(t, opts.GrantAllPipelines)
	})

	t.Run("Required flag", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{
			Use: "test",
		}
		AddCreateCommonFlags(cmd)

		flag := cmd.Flag("name")
		require.NotNil(t, flag)

		// Check if it's marked as required in the command
		// Cobra stores required flags in an internal map, but we can check the annotation
		require.Equal(t, "true", flag.Annotations[cobra.BashCompOneRequiredFlag][0])
	})

	t.Run("Invalid flag values", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		AddCreateCommonFlags(cmd)

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

		AddCreateCommonFlags(cmd)

		updatedCtx := cmd.Context()
		require.Equal(t, "value", updatedCtx.Value(key("existing")))
		require.NotNil(t, updatedCtx.Value("createCommonOptions"))
	})

	t.Run("Required flag enforcement", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{
			Use: "test",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		AddCreateCommonFlags(cmd)

		// Disable printing to avoid noise
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		cmd.SetArgs([]string{"--description", "desc"})
		err := cmd.Execute()
		require.Error(t, err)
		require.Contains(t, err.Error(), "required flag(s) \"name\" not set")
	})
}
