package list

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/agent/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

type opts struct {
	targetArg           string
	filter              string
	includeCapabilities bool
	maxItems            int
	exporter            util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]POOL",
		Short: "List agents in an agent pool",
		Long: heredoc.Doc(`
			List every agent in an Azure DevOps agent pool.
			The pool is identified by a positional target that can be a numeric ID or a name.
		`),
		Example: heredoc.Doc(`
			# List all agents in pool 1
			$ azdo pipelines agent list 1

			# List agents in a named pool
			$ azdo pipelines agent list Default

			# List agents in pool 1 in a specific organization
			$ azdo pipelines agent list "myorg/1"

			# List agents in a named pool in a specific organization
			$ azdo pipelines agent list "myorg/Default"

			# List agents filtered by name
			$ azdo pipelines agent list 1 --filter "my-agent"

			# List agents filtered by name in a specific organization
			$ azdo pipelines agent list "myorg/1" --filter "my-agent"

			# List agents with capabilities included
			$ azdo pipelines agent list 1 --include-capabilities

			# Output as JSON
			$ azdo pipelines agent list 1 --json
		`),
		Aliases: []string{
			"ls",
			"l",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return run(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.filter, "filter", "f", "", "Filter agents by name")
	cmd.Flags().BoolVar(&opts.includeCapabilities, "include-capabilities", false, "Include agent capabilities in the response")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Optional client-side cap on results")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"_links",
		"accessPoint",
		"assignedAgentCloudRequest",
		"assignedRequest",
		"authorization",
		"createdOn",
		"id",
		"lastCompletedRequest",
		"maxParallelism",
		"name",
		"status",
		"enabled",
		"osDescription",
		"pendingUpdate",
		"properties",
		"provisioningState",
		"statusChangedOn",
		"version",
		"systemCapabilities",
		"userCapabilities",
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
		return util.FlagErrorf("invalid --max-items value %d; must be greater than 0", opts.maxItems)
	}

	scope, err := util.ParseTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorf("invalid agent list target: %w", err)
	}
	if scope.Project != "" {
		return util.FlagErrorf("agent list does not accept a project scope; got %q", opts.targetArg)
	}

	org := scope.Organization

	tac, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), org)
	if err != nil {
		return err
	}

	poolID, err := shared.ResolvePool(cmdCtx, tac, scope.Targets[0])
	if err != nil {
		return err
	}

	args := taskagent.GetAgentsArgs{
		PoolId:              types.ToPtr(poolID),
		AgentName:           nil,
		IncludeCapabilities: nil,
	}

	if opts.filter != "" {
		args.AgentName = types.ToPtr(opts.filter)
	}
	if opts.includeCapabilities {
		args.IncludeCapabilities = types.ToPtr(true)
	}

	agents, err := tac.GetAgents(cmdCtx.Context(), args)
	if err != nil {
		return err
	}

	logger := zap.L()

	agentList := *agents

	if opts.maxItems > 0 && len(agentList) > opts.maxItems {
		logger.Debug("truncating result set to max-items", zap.Int("maxItems", opts.maxItems))
		agentList = agentList[:opts.maxItems]
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, agentList)
	}

	tp, err := cmdCtx.Printer("table")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "NAME", "STATUS", "ENABLED", "VERSION", "OS", "CREATED ON")

	for _, a := range agentList {
		id := types.GetValue(a.Id, 0)
		name := types.GetValue(a.Name, "")
		status := ""
		if a.Status != nil {
			status = string(*a.Status)
		}
		enabled := strconv.FormatBool(types.GetValue(a.Enabled, false))
		version := types.GetValue(a.Version, "")
		osDesc := types.GetValue(a.OsDescription, "")
		createdOn := util.FormatTimeShort(a.CreatedOn)

		tp.AddField(fmt.Sprintf("%d", id))
		tp.AddField(name)
		tp.AddField(status)
		tp.AddField(enabled)
		tp.AddField(version)
		tp.AddField(osDesc)
		tp.AddField(createdOn)
		tp.EndRow()
	}

	return tp.Render()
}
