package list

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type listOptions struct {
	scopeArg     string
	path         string
	depth        int
	includeDates bool
	exporter     util.Exporter
	startFilter  string
	finishFilter string
}

type iterationRow struct {
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Level       int        `json:"level"`
	HasChildren bool       `json:"hasChildren"`
	StartDate   *time.Time `json:"startDate,omitempty"`
	FinishDate  *time.Time `json:"finishDate,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{
		depth: 3,
	}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List iteration hierarchy for a project.",
		Long: heredoc.Doc(`
			List the iteration (sprint) hierarchy for a project within an Azure DevOps organization.
		`),
		Example: heredoc.Doc(`
			# List the top-level iterations (depth 3)
			azdo boards iteration project list myorg/myproject

			# List from a specific path
			azdo boards iteration project list myproject --path "Release 2025/Sprint 1"

			# Include start and finish dates
			azdo boards iteration project list myproject --include-dates

			# List iterations starting today or later
			azdo boards iteration project list myproject --start-date ">=today"

			# Filter to iterations finishing before a specific date
			azdo boards iteration project list myproject --finish-date "<=2024-12-31"

			# Export JSON
			azdo boards iteration project list myproject --json name,path,startDate
		`),
		Aliases: []string{"ls", "l"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runList(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.path, "path", "p", "", "Iteration path relative to project root")
	cmd.Flags().IntVarP(&opts.depth, "depth", "d", opts.depth, "Depth to fetch (1-10)")
	cmd.Flags().BoolVar(&opts.includeDates, "include-dates", false, "Include iteration start and finish dates")
	cmd.Flags().StringVar(&opts.startFilter, "start-date", "", "Apply a comparison filter to iteration start dates; supports operators like >= and special value \"today\" (e.g., \">=today\")")
	cmd.Flags().StringVar(&opts.finishFilter, "finish-date", "", "Apply a comparison filter to iteration finish dates; supports operators like <= and special value \"today\" (e.g., \"<=today\")")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"name", "path", "level", "hasChildren", "startDate", "finishDate"})

	return cmd
}

func runList(ctx util.CmdContext, opts *listOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	if opts.depth < 1 || opts.depth > 10 {
		return util.FlagErrorf("--depth must be between 1 and 10")
	}

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	normalizedPath, err := shared.BuildClassificationPath(scope.Project, true, "Iteration", opts.path)
	if err != nil {
		return err
	}

	treeGroup := workitemtracking.TreeStructureGroupValues.Iterations

	witClient, err := ctx.ClientFactory().WorkItemTracking(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create work item tracking client: %w", err)
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	args := workitemtracking.GetClassificationNodeArgs{
		Project:        types.ToPtr(scope.Project),
		StructureGroup: &treeGroup,
		Path: func() *string {
			if normalizedPath == "" {
				return nil
			}
			return &normalizedPath
		}(),
		Depth: &opts.depth,
	}

	root, err := witClient.GetClassificationNode(ctx.Context(), args)
	if err != nil {
		return fmt.Errorf("failed to resolve iteration hierarchy: %w", err)
	}

	rows := make([]iterationRow, 0)
	flattenIterations(root, 1, &rows)
	if len(rows) == 0 {
		return util.NewNoResultsError("no iteration nodes found")
	}

	startConstraint, err := parseDateConstraint(opts.startFilter, "start-date")
	if err != nil {
		return err
	}
	finishConstraint, err := parseDateConstraint(opts.finishFilter, "finish-date")
	if err != nil {
		return err
	}
	if err := ensureFilterCompatibility(startConstraint, finishConstraint); err != nil {
		return err
	}

	rows = filterIterations(rows, startConstraint, finishConstraint)
	if len(rows) == 0 {
		return util.NewNoResultsError("no iteration nodes matched the provided date filters")
	}

	if opts.exporter != nil {
		ios.StopProgressIndicator()
		return opts.exporter.Write(ios, rows)
	}

	tp, err := ctx.Printer("table")
	if err != nil {
		return err
	}

	if opts.includeDates {
		tp.AddColumns("Name", "Path", "Level", "HasChildren", "StartDate", "FinishDate")
	} else {
		tp.AddColumns("Name", "Path", "Level", "HasChildren")
	}
	tp.EndRow()

	for _, row := range rows {
		tp.AddField(row.Name)
		tp.AddField(row.Path)
		tp.AddField(strconv.Itoa(row.Level))
		tp.AddField(strconv.FormatBool(row.HasChildren))
		if opts.includeDates {
			if row.StartDate != nil {
				tp.AddField(row.StartDate.Format(time.RFC3339))
			} else {
				tp.AddField("")
			}
			if row.FinishDate != nil {
				tp.AddField(row.FinishDate.Format(time.RFC3339))
			} else {
				tp.AddField("")
			}
		}
		tp.EndRow()
	}

	ios.StopProgressIndicator()

	return tp.Render()
}

func flattenIterations(node *workitemtracking.WorkItemClassificationNode, level int, rows *[]iterationRow) {
	if node == nil {
		return
	}

	name := types.GetValue(node.Name, "")
	path := shared.NormalizeClassificationPath(types.GetValue(node.Path, ""))
	hasChildren := types.GetValue(node.HasChildren, false)

	row := iterationRow{
		Name:        name,
		Path:        path,
		Level:       level,
		HasChildren: hasChildren,
		StartDate:   extractDate(node.Attributes, "startDate"),
		FinishDate:  extractDate(node.Attributes, "finishDate"),
	}

	*rows = append(*rows, row)

	children := types.GetValue(node.Children, []workitemtracking.WorkItemClassificationNode{})
	for i := range children {
		child := children[i]
		flattenIterations(&child, level+1, rows)
	}
}

func extractDate(attrs *map[string]any, key string) *time.Time {
	if attrs == nil {
		return nil
	}

	raw, ok := (*attrs)[key]
	if !ok || raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			return &parsed
		}
	case time.Time:
		parsed := v
		return &parsed
	case *time.Time:
		if v != nil {
			parsed := *v
			return &parsed
		}
	}

	return nil
}

type comparisonOperator int

const (
	opUnset comparisonOperator = iota
	opEqual
	opLess
	opLessOrEqual
	opGreater
	opGreaterOrEqual
)

type dateConstraint struct {
	operator comparisonOperator
	value    time.Time
	source   string
}

var nowUTC = func() time.Time {
	return time.Now().UTC()
}

func parseDateConstraint(raw string, flagName string) (*dateConstraint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	op, rest := splitOperator(raw)
	if op == opUnset || strings.TrimSpace(rest) == "" {
		return nil, util.FlagErrorf("invalid %s %q: expected comparison operator followed by a RFC3339 or YYYY-MM-DD date", flagName, raw)
	}

	parsed, err := parseFlexibleDate(strings.TrimSpace(rest))
	if err != nil {
		return nil, util.FlagErrorf("invalid %s %q: %v", flagName, raw, err)
	}

	return &dateConstraint{
		operator: op,
		value:    parsed,
		source:   raw,
	}, nil
}

func splitOperator(raw string) (comparisonOperator, string) {
	for _, pair := range []struct {
		prefix string
		op     comparisonOperator
	}{
		{"<=", opLessOrEqual},
		{">=", opGreaterOrEqual},
		{"==", opEqual},
		{"<", opLess},
		{">", opGreater},
	} {
		if strings.HasPrefix(raw, pair.prefix) {
			return pair.op, raw[len(pair.prefix):]
		}
	}

	return opUnset, raw
}

func parseFlexibleDate(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.EqualFold(trimmed, "today") {
		current := nowUTC()
		return time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC), nil
	}

	if strings.Contains(trimmed, "T") {
		return time.Parse(time.RFC3339, trimmed)
	}

	return time.Parse("2006-01-02", trimmed)
}

func ensureFilterCompatibility(start, finish *dateConstraint) error {
	if start == nil || finish == nil {
		return nil
	}

	// handle equality conflicts
	if start.operator == opEqual && finish.operator == opEqual && !start.value.Equal(finish.value) {
		return util.FlagErrorf("start-date %q conflicts with finish-date %q", start.source, finish.source)
	}

	if err := compareBounds(start, finish); err != nil {
		return util.FlagErrorf("start-date %q conflicts with finish-date %q: %v", start.source, finish.source, err)
	}

	return nil
}

func compareBounds(start, finish *dateConstraint) error {
	finishBefore := func(t time.Time) bool { return t.After(finish.value) }
	finishAfter := func(t time.Time) bool { return t.Before(finish.value) }

	switch start.operator {
	case opUnset:
		return nil
	case opEqual:
		switch finish.operator {
		case opLess:
			if !finishBefore(start.value) {
				return fmt.Errorf("finish date must be before %s", start.value.Format(time.RFC3339))
			}
		case opLessOrEqual:
			if finishBefore(start.value) {
				return nil
			}
			if finish.value.Before(start.value) {
				return fmt.Errorf("finish date must not be before %s", start.value.Format(time.RFC3339))
			}
		case opEqual:
			if !finish.value.Equal(start.value) {
				return fmt.Errorf("dates must be equal")
			}
		case opGreater:
			if !finishAfter(start.value) {
				return fmt.Errorf("finish date must be after %s", start.value.Format(time.RFC3339))
			}
		case opGreaterOrEqual:
			if finishAfter(start.value) {
				return nil
			}
			if finish.value.After(start.value) {
				return fmt.Errorf("finish date must not be after %s", start.value.Format(time.RFC3339))
			}
		case opUnset:
			return nil
		}
	case opGreater, opGreaterOrEqual:
		switch finish.operator {
		case opLess, opLessOrEqual:
			if start.value.After(finish.value) {
				return fmt.Errorf("start date cannot be after finish date")
			}
		case opEqual:
			if start.value.After(finish.value) {
				return fmt.Errorf("start date cannot be after finish date")
			}
		case opGreater, opGreaterOrEqual:
			return nil
		case opUnset:
			return nil
		}
	case opLess, opLessOrEqual:
		switch finish.operator {
		case opEqual:
			if start.value.After(finish.value) {
				return fmt.Errorf("start date cannot be after finish date")
			}
		case opLess, opLessOrEqual, opGreater, opGreaterOrEqual, opUnset:
			return nil
		}
	default:
		return nil
	}

	if finish.operator == opEqual && start.value.After(finish.value) {
		return fmt.Errorf("start date cannot be after finish date")
	}

	return nil
}

func filterIterations(rows []iterationRow, start, finish *dateConstraint) []iterationRow {
	if start == nil && finish == nil {
		return rows
	}

	filtered := make([]iterationRow, 0, len(rows))
	for _, row := range rows {
		if satisfiesConstraint(row.StartDate, start) && satisfiesConstraint(row.FinishDate, finish) {
			filtered = append(filtered, row)
		}
	}

	return filtered
}

func satisfiesConstraint(date *time.Time, constraint *dateConstraint) bool {
	if constraint == nil {
		return true
	}
	if date == nil {
		return false
	}

	switch constraint.operator {
	case opEqual:
		return date.Equal(constraint.value)
	case opLess:
		return date.Before(constraint.value)
	case opLessOrEqual:
		return !date.After(constraint.value)
	case opGreater:
		return date.After(constraint.value)
	case opGreaterOrEqual:
		return !date.Before(constraint.value)
	default:
		return true
	}
}
