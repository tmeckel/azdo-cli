package show

import (
	_ "embed"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spewerspew/spew"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	agentShared "github.com/tmeckel/azdo-cli/internal/cmd/pipelines/agent/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type templateData struct {
	Pool *taskagent.TaskAgentPool
}

type showOptions struct {
	targetArg string
	raw       bool
	exporter  util.Exporter
}

//go:embed show.tpl
var showTempl string

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]POOL",
		Short: "Show details of an agent pool",
		Long: heredoc.Doc(`
			Display the details of a single Azure DevOps agent pool.
			The pool is identified by integer ID or name, with an
			optional organization prefix.
		`),
		Example: heredoc.Doc(`
			# Show a pool by ID
			azdo pipelines pool show 42

			# Show a pool by name
			azdo pipelines pool show 'Default'

			# Show a pool in a specific organization
			azdo pipelines pool show 'myorg/Default'
		`),
		Aliases: []string{"view", "status"},
		Args:    util.ExactArgs(1, "pool target is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runShow(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.raw, "raw", "r", false, "Dump raw pool object to stderr")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "name", "poolType", "isHosted", "isLegacy",
		"autoProvision", "autoUpdate", "createdOn", "createdBy",
		"owner", "options", "properties", "scope", "size", "targetSize",
	})

	return cmd
}

func runShow(cmdCtx util.CmdContext, opts *showOptions) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	if len(scope.Targets) == 0 {
		return util.FlagErrorf("pool target is required")
	}
	poolTarget := scope.Targets[0]

	taskClient, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create task agent client: %w", err)
	}

	poolID, err := agentShared.ResolvePool(cmdCtx, taskClient, poolTarget)
	if err != nil {
		return err
	}

	zap.L().Debug(
		"fetching pool",
		zap.String("organization", scope.Organization),
		zap.Int("poolId", poolID),
	)

	pool, err := taskClient.GetAgentPool(cmdCtx.Context(), taskagent.GetAgentPoolArgs{
		PoolId: types.ToPtr(poolID),
	})
	if err != nil {
		return fmt.Errorf("failed to get pool: %w", err)
	}
	if pool == nil {
		return fmt.Errorf("pool %q not found", poolTarget)
	}

	if opts.raw {
		ios.StopProgressIndicator()
		spew.Dump(pool)
		return nil
	}

	if opts.exporter != nil {
		ios.StopProgressIndicator()
		return opts.exporter.Write(ios, pool)
	}

	ios.StopProgressIndicator()

	t := template.New(
		ios.Out,
		ios.TerminalWidth(),
		ios.ColorEnabled(),
	).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"hasText": template.HasText,
			"s":       template.StringOrEmpty,
			"u":       template.UUIDString,
		})

	err = t.Parse(showTempl)
	if err != nil {
		return err
	}

	return t.ExecuteData(templateData{Pool: pool})
}
