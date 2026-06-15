package list

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	orgArg   string
	name     string
	poolType string
	maxItems int
	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION]",
		Short: "List agent pools",
		Long: heredoc.Doc(`
			List Azure DevOps agent pools for an organization.
		`),
		Example: heredoc.Doc(`
			# List all pools in the default organization
			azdo pipelines pool list

			# List pools in a specific organization
			azdo pipelines pool list myorg

			# List pools filtered by name
			azdo pipelines pool list myorg --name Default

			# List deployment pools
			azdo pipelines pool list myorg --pool-type deployment

			# Output as JSON
			azdo pipelines pool list myorg --json
		`),
		Aliases: []string{
			"ls",
			"l",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.orgArg = args[0]
			}
			return run(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Filter pools by name")
	util.StringEnumFlag(cmd, &opts.poolType, "pool-type", "", "", []string{
		string(taskagent.TaskAgentPoolTypeValues.Automation),
		string(taskagent.TaskAgentPoolTypeValues.Deployment),
	}, "Filter pools by type")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Optional client-side cap on results")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "isHosted", "isLegacy", "name", "options", "poolType",
		"scope", "size", "agentCloudId", "autoProvision", "autoSize",
		"autoUpdate", "createdBy", "createdOn", "owner", "properties",
		"targetSize",
	})

	return cmd
}

func run(cmdCtx util.CmdContext, opts *opts) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if opts.maxItems < 0 {
		return util.FlagErrorf("invalid --max-items value %d; must be >= 0", opts.maxItems)
	}

	organization, err := util.ParseOrganizationArg(cmdCtx, opts.orgArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	taskClient, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), organization)
	if err != nil {
		return fmt.Errorf("failed to create task agent client: %w", err)
	}

	var poolType *taskagent.TaskAgentPoolType
	if trimmed := strings.TrimSpace(opts.poolType); trimmed != "" {
		value := taskagent.TaskAgentPoolType(strings.ToLower(trimmed))
		poolType = &value
	}

	args := taskagent.GetAgentPoolsArgs{
		PoolName: types.NotZeroPtrOrNil(strings.TrimSpace(opts.name)),
		PoolType: poolType,
	}

	zap.L().Debug(
		"listing agent pools",
		zap.String("organization", organization),
		zap.String("poolName", types.GetValue(args.PoolName, "")),
		zap.String("poolType", string(types.GetValue(args.PoolType, taskagent.TaskAgentPoolType("")))),
	)

	resp, err := taskClient.GetAgentPools(cmdCtx.Context(), args)
	if err != nil {
		return fmt.Errorf("failed to list pools: %w", err)
	}

	pools := []taskagent.TaskAgentPool{}
	if resp != nil {
		pools = *resp
	}

	if opts.maxItems > 0 && len(pools) > opts.maxItems {
		zap.L().Debug("truncating result set to max-items", zap.Int("maxItems", opts.maxItems))
		pools = pools[:opts.maxItems]
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, pools)
	}

	tp, err := cmdCtx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "NAME", "POOL TYPE", "SCOPE", "SIZE", "IS HOSTED", "IS LEGACY", "AUTO PROVISION", "CREATED ON")

	for _, pool := range pools {
		scope := ""
		if pool.Scope != nil {
			scope = pool.Scope.String()
		}

		tp.AddField(fmt.Sprintf("%d", types.GetValue(pool.Id, 0)))
		tp.AddField(types.GetValue(pool.Name, ""))
		tp.AddField(string(types.GetValue(pool.PoolType, "")))
		tp.AddField(scope)
		tp.AddField(fmt.Sprintf("%d", types.GetValue(pool.Size, 0)))
		tp.AddField(fmt.Sprintf("%v", types.GetValue(pool.IsHosted, false)))
		tp.AddField(fmt.Sprintf("%v", types.GetValue(pool.IsLegacy, false)))
		tp.AddField(fmt.Sprintf("%v", types.GetValue(pool.AutoProvision, false)))
		tp.AddField(util.FormatTimeShort(pool.CreatedOn))
		tp.EndRow()
	}

	return tp.Render()
}
