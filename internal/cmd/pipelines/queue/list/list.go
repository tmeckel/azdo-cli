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
	scope        string
	name         string
	actionFilter *string
	maxItems     int
	exporter     util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List agent queues",
		Long: heredoc.Doc(`
			List agent queues in an Azure DevOps project.
		`),
		Example: heredoc.Doc(`
			# List all queues in a project
			azdo pipelines queue list Fabrikam

			# List queues in a specific organization
			azdo pipelines queue list myorg/Fabrikam

			# List queues filtered by name
			azdo pipelines queue list myorg/Fabrikam --name Default

			# Output as JSON
			azdo pipelines queue list Fabrikam --json
		`),
		Aliases: []string{
			"ls",
			"l",
		},
		Args: util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scope = args[0]
			return run(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Filter queues by name")
	util.NilStringEnumFlag(cmd, &opts.actionFilter, "action-filter", "", actionFilterMap.Keys(), "Filter queues by caller permissions")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Optional client-side cap on results")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id",
		"name",
		"pool",
		"projectId",
	})

	return cmd
}

var actionFilterMap = types.EnumLookup[taskagent.TaskAgentQueueActionFilter]{
	"none":   taskagent.TaskAgentQueueActionFilterValues.None,
	"manage": taskagent.TaskAgentQueueActionFilterValues.Manage,
	"use":    taskagent.TaskAgentQueueActionFilterValues.Use,
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

	scopeArg := strings.TrimSpace(opts.scope)
	if strings.Count(scopeArg, "/") > 1 {
		return util.FlagErrorf("invalid project argument: expected [ORGANIZATION/]PROJECT")
	}

	scope, err := util.ParseProjectScope(cmdCtx, scopeArg)
	if err != nil {
		return util.FlagErrorf("invalid project argument: %w", err)
	}
	if len(scope.Targets) != 0 {
		return util.FlagErrorf("invalid project argument: expected [ORGANIZATION/]PROJECT")
	}

	taskClient, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create task agent client: %w", err)
	}

	args := taskagent.GetAgentQueuesArgs{
		Project:   types.ToPtr(scope.Project),
		QueueName: types.NotZeroPtrOrNil(strings.TrimSpace(opts.name)),
	}

	actionFilter, ok := actionFilterMap.GetValuePtr(opts.actionFilter)
	if !ok {
		return util.FlagErrorf("invalid action filter %q; expected none, manage, or use", *opts.actionFilter)
	}
	if actionFilter != nil {
		args.ActionFilter = actionFilter
	}

	zap.L().Debug(
		"listing agent queues",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("queueName", types.GetValue(args.QueueName, "")),
	)

	resp, err := taskClient.GetAgentQueues(cmdCtx.Context(), args)
	if err != nil {
		return fmt.Errorf("failed to list queues: %w", err)
	}

	var queues []taskagent.TaskAgentQueue
	if resp != nil {
		queues = *resp
	}

	if opts.maxItems > 0 && len(queues) > opts.maxItems {
		zap.L().Debug("truncating result set to max-items", zap.Int("maxItems", opts.maxItems))
		queues = queues[:opts.maxItems]
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, queues)
	}

	tp, err := cmdCtx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "NAME", "POOL", "PROJECT")

	for _, q := range queues {
		poolName := ""
		if q.Pool != nil {
			poolName = types.GetValue(q.Pool.Name, "")
		}

		projectID := ""
		if q.ProjectId != nil {
			projectID = q.ProjectId.String()
		}

		tp.AddField(fmt.Sprintf("%d", types.GetValue(q.Id, 0)))
		tp.AddField(types.GetValue(q.Name, ""))
		tp.AddField(poolName)
		tp.AddField(projectID)
		tp.EndRow()
	}

	return tp.Render()
}
