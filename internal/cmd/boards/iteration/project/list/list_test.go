package list

import (
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/tmeckel/azdo-cli/internal/types"
)

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
