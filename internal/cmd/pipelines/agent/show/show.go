package show

import (
	_ "embed"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spewerspew/spew"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/agent/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type templateData struct {
	Agent               *taskagent.TaskAgent
	PoolName            string
	IncludeCapabilities bool
}

type showOptions struct {
	targetArg           string
	includeCapabilities bool
	raw                 bool
	exporter            util.Exporter
}

//go:embed show.tpl
var showTempl string

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]POOL/AGENT",
		Short: "Show details of a pipeline agent",
		Long: heredoc.Doc(`
			Display the details of a single Azure DevOps pipeline agent.
			The agent is specified as a pool and agent ID or name, with
			an optional organization prefix.
		`),
		Example: heredoc.Doc(`
			# Show an agent by pool ID and agent ID
			azdo pipelines agent show 1/42

			# Show an agent by pool name and agent name
			azdo pipelines agent show 'Default/my-agent'

			# Show an agent in a specific organization
			azdo pipelines agent show 'myorg/Default/my-agent'

			# Show an agent with capabilities
			azdo pipelines agent show 1/42 --include-capabilities

			# Show agent as JSON
			azdo pipelines agent show 1/42 --json
		`),
		Aliases: []string{
			"view",
			"status",
		},
		Args: util.ExactArgs(1, "agent target is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runShow(ctx, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.includeCapabilities, "include-capabilities", false, "Include system and user capabilities in the output")
	cmd.Flags().BoolVarP(&opts.raw, "raw", "r", false, "Dump raw agent object to stderr")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "name", "pool", "status", "enabled", "version", "osDescription",
		"accessPoint", "provisioningState", "maxParallelism",
		"createdOn", "statusChangedOn", "createdBy", "authorization",
		"systemCapabilities", "userCapabilities", "assignedRequest",
		"lastCompletedRequest", "pendingUpdate", "properties", "_links",
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

	scope, err := util.ParsePoolAgentTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	poolTarget := scope.Targets[0]
	agentTarget := scope.Targets[1]

	taskClient, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create task agent client: %w", err)
	}

	agent, err := shared.ResolvePoolAgent(cmdCtx, taskClient, scope.Organization, poolTarget, agentTarget)
	if err != nil {
		return err
	}

	zap.L().Debug(
		"resolved agent",
		zap.String("organization", scope.Organization),
		zap.String("pool", poolTarget),
		zap.String("agent", agentTarget),
		zap.Int("agentId", types.GetValue(agent.Id, 0)),
	)

	if opts.raw {
		ios.StopProgressIndicator()
		spew.Dump(agent)
		return nil
	}

	if opts.exporter != nil {
		ios.StopProgressIndicator()
		return opts.exporter.Write(ios, agent)
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
		})
	err = t.Parse(showTempl)
	if err != nil {
		return err
	}

	return t.ExecuteData(templateData{
		Agent:               agent,
		PoolName:            poolTarget,
		IncludeCapabilities: opts.includeCapabilities,
	})
}
