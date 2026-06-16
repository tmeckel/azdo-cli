package show

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spewerspew/spew"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/types"
)

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
		Use:   "show [ORGANIZATION/]PROJECT/QUEUE",
		Short: "Show details of an agent queue",
		Long: heredoc.Doc(`
			Display the details of a single Azure DevOps agent queue.
			The queue is identified by integer ID or name, with an
			optional organization prefix.
		`),
		Example: heredoc.Doc(`
			# Show a queue by ID
			azdo pipelines queue show Fabrikam/7

			# Show a queue by name
			azdo pipelines queue show 'Fabrikam/Default'

			# Show a queue in a specific organization
			azdo pipelines queue show 'myorg/Fabrikam/Default'
		`),
		Aliases: []string{"view", "status"},
		Args:    util.ExactArgs(1, "queue target is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runShow(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.raw, "raw", "r", false, "Dump raw queue object to stderr")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "name", "pool", "projectId",
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

	scope, err := util.ParseProjectTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	if len(scope.Targets) == 0 {
		return util.FlagErrorf("queue target is required")
	}
	taskClient, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create task agent client: %w", err)
	}

	queueID, err := strconv.Atoi(scope.Targets[0])
	if err == nil {
		if queueID <= 0 {
			return util.FlagErrorf("invalid queue id %d", queueID)
		}
	} else {
		queues, listErr := taskClient.GetAgentQueues(cmdCtx.Context(), taskagent.GetAgentQueuesArgs{
			Project:   types.ToPtr(scope.Project),
			QueueName: types.ToPtr(scope.Targets[0]),
		})
		if listErr != nil {
			return fmt.Errorf("failed to list queues: %w", listErr)
		}

		matchCount := 0
		for _, q := range types.GetValue(queues, []taskagent.TaskAgentQueue{}) {
			if q.Name == nil || !strings.EqualFold(*q.Name, scope.Targets[0]) {
				continue
			}
			if q.Id == nil {
				return fmt.Errorf("queue %q returned without an ID", scope.Targets[0])
			}
			queueID = *q.Id
			matchCount++
		}

		switch {
		case matchCount == 0:
			return fmt.Errorf("queue %q not found", scope.Targets[0])
		case matchCount > 1:
			return fmt.Errorf("multiple queues named %q found; specify the numeric ID", scope.Targets[0])
		}
	}

	zap.L().Debug(
		"fetching queue",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.Int("queueId", queueID),
	)

	queue, err := taskClient.GetAgentQueue(cmdCtx.Context(), taskagent.GetAgentQueueArgs{
		QueueId: types.ToPtr(queueID),
		Project: types.ToPtr(scope.Project),
	})
	if err != nil {
		return fmt.Errorf("failed to get queue: %w", err)
	}
	if queue == nil {
		return fmt.Errorf("queue %q not found", scope.Targets[0])
	}

	if opts.raw {
		ios.StopProgressIndicator()
		spew.Dump(queue)
		return nil
	}

	if opts.exporter != nil {
		ios.StopProgressIndicator()
		return opts.exporter.Write(ios, queue)
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

	return t.ExecuteData(queue)
}
