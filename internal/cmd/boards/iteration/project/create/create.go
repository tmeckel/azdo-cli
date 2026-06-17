package create

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type createOptions struct {
	scopeArg   string
	name       string
	path       string
	startDate  string
	finishDate string
	attributes []string
	exporter   util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:   "create [ORGANIZATION/]PROJECT",
		Short: "Create an iteration (sprint) in a project.",
		Example: heredoc.Doc(`
			# Create a top-level iteration
			azdo boards iteration project create Fabrikam --name "Sprint 1"

			# Schedule a sprint with start and finish dates
			azdo boards iteration project create Fabrikam \
				--name "Sprint 2" --start-date 2025-01-06 --finish-date 2025-01-19

			# Create a nested iteration under an existing release
			azdo boards iteration project create myorg/Fabrikam --name "Sprint 2" --path "Release 2025"

			# Set a custom attribute alongside the dates
			azdo boards iteration project create Fabrikam \
				--name "Sprint 1" --start-date 2025-01-06 --finish-date 2025-01-19 \
				--attributes goal="Ship login"

			# Emit JSON
			azdo boards iteration project create Fabrikam --name "Sprint 1" --json
		`),
		Aliases: []string{"c", "cr"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runCreate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Name of the new iteration (required).")
	cmd.Flags().StringVar(&opts.path, "path", "", "Parent iteration path under /Iteration. Omit to create at the project root.")
	cmd.Flags().StringVar(&opts.startDate, "start-date", "", "Iteration start date (RFC 3339 or YYYY-MM-DD).")
	cmd.Flags().StringVar(&opts.finishDate, "finish-date", "", "Iteration finish date (RFC 3339 or YYYY-MM-DD).")
	cmd.Flags().StringSliceVar(&opts.attributes, "attributes", nil, "Custom attribute in key=value form. Repeatable. start-date/finish-date win on key conflict.")
	_ = cmd.MarkFlagRequired("name")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "identifier", "name", "path", "structureType", "hasChildren", "attributes", "url", "_links",
	})

	return cmd
}

func runCreate(ctx util.CmdContext, opts *createOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()
	if parts := strings.Split(strings.TrimSpace(opts.scopeArg), "/"); len(parts) > 2 {
		return util.FlagErrorf("invalid project scope %q: expected [ORGANIZATION/]PROJECT", opts.scopeArg)
	}

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	name := strings.TrimSpace(opts.name)
	if name == "" {
		return util.FlagErrorf("--name must not be empty")
	}

	parentPath, err := shared.BuildClassificationPath(scope.Project, true, "Iteration", opts.path)
	if err != nil {
		return util.FlagErrorf("invalid --path: %w", err)
	}

	attrs, err := buildAttributes(opts)
	if err != nil {
		return err
	}

	postedNode := &workitemtracking.WorkItemClassificationNode{
		Name: types.ToPtr(name),
	}
	if len(attrs) > 0 {
		postedNode.Attributes = &attrs
	}

	args := workitemtracking.CreateOrUpdateClassificationNodeArgs{
		PostedNode:     postedNode,
		Project:        types.ToPtr(scope.Project),
		StructureGroup: types.ToPtr(workitemtracking.TreeStructureGroupValues.Iterations),
	}
	if parentPath != "" {
		args.Path = types.ToPtr(parentPath)
	}

	zap.L().Debug(
		"creating iteration",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("name", name),
		zap.String("parentPath", parentPath),
		zap.Int("attributeCount", len(attrs)),
	)

	wit, err := ctx.ClientFactory().WorkItemTracking(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to get classification client: %w", err)
	}

	res, err := wit.CreateOrUpdateClassificationNode(ctx.Context(), args)
	if err != nil {
		return fmt.Errorf("failed to create iteration: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, res)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("ID", "NAME", "PATH", "START DATE", "FINISH DATE", "HAS CHILDREN")
	tp.AddField(strconv.Itoa(types.GetValue(res.Id, 0)))
	tp.AddField(types.GetValue(res.Name, ""))
	tp.AddField(shared.NormalizeClassificationPath(types.GetValue(res.Path, "")))
	tp.AddField(formatAttributeDate(res.Attributes, "startDate"))
	tp.AddField(formatAttributeDate(res.Attributes, "finishDate"))
	tp.AddField(strconv.FormatBool(types.GetValue(res.HasChildren, false)))
	tp.EndRow()
	return tp.Render()
}

// buildAttributes assembles iteration attributes. Start/finish flags win over --attributes.
func buildAttributes(opts *createOptions) (map[string]any, error) {
	attrs := make(map[string]any)
	var startTime *time.Time
	var finishTime *time.Time

	if raw := strings.TrimSpace(opts.startDate); raw != "" {
		t, err := parseStrictDate(raw)
		if err != nil {
			return nil, util.FlagErrorf("invalid --start-date: %w", err)
		}
		start := t.UTC()
		startTime = &start
	}
	if raw := strings.TrimSpace(opts.finishDate); raw != "" {
		t, err := parseStrictDate(raw)
		if err != nil {
			return nil, util.FlagErrorf("invalid --finish-date: %w", err)
		}
		finish := t.UTC()
		finishTime = &finish
	}
	if startTime != nil && finishTime != nil && finishTime.Before(*startTime) {
		return nil, util.FlagErrorf("--finish-date must be on or after --start-date")
	}
	for _, kv := range opts.attributes {
		idx := strings.Index(kv, "=")
		if idx <= 0 {
			return nil, util.FlagErrorf("invalid --attributes %q: expected key=value", kv)
		}
		key := strings.TrimSpace(kv[:idx])
		if key == "" {
			return nil, util.FlagErrorf("invalid --attributes %q: empty key", kv)
		}
		if _, reserved := attrs[key]; reserved {
			continue
		}
		attrs[key] = kv[idx+1:]
	}
	if startTime != nil {
		attrs["startDate"] = startTime.Format(time.RFC3339)
	}
	if finishTime != nil {
		attrs["finishDate"] = finishTime.Format(time.RFC3339)
	}

	return attrs, nil
}

func parseStrictDate(raw string) (time.Time, error) {
	if strings.Contains(raw, "T") {
		return time.Parse(time.RFC3339, raw)
	}
	return time.Parse("2006-01-02", raw)
}

func formatAttributeDate(attrs *map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	raw, ok := (*attrs)[key]
	if !ok || raw == nil {
		return ""
	}
	if s, ok := raw.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", raw)
}
