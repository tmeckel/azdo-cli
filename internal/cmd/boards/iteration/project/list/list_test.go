package list

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	wit        *mocks.MockWorkItemTrackingClient
	stdout     *bytes.Buffer
	org        string
}

func newDependencies(t *testing.T, organization string) *dependencies {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, out, _ := iostreams.Test()
	ioStreams.SetStdoutTTY(false)
	ioStreams.SetStderrTTY(false)

	deps := &dependencies{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		stdout:     out,
		org:        organization,
	}

	deps.cmd.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(deps.wit, nil).AnyTimes()
	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)
	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return(organization, nil).AnyTimes()

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	return deps
}

func listNode(path string) *workitemtracking.WorkItemClassificationNode {
	name := path[strings.LastIndex(path, "\\")+1:]
	hasChildren := false
	return &workitemtracking.WorkItemClassificationNode{
		Name:        &name,
		Path:        &path,
		HasChildren: &hasChildren,
	}
}

func captureListArgs(t *testing.T, deps *dependencies, opts *listOptions, response *workitemtracking.WorkItemClassificationNode) (workitemtracking.GetClassificationNodeArgs, error) {
	t.Helper()

	var got workitemtracking.GetClassificationNodeArgs
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			got = args
			return response, nil
		},
	)

	err := runList(deps.cmd, opts)
	return got, err
}

func requireFlagError(t *testing.T, err error, substr string) {
	t.Helper()
	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), substr)
}

func TestNewCmd_TargetArgRequired(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs(nil)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "project argument required")
}

func TestRunList_DepthBounds(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	err := runList(deps.cmd, &listOptions{scopeArg: "myproject", depth: 0})
	requireFlagError(t, err, "--depth must be between 1 and 10")
}

func TestRunList_RequestArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		depsOrg  string
		scopeArg string
		wantProj string
		wantPath *string
	}{
		{name: "project root uses nil path", depsOrg: "default-org", scopeArg: "myproject", wantProj: "myproject", wantPath: nil},
		{name: "subtree uses positional path", depsOrg: "org", scopeArg: "myproject/Release 2025", wantProj: "myproject", wantPath: types.ToPtr("Release%202025")},
		{name: "explicit org stays explicit when unambiguous", depsOrg: "org", scopeArg: "org/myproject/Release 2025/Sprint 1", wantProj: "myproject", wantPath: types.ToPtr("Release%202025/Sprint%201")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, tc.depsOrg)
			args, err := captureListArgs(t, deps, &listOptions{scopeArg: tc.scopeArg, depth: 3}, listNode("Project\\Iteration\\Sprint 1"))
			require.NoError(t, err)
			assert.Equal(t, tc.wantProj, *args.Project)
			assert.Equal(t, 3, *args.Depth)
			assert.Equal(t, workitemtracking.TreeStructureGroupValues.Iterations, *args.StructureGroup)
			if tc.wantPath == nil {
				assert.Nil(t, args.Path)
			} else {
				require.NotNil(t, args.Path)
				assert.Equal(t, *tc.wantPath, *args.Path)
			}
		})
	}
}

func TestRunList_ClientFactoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, _, _ := iostreams.Test()
	cmd := mocks.NewMockCmdContext(ctrl)
	clientFact := mocks.NewMockClientFactory(ctrl)
	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)

	cmd.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
	cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmd.EXPECT().ClientFactory().Return(clientFact).AnyTimes()
	cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return("default-org", nil).AnyTimes()
	clientFact.EXPECT().WorkItemTracking(gomock.Any(), "default-org").Return(nil, errors.New("boom"))

	err := runList(cmd, &listOptions{scopeArg: "org/Fabrikam", depth: 3})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create work item tracking client")
}

func TestRunList_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "default-org")
	attrs := map[string]any{"startDate": "2025-01-06T00:00:00Z"}
	node := &workitemtracking.WorkItemClassificationNode{
		Name:        types.ToPtr("Sprint 1"),
		Path:        types.ToPtr("myproject\\Iteration\\Sprint 1"),
		HasChildren: types.ToPtr(false),
		Attributes:  &attrs,
	}
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(node, nil)

	err := runList(deps.cmd, &listOptions{scopeArg: "myproject", depth: 3, exporter: util.NewJSONExporter()})

	require.NoError(t, err)
	var got []map[string]any
	require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, "Sprint 1", got[0]["name"])
	assert.Equal(t, "myproject/Iteration/Sprint 1", got[0]["path"])
}

func TestExtractDate(t *testing.T) {
	t.Run("parses RFC3339 string", func(t *testing.T) {
		date := "2024-01-15T13:45:00Z"
		attrs := map[string]any{"startDate": date}
		got := extractDate(&attrs, "startDate")
		if got == nil {
			t.Fatalf("expected date, got nil")
		}
		if got.Format(time.RFC3339) != date {
			t.Fatalf("expected %s, got %s", date, got.Format(time.RFC3339))
		}
	})

	t.Run("returns nil on unknown format", func(t *testing.T) {
		attrs := map[string]any{"startDate": 1234}
		if got := extractDate(&attrs, "startDate"); got != nil {
			t.Fatalf("expected nil for unsupported type, got %v", got)
		}
	})
}

func TestFlattenIterations(t *testing.T) {
	child := workitemtracking.WorkItemClassificationNode{
		Name:        types.ToPtr("Sprint 1"),
		Path:        types.ToPtr("Project/Iteration/Sprint 1"),
		HasChildren: types.ToPtr(false),
	}
	children := []workitemtracking.WorkItemClassificationNode{child}
	attrs := map[string]any{
		"startDate":  "2024-01-01T00:00:00Z",
		"finishDate": "2024-01-15T00:00:00Z",
	}
	root := &workitemtracking.WorkItemClassificationNode{
		Name:        types.ToPtr("Project/Iteration"),
		Path:        types.ToPtr("Project/Iteration"),
		HasChildren: types.ToPtr(true),
		Attributes:  &attrs,
		Children:    &children,
	}

	rows := make([]iterationRow, 0)
	flattenIterations(root, 1, &rows)

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Level != 1 || rows[0].Name != "Project/Iteration" {
		t.Fatalf("unexpected root row: %+v", rows[0])
	}
	if rows[0].StartDate == nil || rows[0].StartDate.Format(time.RFC3339) != "2024-01-01T00:00:00Z" {
		t.Fatalf("expected parsed start date, got %+v", rows[0].StartDate)
	}
	if rows[0].FinishDate == nil || rows[0].FinishDate.Format(time.RFC3339) != "2024-01-15T00:00:00Z" {
		t.Fatalf("expected parsed finish date, got %+v", rows[0].FinishDate)
	}
	if rows[1].Level != 2 || rows[1].Name != "Sprint 1" {
		t.Fatalf("unexpected child row: %+v", rows[1])
	}
	if rows[1].StartDate != nil || rows[1].FinishDate != nil {
		t.Fatalf("child dates should be nil: %+v", rows[1])
	}
}

func TestParseDateConstraint(t *testing.T) {
	fixedNow := time.Date(2024, time.April, 5, 10, 30, 0, 0, time.UTC)
	originalNow := nowUTC
	nowUTC = func() time.Time { return fixedNow }
	defer func() { nowUTC = originalNow }()

	t.Run("parses valid operators and formats", func(t *testing.T) {
		tests := []struct {
			raw    string
			op     comparisonOperator
			value  string
			format string
		}{
			{raw: ">=2024-01-01", op: opGreaterOrEqual, value: "2024-01-01", format: "2006-01-02"},
			{raw: "<=2024-02-01T15:00:00Z", op: opLessOrEqual, value: "2024-02-01T15:00:00Z", format: time.RFC3339},
			{raw: "==2025-12-31", op: opEqual, value: "2025-12-31", format: "2006-01-02"},
			{raw: "< 2023-07-04", op: opLess, value: "2023-07-04", format: "2006-01-02"},
			{raw: "> 2026-03-21T00:00:00Z", op: opGreater, value: "2026-03-21T00:00:00Z", format: time.RFC3339},
			{raw: ">=today", op: opGreaterOrEqual, value: "2024-04-05", format: "2006-01-02"},
		}

		for _, tc := range tests {
			t.Run(tc.raw, func(t *testing.T) {
				got, err := parseDateConstraint(tc.raw, "start-date")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got == nil {
					t.Fatalf("expected constraint")
				}
				if got.operator != tc.op {
					t.Fatalf("expected operator %v, got %v", tc.op, got.operator)
				}
				want, err := time.Parse(tc.format, tc.value)
				if err != nil {
					t.Fatalf("failed parsing test expectation: %v", err)
				}
				if !got.value.Equal(want) {
					t.Fatalf("expected value %s, got %s", want.Format(time.RFC3339), got.value.Format(time.RFC3339))
				}
			})
		}
	})

	t.Run("returns error on missing operator", func(t *testing.T) {
		if _, err := parseDateConstraint("2024-01-01", "start-date"); err == nil {
			t.Fatalf("expected error for missing operator")
		}
	})

	t.Run("returns error on invalid format", func(t *testing.T) {
		if _, err := parseDateConstraint(">=notadate", "start-date"); err == nil {
			t.Fatalf("expected error for invalid date")
		}
	})
}

func TestEnsureFilterCompatibility(t *testing.T) {
	cases := []struct {
		name   string
		start  string
		finish string
		ok     bool
	}{
		{
			name:   "compatible bounds",
			start:  ">=2024-01-01",
			finish: "<=2024-12-31",
			ok:     true,
		},
		{
			name:   "conflicting equality",
			start:  "==2024-01-05",
			finish: "==2024-01-06",
			ok:     false,
		},
		{
			name:   "start after finish",
			start:  ">=2024-02-01",
			finish: "<=2024-01-01",
			ok:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, err := parseDateConstraint(tc.start, "start-date")
			if err != nil {
				t.Fatalf("unexpected error parsing start: %v", err)
			}
			finish, err := parseDateConstraint(tc.finish, "finish-date")
			if err != nil {
				t.Fatalf("unexpected error parsing finish: %v", err)
			}

			err = ensureFilterCompatibility(start, finish)
			if tc.ok && err != nil {
				t.Fatalf("expected compatibility, got error %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected error due to incompatibility")
			}
		})
	}
}

func TestFilterIterations(t *testing.T) {
	date := func(year int, month time.Month, day int) *time.Time {
		tm := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		return &tm
	}

	originalNow := nowUTC
	nowUTC = func() time.Time { return time.Date(2024, time.January, 15, 9, 0, 0, 0, time.UTC) }
	defer func() { nowUTC = originalNow }()

	rows := []iterationRow{
		{Name: "A", StartDate: date(2024, time.January, 1), FinishDate: date(2024, time.January, 10)},
		{Name: "B", StartDate: date(2024, time.February, 1), FinishDate: date(2024, time.February, 10)},
		{Name: "C", StartDate: nil, FinishDate: date(2024, time.March, 1)},
	}

	start, err := parseDateConstraint(">=today", "start-date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	finish, err := parseDateConstraint("<=2024-02-15", "finish-date")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filtered := filterIterations(rows, start, finish)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 row, got %d", len(filtered))
	}
	if filtered[0].Name != "B" {
		t.Fatalf("expected row B, got %s", filtered[0].Name)
	}
}

func TestCompareBounds(t *testing.T) {
	makeConstraint := func(raw, flagName string) *dateConstraint {
		c, err := parseDateConstraint(raw, flagName)
		if err != nil {
			t.Fatalf("failed to parse constraint %q: %v", raw, err)
		}
		return c
	}

	cases := []struct {
		name   string
		start  *dateConstraint
		finish *dateConstraint
		ok     bool
	}{
		{
			name:   "compatible greater and less bounds",
			start:  makeConstraint(">=2024-01-01", "start-date"),
			finish: makeConstraint("<=2024-01-10", "finish-date"),
			ok:     true,
		},
		{
			name:   "start after finish should error",
			start:  makeConstraint(">=2024-01-11", "start-date"),
			finish: makeConstraint("<=2024-01-10", "finish-date"),
			ok:     false,
		},
		{
			name:   "less-or-equal start against equal finish with inverted range",
			start:  makeConstraint("<=2024-01-10", "start-date"),
			finish: makeConstraint("==2024-01-05", "finish-date"),
			ok:     false,
		},
		{
			name:   "unset start constraint ignored",
			start:  &dateConstraint{operator: opUnset},
			finish: makeConstraint("==2024-01-05", "finish-date"),
			ok:     true,
		},
		{
			name:   "unset finish constraint ignored",
			start:  makeConstraint("==2024-01-05", "start-date"),
			finish: &dateConstraint{operator: opUnset},
			ok:     true,
		},
		{
			name:   "greater start and greater finish are compatible",
			start:  makeConstraint(">2024-01-01", "start-date"),
			finish: makeConstraint(">=2024-01-02", "finish-date"),
			ok:     true,
		},
		{
			name:   "less start and less finish are compatible",
			start:  makeConstraint("<2024-01-10", "start-date"),
			finish: makeConstraint("<=2024-01-15", "finish-date"),
			ok:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := compareBounds(tc.start, tc.finish)
			if tc.ok && err != nil {
				t.Fatalf("expected success, got error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected error but got nil")
			}
		})
	}
}
