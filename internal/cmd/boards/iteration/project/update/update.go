package update

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

type updateOptions struct {
	scopeArg   string
	startDate  string
	finishDate string
	attributes []string
	exporter   util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &updateOptions{}

	cmd := &cobra.Command{
		Use:   "update [ORGANIZATION/]PROJECT[/PATH]/NAME",
		Short: "Update an iteration in a project.",
		Long: heredoc.Doc(`
			Update an iteration (sprint) in a project. The positional argument identifies
			the iteration as [ORGANIZATION/]PROJECT[/PATH]/NAME.

			Supports changing start/finish dates and setting arbitrary attributes.
		`),
		Example: heredoc.Doc(`
			# Reschedule a sprint
			azdo boards iteration project update Fabrikam/Sprint\ 1 \
				--start-date 2025-01-06 --finish-date 2025-01-19

			# Add or change a custom attribute, keeping the existing dates
			azdo boards iteration project update Fabrikam/Release\ 2025/Sprint\ 1 \
				--attributes goal="Ship login"

			# Combine: reschedule + set a custom attribute
			azdo boards iteration project update myorg/Fabrikam/Release\ 2025/Sprint\ 1 \
				--start-date 2025-01-06 --finish-date 2025-01-19 \
				--attributes goal="Ship login"

			# Emit JSON
			azdo boards iteration project update Fabrikam/Sprint\ 1 --json
		`),
		Aliases: []string{"u", "up"},
		Args:    util.ExactArgs(1, "target argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runUpdate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.startDate, "start-date", "", "New start date (RFC 3339 or YYYY-MM-DD). Wins on conflict with --attributes startDate.")
	cmd.Flags().StringVar(&opts.finishDate, "finish-date", "", "New finish date (RFC 3339 or YYYY-MM-DD). Wins on conflict with --attributes finishDate. Must be on or after start-date when both are set.")
	cmd.Flags().StringSliceVar(&opts.attributes, "attributes", nil, "Custom attribute in key=value form. Repeatable. Existing attributes not mentioned are preserved.")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "identifier", "name", "path", "structureType", "hasChildren", "attributes", "url", "_links",
	})

	return cmd
}

func runUpdate(ctx util.CmdContext, opts *updateOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if strings.TrimSpace(opts.startDate) == "" &&
		strings.TrimSpace(opts.finishDate) == "" &&
		len(opts.attributes) == 0 {
		return util.FlagErrorf("at least one of --start-date, --finish-date, or --attributes is required")
	}

	target, err := util.Parse(ctx, opts.scopeArg, util.ParseOptions{
		AllowImplicitOrg: true,
		RequireProject:   true,
		MinTargets:       1,
		MaxTargets:       64,
	})
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	rawPath := strings.Join(target.Targets, "/")
	nodePath, err := shared.BuildClassificationPath(target.Project, true, "Iteration", rawPath)
	if err != nil {
		return util.FlagErrorf("invalid target %q: %w", opts.scopeArg, err)
	}
	if nodePath == "" {
		return util.FlagErrorf("target must reference a child of /Iteration")
	}

	wit, err := ctx.ClientFactory().WorkItemTracking(ctx.Context(), target.Organization)
	if err != nil {
		return fmt.Errorf("failed to get classification client: %w", err)
	}

	getArgs := workitemtracking.GetClassificationNodeArgs{
		Project:        types.ToPtr(target.Project),
		StructureGroup: types.ToPtr(workitemtracking.TreeStructureGroupValues.Iterations),
		Path:           types.ToPtr(nodePath),
	}

	zap.L().Debug(
		"fetching iteration before update",
		zap.String("organization", target.Organization),
		zap.String("project", target.Project),
		zap.String("path", nodePath),
	)

	existing, err := wit.GetClassificationNode(ctx.Context(), getArgs)
	if err != nil {
		return fmt.Errorf("failed to fetch iteration: %w", err)
	}
	if existing == nil || existing.Id == nil {
		return util.FlagErrorf("existing iteration has no ID; cannot update")
	}

	mergedAttrs, err := buildUpdateAttributes(existing.Attributes, opts.startDate, opts.finishDate, opts.attributes)
	if err != nil {
		return err
	}

	updateArgs := workitemtracking.CreateOrUpdateClassificationNodeArgs{
		Project:        types.ToPtr(target.Project),
		StructureGroup: types.ToPtr(workitemtracking.TreeStructureGroupValues.Iterations),
		Path:           types.ToPtr(nodePath),
		PostedNode: &workitemtracking.WorkItemClassificationNode{
			Id:   existing.Id,
			Name: existing.Name,
		},
	}
	if len(mergedAttrs) > 0 {
		updateArgs.PostedNode.Attributes = &mergedAttrs
	}

	zap.L().Debug(
		"updating iteration",
		zap.String("organization", target.Organization),
		zap.String("project", target.Project),
		zap.String("path", nodePath),
		zap.Int("id", *existing.Id),
		zap.String("name", types.GetValue(existing.Name, "")),
		zap.Int("attributeCount", len(mergedAttrs)),
	)

	res, err := wit.CreateOrUpdateClassificationNode(ctx.Context(), updateArgs)
	if err != nil {
		return fmt.Errorf("failed to update iteration: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, res)
	}

	tp, err := ctx.Printer("table")
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

func buildUpdateAttributes(existing *map[string]any, startDate, finishDate string, attrs []string) (map[string]any, error) {
	result := make(map[string]any)
	if existing != nil {
		for k, v := range *existing {
			result[k] = v
		}
	}

	for _, kv := range attrs {
		idx := strings.Index(kv, "=")
		if idx <= 0 {
			return nil, util.FlagErrorf("invalid --attributes %q: expected key=value", kv)
		}
		key := strings.TrimSpace(kv[:idx])
		if key == "" {
			return nil, util.FlagErrorf("invalid --attributes %q: empty key", kv)
		}
		result[key] = kv[idx+1:]
	}

	if raw := strings.TrimSpace(startDate); raw != "" {
		t, err := parseStrictDate(raw)
		if err != nil {
			return nil, util.FlagErrorf("invalid --start-date: %w", err)
		}
		result["startDate"] = t.UTC().Format(time.RFC3339)
	}
	if raw := strings.TrimSpace(finishDate); raw != "" {
		t, err := parseStrictDate(raw)
		if err != nil {
			return nil, util.FlagErrorf("invalid --finish-date: %w", err)
		}
		result["finishDate"] = t.UTC().Format(time.RFC3339)
	}

	if strings.TrimSpace(startDate) != "" && strings.TrimSpace(finishDate) != "" {
		start, err := time.Parse(time.RFC3339, result["startDate"].(string))
		if err != nil {
			return nil, util.FlagErrorf("invalid --start-date: %w", err)
		}
		finish, err := time.Parse(time.RFC3339, result["finishDate"].(string))
		if err != nil {
			return nil, util.FlagErrorf("invalid --finish-date: %w", err)
		}
		if finish.Before(start) {
			return nil, util.FlagErrorf("--finish-date must be on or after --start-date")
		}
	}

	return result, nil
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
