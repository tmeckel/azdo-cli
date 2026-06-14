package show

import (
	_ "embed"
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
)

type showOptions struct {
	exporter util.Exporter
	scopeArg string
}

//go:embed show.tpl
var showTmpl string

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]PROJECT RUN_ID",
		Short: "Show details of a pipeline run",
		Long: heredoc.Doc(`
			Display the details of a single Azure Pipelines run.

			Mirrors 'az pipelines runs show'.
		`),
		Example: heredoc.Doc(`
			# Show a run by ID using the default organization
			azdo pipelines runs show Fabrikam 12345

			# Show a run by ID with explicit organization
			azdo pipelines runs show MyOrg/Fabrikam 12345

			# Export as JSON
			azdo pipelines runs show Fabrikam 12345 --json id,buildNumber,status,result
		`),
		Aliases: []string{"view", "status"},
		Args:    util.ExactArgs(2, "project and run id required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			rawID := args[1]
			return runShow(ctx, opts, rawID)
		},
	}

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "buildNumber", "status", "result", "queueTime", "startTime", "finishTime",
		"url", "definition", "queue", "requestedBy", "requestedFor", "lastChangedBy",
		"sourceVersion", "sourceBranch", "reason", "priority", "tags", "parameters",
		"triggerInfo", "retainedByRelease",
	})

	return cmd
}

func runShow(ctx util.CmdContext, opts *showOptions, rawID string) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	runID, err := strconv.Atoi(rawID)
	if err != nil {
		return util.FlagErrorf("invalid run id %q: must be an integer", rawID)
	}
	if runID <= 0 {
		return util.FlagErrorf("invalid run id %q: must be a positive integer", rawID)
	}
	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	zap.L().Debug(
		"fetching pipeline run",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.Int("runId", runID),
	)

	client, err := ctx.ClientFactory().Build(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Build client: %w", err)
	}

	project := scope.Project
	res, err := client.GetBuild(ctx.Context(), build.GetBuildArgs{
		Project: &project,
		BuildId: &runID,
	})
	if err != nil {
		return fmt.Errorf("GetBuild: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, res)
	}

	t := template.New(
		ios.Out,
		ios.TerminalWidth(),
		ios.ColorEnabled(),
	).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"formatEntity": func(primary, secondary any) string {
				first := template.StringOrEmpty(primary)
				second := template.StringOrEmpty(secondary)
				switch {
				case first != "" && second != "":
					return fmt.Sprintf("%s (%s)", first, second)
				case first != "":
					return first
				default:
					return second
				}
			},
			"formatDuration": func(start, finish *azuredevops.Time) string {
				if start == nil || finish == nil {
					return ""
				}
				return template.FormatDuration(finish.Time.Sub(start.Time))
			},
			"hasItems": template.HasItems,
			"hasText":  template.HasText,
			"s":        template.StringOrEmpty,
		})

	err = t.Parse(showTmpl)
	if err != nil {
		return err
	}

	return t.ExecuteData(res)
}
