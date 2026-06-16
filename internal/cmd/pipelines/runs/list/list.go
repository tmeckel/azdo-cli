package list

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type runOptions struct {
	scopeArg string

	pipelineIDs  []int
	branch       *string
	status       *string
	result       *string
	reason       *string
	requestedFor *string
	tags         []string
	queryOrder   *string

	top      int
	maxItems int

	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &runOptions{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List runs of pipelines in a project.",
		Long: heredoc.Doc(`
			List runs of pipelines in an Azure DevOps project. Mirrors
			'az pipelines runs list'.

			Filters support pipeline, branch, status, result, reason, requester,
			and tags. The full result set is paginated server-side; use
			--max-items to cap the response client-side.
		`),
		Example: heredoc.Doc(`
			# List the 20 most recent runs for a project (default org)
			azdo pipelines runs list Fabrikam --top 20

			# Filter by pipeline and branch
			azdo pipelines runs list MyOrg/Fabrikam --pipeline-id 42 --branch main

			# Order by queue time, descending
			azdo pipelines runs list Fabrikam --query-order queueTimeDescending

			# Export as JSON
			azdo pipelines runs list Fabrikam --json id,buildNumber,status,result
		`),
		Aliases: []string{"l", "ls"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().IntSliceVar(&opts.pipelineIDs, "pipeline-id", nil, "Filter by pipeline IDs (repeatable).")
	util.NilStringFlag(cmd, &opts.branch, "branch", "", "Filter by source branch. Bare names get refs/heads/ prepended.")
	util.NilStringEnumFlag(cmd, &opts.status, "status", "", buildStatusLookup.Keys(), "Filter by status")
	util.NilStringEnumFlag(cmd, &opts.result, "result", "", buildResultLookup.Keys(), "Filter by result")
	util.NilStringEnumFlag(cmd, &opts.reason, "reason", "", buildReasonLookup.Keys(), "Filter by reason")
	util.NilStringFlag(cmd, &opts.requestedFor, "requested-for", "", "Filter by the user who queued the run. Accepts @me to mean the authenticated user.")
	cmd.Flags().StringSliceVar(&opts.tags, "tag", nil, "Filter by tags (all supplied tags must match).")
	util.NilStringEnumFlag(cmd, &opts.queryOrder, "query-order", "", buildQueryOrderLookup.Keys(), "Order the results")
	cmd.Flags().IntVar(&opts.top, "top", 0, "Maximum number of runs to request per server page (0 = server default).")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Maximum number of runs to return client-side (0 = unlimited).")

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "buildNumber", "status", "result", "reason",
		"definition", "project", "sourceBranch", "sourceVersion",
		"startTime", "finishTime", "queueTime", "requestedBy", "requestedFor",
		"tags", "uri", "url",
	})

	return cmd
}

func runCmd(ctx util.CmdContext, opts *runOptions) error {
	if opts.top < 0 {
		return util.FlagErrorf("--top must be >= 0")
	}
	if opts.maxItems < 0 {
		return util.FlagErrorf("--max-items must be >= 0")
	}

	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	client, err := ctx.ClientFactory().Build(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Build client: %w", err)
	}

	requestedFor := types.GetValue(opts.requestedFor, "")
	if strings.EqualFold(requestedFor, "@me") {
		zap.L().Debug("resolving @me to current user identity", zap.String("organization", scope.Organization))

		extensionsClient, err := ctx.ClientFactory().Extensions(ctx.Context(), scope.Organization)
		if err != nil {
			return fmt.Errorf("failed to create Extensions client: %w", err)
		}

		identityClient, err := ctx.ClientFactory().Identity(ctx.Context(), scope.Organization)
		if err != nil {
			return fmt.Errorf("failed to create Identity client: %w", err)
		}

		selfID, err := extensionsClient.GetSelfID(ctx.Context())
		if err != nil {
			return fmt.Errorf("failed to resolve @me identity: %w", err)
		}

		idStr := selfID.String()
		identities, err := identityClient.ReadIdentities(ctx.Context(), identity.ReadIdentitiesArgs{
			IdentityIds: &idStr,
		})
		if err != nil {
			return fmt.Errorf("failed to resolve @me identity details: %w", err)
		}
		if identities == nil || len(*identities) != 1 {
			return fmt.Errorf("failed to resolve @me identity details")
		}

		requestedFor = types.GetValue((*identities)[0].ProviderDisplayName, "")
	}

	project := scope.Project
	bArgs := build.GetBuildsArgs{Project: &project}
	if ids := opts.pipelineIDs; len(ids) > 0 {
		bArgs.Definitions = &opts.pipelineIDs
	}
	if opts.branch != nil {
		branch := *opts.branch
		if !strings.HasPrefix(branch, "refs/") {
			branch = "refs/heads/" + branch
		}
		bArgs.BranchName = &branch
	}
	if opts.status != nil {
		status, ok := buildStatusLookup.GetValue(*opts.status)
		if !ok {
			return util.FlagErrorf("unknown --status value %q", *opts.status)
		}
		bArgs.StatusFilter = types.ToPtr(status)
	}
	if opts.result != nil {
		result, ok := buildResultLookup.GetValue(*opts.result)
		if !ok {
			return util.FlagErrorf("unknown --result value %q", *opts.result)
		}
		bArgs.ResultFilter = types.ToPtr(result)
	}
	if opts.reason != nil {
		reason, ok := buildReasonLookup.GetValue(*opts.reason)
		if !ok {
			return util.FlagErrorf("unknown --reason value %q", *opts.reason)
		}
		bArgs.ReasonFilter = types.ToPtr(reason)
	}
	if requestedFor != "" {
		bArgs.RequestedFor = &requestedFor
	}
	if len(opts.tags) > 0 {
		bArgs.TagFilters = &opts.tags
	}
	if opts.queryOrder != nil {
		order, ok := buildQueryOrderLookup.GetValue(*opts.queryOrder)
		if !ok {
			return util.FlagErrorf("unknown --query-order value %q", *opts.queryOrder)
		}
		bArgs.QueryOrder = types.ToPtr(order)
	}
	if opts.top > 0 {
		bArgs.Top = &opts.top
	}

	var runs []build.Build
paginate:
	for {
		resp, err := client.GetBuilds(ctx.Context(), bArgs)
		if err != nil {
			return fmt.Errorf("GetBuilds: %w", err)
		}
		if resp != nil {
			for _, b := range resp.Value {
				runs = append(runs, b)
				if opts.maxItems > 0 && len(runs) >= opts.maxItems {
					break paginate
				}
			}
		}
		if resp == nil || resp.ContinuationToken == "" {
			break
		}
		token := resp.ContinuationToken
		bArgs.ContinuationToken = &token
	}

	if opts.exporter != nil {
		return opts.exporter.Write(ios, runs)
	}

	tp, err := ctx.Printer("table")
	if err != nil {
		return fmt.Errorf("printer: %w", err)
	}
	tp.AddColumns("ID", "NUMBER", "STATUS", "RESULT", "REASON", "PIPELINE", "BRANCH", "REQUESTED FOR", "STARTED", "FINISHED")
	for i := range runs {
		run := runs[i]
		tp.AddField(strconv.Itoa(types.GetValue(run.Id, 0)))
		tp.AddField(types.GetValue(run.BuildNumber, ""))
		tp.AddField(string(types.GetValue(run.Status, build.BuildStatus(""))))
		tp.AddField(string(types.GetValue(run.Result, build.BuildResult(""))))
		tp.AddField(string(types.GetValue(run.Reason, build.BuildReason(""))))

		var defName string
		if def := run.Definition; def != nil {
			defName = types.GetValue(def.Name, "")
			if defName == "" && def.Id != nil {
				defName = strconv.Itoa(types.GetValue(def.Id, 0))
			}
		}
		tp.AddField(defName)

		tp.AddField(types.GetValue(run.SourceBranch, ""))

		var identName string
		if ref := run.RequestedFor; ref != nil {
			identName = types.GetValue(ref.DisplayName, types.GetValue(ref.UniqueName, ""))
		}
		tp.AddField(identName)

		tp.AddField(util.FormatTimeShort(run.StartTime))
		tp.AddField(util.FormatTimeShort(run.FinishTime))
		tp.EndRow()
	}
	return tp.Render()
}

var buildStatusLookup = types.EnumLookup[build.BuildStatus]{
	"none":       build.BuildStatusValues.None,
	"inprogress": build.BuildStatusValues.InProgress,
	"completed":  build.BuildStatusValues.Completed,
	"cancelling": build.BuildStatusValues.Cancelling,
	"postponed":  build.BuildStatusValues.Postponed,
	"notstarted": build.BuildStatusValues.NotStarted,
	"all":        build.BuildStatusValues.All,
}

var buildResultLookup = types.EnumLookup[build.BuildResult]{
	"none":               build.BuildResultValues.None,
	"succeeded":          build.BuildResultValues.Succeeded,
	"partiallysucceeded": build.BuildResultValues.PartiallySucceeded,
	"failed":             build.BuildResultValues.Failed,
	"canceled":           build.BuildResultValues.Canceled,
}

var buildReasonLookup = types.EnumLookup[build.BuildReason]{
	"none":              build.BuildReasonValues.None,
	"manual":            build.BuildReasonValues.Manual,
	"individualci":      build.BuildReasonValues.IndividualCI,
	"batchedci":         build.BuildReasonValues.BatchedCI,
	"schedule":          build.BuildReasonValues.Schedule,
	"scheduleforced":    build.BuildReasonValues.ScheduleForced,
	"usercreated":       build.BuildReasonValues.UserCreated,
	"validateshelveset": build.BuildReasonValues.ValidateShelveset,
	"checkinshelveset":  build.BuildReasonValues.CheckInShelveset,
	"pullrequest":       build.BuildReasonValues.PullRequest,
	"buildcompletion":   build.BuildReasonValues.BuildCompletion,
	"resourcetrigger":   build.BuildReasonValues.ResourceTrigger,
	"triggered":         build.BuildReasonValues.Triggered,
	"all":               build.BuildReasonValues.All,
}

var buildQueryOrderLookup = types.EnumLookup[build.BuildQueryOrder]{
	"finishtimeascending":  build.BuildQueryOrderValues.FinishTimeAscending,
	"finishtimedescending": build.BuildQueryOrderValues.FinishTimeDescending,
	"queuetimedescending":  build.BuildQueryOrderValues.QueueTimeDescending,
	"queuetimeascending":   build.BuildQueryOrderValues.QueueTimeAscending,
	"starttimedescending":  build.BuildQueryOrderValues.StartTimeDescending,
	"starttimeascending":   build.BuildQueryOrderValues.StartTimeAscending,
}
