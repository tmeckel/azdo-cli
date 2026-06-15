package run

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type runOptions struct {
	targetArg  string
	branch     string
	commitID   string
	variables  []string
	folderPath string
	exporter   util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &runOptions{}

	cmd := &cobra.Command{
		Use:   "run [ORGANIZATION/]PROJECT/PIPELINE",
		Short: "Queue a pipeline run",
		Long: heredoc.Doc(`
			Queue (run) an existing Azure Pipeline definition. The pipeline is
			resolved by positive numeric ID or by name.  Supply --branch,
			--commit-id, and --variable to customise the run.
		`),
		Example: heredoc.Doc(`
			# Queue a run by pipeline ID
			azdo pipelines run Fabrikam/42

			# Queue against a specific branch
			azdo pipelines run MyOrg/Fabrikam/42 --branch main

			# Queue with a commit and a variable
			azdo pipelines run Fabrikam/MyPipeline --commit-id abc123 --variable env=prod
		`),
		Args: util.ExactArgs(1, "pipeline target is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runRun(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.branch, "branch", "", "Branch or ref to build (bare names get refs/heads/ prepended)")
	cmd.Flags().StringVar(&opts.commitID, "commit-id", "", "Source commit SHA to build")
	cmd.Flags().StringSliceVar(&opts.variables, "variable", nil, "Queue-time variable in name=value format (repeatable)")
	cmd.Flags().StringVar(&opts.folderPath, "folder-path", "", "Folder path filter used when resolving a pipeline name")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "buildNumber", "status", "result", "sourceBranch",
		"sourceVersion", "queueTime", "reason",
	})

	return cmd
}

func runRun(cmdCtx util.CmdContext, opts *runOptions) error {
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

	buildClient, err := cmdCtx.ClientFactory().Build(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Build client: %w", err)
	}

	target := strings.TrimSpace(scope.Targets[0])
	if target == "" {
		return util.FlagErrorf("pipeline target cannot be empty")
	}

	pipelineID, err := strconv.Atoi(target)
	if err == nil {
		if pipelineID <= 0 {
			return fmt.Errorf("pipeline id must be greater than zero: %q", target)
		}
	} else {
		defs, err := buildClient.GetDefinitions(cmdCtx.Context(), build.GetDefinitionsArgs{
			Project: types.ToPtr(scope.Project),
			Name:    types.ToPtr(target),
			Path:    types.NotZeroPtrOrNil(opts.folderPath),
		})
		if err != nil {
			return fmt.Errorf("failed to query pipeline definitions: %w", err)
		}
		if defs == nil || len(defs.Value) == 0 {
			return fmt.Errorf("pipeline %q not found", target)
		}

		pipelineID = types.GetValue(defs.Value[0].Id, 0)
		if pipelineID <= 0 {
			return fmt.Errorf("pipeline %q returned empty id", target)
		}
	}

	payload := build.Build{
		Definition: &build.DefinitionReference{
			Id: types.ToPtr(pipelineID),
		},
	}

	if opts.branch != "" {
		payload.SourceBranch = types.ToPtr(normalizeBranch(opts.branch))
	}
	if opts.commitID != "" {
		payload.SourceVersion = types.ToPtr(opts.commitID)
	}
	if len(opts.variables) > 0 {
		params, err := encodeVariables(opts.variables)
		if err != nil {
			return err
		}
		payload.Parameters = params
	}

	zap.L().Debug(
		"queueing build",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.Int("pipelineId", pipelineID),
		zap.String("branch", opts.branch),
	)

	queued, err := buildClient.QueueBuild(cmdCtx.Context(), build.QueueBuildArgs{
		Project: types.ToPtr(scope.Project),
		Build:   &payload,
	})
	if err != nil {
		return fmt.Errorf("failed to queue pipeline %d: %w", pipelineID, err)
	}
	if queued == nil {
		return fmt.Errorf("queue pipeline %d returned empty build", pipelineID)
	}

	zap.L().Debug(
		"build queued",
		zap.Int("runId", types.GetValue(queued.Id, 0)),
		zap.String("buildNumber", types.GetValue(queued.BuildNumber, "")),
	)

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, queued)
	}

	tp, err := cmdCtx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns(
		"Run ID", "Number", "Status", "Result",
		"Pipeline ID", "Pipeline Name",
		"Source Branch", "Queued Time", "Reason",
	)
	tp.AddField(strconv.Itoa(types.GetValue(queued.Id, 0)))
	tp.AddField(types.GetValue(queued.BuildNumber, ""))
	tp.AddField(string(types.GetValue(queued.Status, build.BuildStatus(""))))
	tp.AddField(string(types.GetValue(queued.Result, build.BuildResult(""))))

	if d := queued.Definition; d != nil {
		tp.AddField(strconv.Itoa(types.GetValue(d.Id, 0)))
		tp.AddField(types.GetValue(d.Name, ""))
	} else {
		tp.AddField("")
		tp.AddField("")
	}

	sb := types.GetValue(queued.SourceBranch, "")
	tp.AddField(strings.TrimPrefix(sb, "refs/heads/"))
	tp.AddField(util.FormatTimeShort(queued.QueueTime))
	tp.AddField(string(types.GetValue(queued.Reason, build.BuildReason(""))))
	tp.EndRow()

	return tp.Render()
}

func normalizeBranch(b string) string {
	if strings.HasPrefix(b, "refs/heads/") || strings.HasPrefix(b, "refs/pull/") || strings.HasPrefix(b, "refs/tags/") {
		return b
	}
	return "refs/heads/" + b
}

func encodeVariables(vars []string) (*string, error) {
	m := make(map[string]string, len(vars))
	for _, v := range vars {
		idx := strings.IndexByte(v, '=')
		if idx <= 0 {
			return nil, util.FlagErrorf("invalid variable %q: expected name=value", v)
		}
		name := strings.TrimSpace(v[:idx])
		if name == "" {
			return nil, util.FlagErrorf("invalid variable %q: name cannot be empty", v)
		}
		m[name] = v[idx+1:]
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to encode queue variables: %w", err)
	}
	return types.ToPtr(string(b)), nil
}
