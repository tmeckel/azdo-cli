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
	branches     []string
	statuses     []string
	results      []string
	reasons      []string
	requestedFor string
	tags         []string
	queryOrder   string

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

	cmd.Flags().IntSliceVar(&opts.pipelineIDs, "pipeline-id", nil, "Limit to runs for these pipeline IDs (repeatable; first value is honored by the SDK).")
	cmd.Flags().StringSliceVar(&opts.branches, "branch", nil, "Filter by source branch (repeatable; first value is honored by the SDK). Bare names get refs/heads/ prepended.")
	cmd.Flags().StringSliceVar(&opts.statuses, "status", nil, "Filter by status (repeatable; first value is honored). Valid: none, inProgress, completed, cancelling, postponed, notStarted, all.")
	cmd.Flags().StringSliceVar(&opts.results, "result", nil, "Filter by result (repeatable; first value is honored). Valid: none, succeeded, partiallySucceeded, failed, canceled.")
	cmd.Flags().StringSliceVar(&opts.reasons, "reason", nil, "Filter by reason (repeatable; first value is honored). Valid: manual, individualCI, batchedCI, schedule, scheduleForced, userCreated, pullRequest, etc.")
	cmd.Flags().StringVar(&opts.requestedFor, "requested-for", "", "Filter by the user who queued the run. Accepts @me to mean the authenticated user.")
	cmd.Flags().StringSliceVar(&opts.tags, "tag", nil, "Filter by tags (all supplied tags must match).")
	cmd.Flags().StringVar(&opts.queryOrder, "query-order", "", "Order the results: finishTimeAscending, finishTimeDescending, queueTimeAscending, queueTimeDescending, startTimeAscending, startTimeDescending.")
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

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		ios.StopProgressIndicator()
		return util.FlagErrorWrap(err)
	}

	client, err := ctx.ClientFactory().Build(ctx.Context(), scope.Organization)
	if err != nil {
		ios.StopProgressIndicator()
		return fmt.Errorf("failed to create Build client: %w", err)
	}

	requestedFor := opts.requestedFor
	if strings.EqualFold(requestedFor, "@me") {
		zap.L().Debug("resolving @me to current user identity", zap.String("organization", scope.Organization))

		extensionsClient, err := ctx.ClientFactory().Extensions(ctx.Context(), scope.Organization)
		if err != nil {
			ios.StopProgressIndicator()
			return fmt.Errorf("failed to create Extensions client: %w", err)
		}

		identityClient, err := ctx.ClientFactory().Identity(ctx.Context(), scope.Organization)
		if err != nil {
			ios.StopProgressIndicator()
			return fmt.Errorf("failed to create Identity client: %w", err)
		}

		selfID, err := extensionsClient.GetSelfID(ctx.Context())
		if err != nil {
			ios.StopProgressIndicator()
			return fmt.Errorf("failed to resolve @me identity: %w", err)
		}

		idStr := selfID.String()
		identities, err := identityClient.ReadIdentities(ctx.Context(), identity.ReadIdentitiesArgs{
			IdentityIds: &idStr,
		})
		if err != nil {
			ios.StopProgressIndicator()
			return fmt.Errorf("failed to resolve @me identity details: %w", err)
		}
		if identities == nil || len(*identities) != 1 {
			ios.StopProgressIndicator()
			return fmt.Errorf("failed to resolve @me identity details")
		}

		requestedFor = types.GetValue((*identities)[0].ProviderDisplayName, "")
	}

	project := scope.Project
	bArgs := build.GetBuildsArgs{Project: &project}
	if ids := opts.pipelineIDs; len(ids) > 0 {
		first := ids
		bArgs.Definitions = &first
	}
	if len(opts.branches) > 0 {
		branch := opts.branches[0]
		if !strings.HasPrefix(branch, "refs/") {
			branch = "refs/heads/" + branch
		}
		bArgs.BranchName = &branch
	}
	if len(opts.statuses) > 0 {
		status, ok := types.LookupEnum(opts.statuses[0], allBuildStatuses)
		if !ok {
			ios.StopProgressIndicator()
			return util.FlagErrorf("unknown --status value %q", opts.statuses[0])
		}
		bArgs.StatusFilter = &status
	}
	if len(opts.results) > 0 {
		result, ok := types.LookupEnum(opts.results[0], allBuildResults)
		if !ok {
			ios.StopProgressIndicator()
			return util.FlagErrorf("unknown --result value %q", opts.results[0])
		}
		bArgs.ResultFilter = &result
	}
	if len(opts.reasons) > 0 {
		reason, ok := types.LookupEnum(opts.reasons[0], allBuildReasons)
		if !ok {
			ios.StopProgressIndicator()
			return util.FlagErrorf("unknown --reason value %q", opts.reasons[0])
		}
		bArgs.ReasonFilter = &reason
	}
	if requestedFor != "" {
		bArgs.RequestedFor = &requestedFor
	}
	if len(opts.tags) > 0 {
		bArgs.TagFilters = &opts.tags
	}
	if opts.queryOrder != "" {
		order, ok := types.LookupEnum(opts.queryOrder, allBuildQueryOrders)
		if !ok {
			ios.StopProgressIndicator()
			return util.FlagErrorf("unknown --query-order value %q", opts.queryOrder)
		}
		bArgs.QueryOrder = &order
	}
	if opts.top > 0 {
		bArgs.Top = &opts.top
	}

	runs := make([]build.Build, 0)
paginate:
	for {
		resp, err := client.GetBuilds(ctx.Context(), bArgs)
		if err != nil {
			ios.StopProgressIndicator()
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

	ios.StopProgressIndicator()

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
			if def.Name != nil && *def.Name != "" {
				defName = *def.Name
			} else if def.Id != nil {
				defName = strconv.Itoa(*def.Id)
			}
		}
		tp.AddField(defName)

		tp.AddField(types.GetValue(run.SourceBranch, ""))

		var identName string
		if ref := run.RequestedFor; ref != nil {
			if name := types.GetValue(ref.DisplayName, ""); name != "" {
				identName = name
			} else {
				identName = types.GetValue(ref.UniqueName, "")
			}
		}
		tp.AddField(identName)

		tp.AddField(util.FormatTimeShort(run.StartTime))
		tp.AddField(util.FormatTimeShort(run.FinishTime))
		tp.EndRow()
	}
	return tp.Render()
}

var allBuildStatuses = []build.BuildStatus{
	build.BuildStatusValues.None,
	build.BuildStatusValues.InProgress,
	build.BuildStatusValues.Completed,
	build.BuildStatusValues.Cancelling,
	build.BuildStatusValues.Postponed,
	build.BuildStatusValues.NotStarted,
	build.BuildStatusValues.All,
}

var allBuildResults = []build.BuildResult{
	build.BuildResultValues.None,
	build.BuildResultValues.Succeeded,
	build.BuildResultValues.PartiallySucceeded,
	build.BuildResultValues.Failed,
	build.BuildResultValues.Canceled,
}

var allBuildReasons = []build.BuildReason{
	build.BuildReasonValues.None,
	build.BuildReasonValues.Manual,
	build.BuildReasonValues.IndividualCI,
	build.BuildReasonValues.BatchedCI,
	build.BuildReasonValues.Schedule,
	build.BuildReasonValues.ScheduleForced,
	build.BuildReasonValues.UserCreated,
	build.BuildReasonValues.ValidateShelveset,
	build.BuildReasonValues.CheckInShelveset,
	build.BuildReasonValues.PullRequest,
	build.BuildReasonValues.BuildCompletion,
	build.BuildReasonValues.ResourceTrigger,
	build.BuildReasonValues.Triggered,
	build.BuildReasonValues.All,
}

var allBuildQueryOrders = []build.BuildQueryOrder{
	build.BuildQueryOrderValues.FinishTimeAscending,
	build.BuildQueryOrderValues.FinishTimeDescending,
	build.BuildQueryOrderValues.QueueTimeDescending,
	build.BuildQueryOrderValues.QueueTimeAscending,
	build.BuildQueryOrderValues.StartTimeDescending,
	build.BuildQueryOrderValues.StartTimeAscending,
}
