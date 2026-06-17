package list

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type listOptions struct {
	scopeArg string

	definitionIDs []int
	branch        *string
	buildNumber   *string

	status       *string
	result       *string
	reason       *string
	tags         []string
	requestedFor *string

	top      int
	maxItems int

	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List classic build results in a project.",
		Long: heredoc.Doc(`
			List classic build (Build v1) records in a project. Supports filter,
			pagination, and JSON export. For the modern Pipelines runs surface,
			see 'azdo pipelines runs list'.
		`),
		Example: heredoc.Doc(`
			# List the 20 most recent builds for a project
			azdo pipelines build list Fabrikam --top 20

			# Filter by branch, status, and tag
			azdo pipelines build list Fabrikam --branch main --status completed --tag release

			# Export as JSON
			azdo pipelines build list Fabrikam --json id,buildNumber,status,result
		`),
		Aliases: []string{"ls", "l"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runList(ctx, opts)
		},
	}

	cmd.Flags().IntSliceVar(&opts.definitionIDs, "definition-id", nil, "Limit to builds for these definition IDs (repeatable)")
	util.NilStringFlag(cmd, &opts.branch, "branch", "", "Limit to builds for this branch. Bare names get refs/heads/ prepended.")
	util.NilStringFlag(cmd, &opts.buildNumber, "build-number", "", "Limit to builds that match this build number. Append * for prefix search.")
	util.NilStringEnumFlag(cmd, &opts.status, "status", "", buildStatusLookup.Keys(), "Limit to builds with this status")
	util.NilStringEnumFlag(cmd, &opts.result, "result", "", buildResultLookup.Keys(), "Limit to builds with this result")
	util.NilStringEnumFlag(cmd, &opts.reason, "reason", "", buildReasonLookup.Keys(), "Limit to builds with this reason")
	cmd.Flags().StringSliceVar(&opts.tags, "tag", nil, "Limit to builds that have all of the specified tags (repeatable)")
	util.NilStringFlag(cmd, &opts.requestedFor, "requested-for", "", "Limit to builds requested for this user or group; supports @me")
	cmd.Flags().IntVar(&opts.top, "top", 0, "Maximum number of builds to return per page (server-side; 0 = server default)")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Maximum number of builds to return across all pages (client-side; 0 = unlimited)")

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "buildNumber", "status", "result", "reason",
		"queueTime", "startTime", "finishTime",
		"sourceBranch", "sourceVersion",
		"definition", "project", "requestedBy", "requestedFor",
		"tags", "uri", "url",
	})

	return cmd
}

func runList(ctx util.CmdContext, opts *listOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if opts.top < 0 {
		return util.FlagErrorf("--top must be >= 0")
	}
	if opts.maxItems < 0 {
		return util.FlagErrorf("--max-items must be >= 0")
	}

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	client, err := ctx.ClientFactory().Build(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Build client: %w", err)
	}

	args := build.GetBuildsArgs{Project: &scope.Project}
	if ids := types.Unique(opts.definitionIDs); len(ids) > 0 {
		args.Definitions = &ids
	}
	if opts.branch != nil {
		branch := *opts.branch
		if !strings.HasPrefix(branch, "refs/") {
			branch = "refs/heads/" + branch
		}
		args.BranchName = &branch
	}
	if opts.buildNumber != nil {
		args.BuildNumber = opts.buildNumber
	}
	if status, ok := buildStatusLookup.GetValuePtr(opts.status); !ok {
		return util.FlagErrorf("unknown --status value %q", types.GetValue(opts.status, ""))
	} else if status != nil {
		args.StatusFilter = status
	}
	if result, ok := buildResultLookup.GetValuePtr(opts.result); !ok {
		return util.FlagErrorf("unknown --result value %q", types.GetValue(opts.result, ""))
	} else if result != nil {
		args.ResultFilter = result
	}
	if reason, ok := buildReasonLookup.GetValuePtr(opts.reason); !ok {
		return util.FlagErrorf("unknown --reason value %q", types.GetValue(opts.reason, ""))
	} else if reason != nil {
		args.ReasonFilter = reason
	}
	if len(opts.tags) > 0 {
		args.TagFilters = &opts.tags
	}
	if requestedFor := types.GetValue(opts.requestedFor, ""); requestedFor != "" {
		if strings.EqualFold(requestedFor, "@me") {
			extClient, err := ctx.ClientFactory().Extensions(ctx.Context(), scope.Organization)
			if err != nil {
				return fmt.Errorf("failed to create Extensions client: %w", err)
			}
			ident, err := extClient.ResolveCurrentIdentity(ctx.Context())
			if err != nil {
				return err
			}
			if m, ok := ident.Properties.(map[string]any); ok {
				if raw, ok := m["Account"]; ok && raw != nil {
					if account, ok := raw.(map[string]any); ok {
						if v, ok := account["$value"].(string); ok && v != "" {
							requestedFor = v
						}
					}
				}
			}
			if requestedFor == "" {
				requestedFor = types.GetValue(ident.ProviderDisplayName, "")
			}
			if requestedFor == "" {
				return fmt.Errorf("authenticated identity is missing account or display name")
			}
		}
		args.RequestedFor = &requestedFor
	}
	if opts.top > 0 {
		args.Top = &opts.top
	}

	builds := make([]build.Build, 0)
paginate:
	for {
		resp, err := client.GetBuilds(ctx.Context(), args)
		if err != nil {
			return fmt.Errorf("failed to list builds: %w", err)
		}
		if resp != nil && len(resp.Value) > 0 {
			for _, b := range resp.Value {
				builds = append(builds, b)
				if opts.maxItems > 0 && len(builds) >= opts.maxItems {
					break paginate
				}
			}
		}
		if resp == nil || resp.ContinuationToken == "" {
			break
		}
		token := resp.ContinuationToken
		args.ContinuationToken = &token
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, builds)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "NUMBER", "STATUS", "RESULT", "REASON", "DEFINITION", "BRANCH", "REQUESTED FOR", "STARTED", "FINISHED")
	for _, b := range builds {
		defName := ""
		if b.Definition != nil {
			defName = types.GetValue(b.Definition.Name, "")
		}
		requestedFor := ""
		if ref := b.RequestedFor; ref != nil {
			if v := types.GetValue(ref.DisplayName, ""); v != "" {
				requestedFor = v
			} else {
				requestedFor = types.GetValue(ref.UniqueName, "")
			}
		}
		status := ""
		if b.Status != nil {
			status = string(*b.Status)
		}
		result := ""
		if b.Result != nil {
			result = string(*b.Result)
		}
		reason := ""
		if b.Reason != nil {
			reason = string(*b.Reason)
		}
		tp.AddField(fmt.Sprintf("%d", types.GetValue(b.Id, 0)))
		tp.AddField(types.GetValue(b.BuildNumber, ""))
		tp.AddField(status)
		tp.AddField(result)
		tp.AddField(reason)
		tp.AddField(defName)
		tp.AddField(types.GetValue(b.SourceBranch, ""))
		tp.AddField(requestedFor)
		tp.AddField(util.FormatTimeShort(b.StartTime))
		tp.AddField(util.FormatTimeShort(b.FinishTime))
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
