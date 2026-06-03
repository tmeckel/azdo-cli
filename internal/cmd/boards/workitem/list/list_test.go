package list

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
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

func ctrlFromT(t *testing.T) *gomock.Controller {
	c := gomock.NewController(t)
	t.Cleanup(c.Finish)
	return c
}

func TestResolveSort(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		values  []string
		want    string
		wantErr string
	}{
		{name: "no values uses default", values: nil, want: "ORDER BY [System.ChangedDate] DESC"},
		{name: "single field default desc", values: []string{"changed"}, want: "ORDER BY [System.ChangedDate] DESC"},
		{name: "single field explicit desc", values: []string{"changed:desc"}, want: "ORDER BY [System.ChangedDate] DESC"},
		{name: "single field asc", values: []string{"title:asc"}, want: "ORDER BY [System.Title] ASC"},
		{name: "multiple fields", values: []string{"state", "id:desc"}, want: "ORDER BY [System.State] ASC, [System.Id] DESC"},
		{name: "duplicate identical explicit ignored", values: []string{"title:asc", "title:asc"}, want: "ORDER BY [System.Title] ASC"},
		{name: "duplicate identical effective ignored", values: []string{"title", "title:asc"}, want: "ORDER BY [System.Title] ASC"},
		{name: "invalid field", values: []string{"banana"}, wantErr: "invalid --sort field"},
		{name: "invalid direction", values: []string{"id:sideways"}, wantErr: "invalid --sort direction"},
		{name: "conflicting direction", values: []string{"title:asc", "title:desc"}, wantErr: "conflicting --sort directives"},
		{name: "default direction is desc for changed/created/id", values: []string{"id"}, want: "ORDER BY [System.Id] DESC"},
		{name: "default direction is asc for others", values: []string{"state"}, want: "ORDER BY [System.State] ASC"},
		{name: "all field mappings", values: []string{"created:asc", "assigned-to", "type", "tags:desc"}, want: "ORDER BY [System.CreatedDate] ASC, [System.AssignedTo] ASC, [System.WorkItemType] ASC, [System.Tags] DESC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveSort(tc.values)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRunList_SortDefaultUnchanged(t *testing.T) {
	t.Parallel()
	deps := setupFakeDeps(t, "org")
	stubDefaultOpenTypes(deps)
	stubBatch(t, deps, false)

	var captured string
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			ids := []int{1}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &[]workitemtracking.WorkItemReference{{Id: &ids[0]}}}, nil
		},
	)

	err := runList(deps.cmd, &listOptions{scopeArg: "org/Fabrikam"})
	require.NoError(t, err)
	assert.Contains(t, captured, "ORDER BY [System.ChangedDate] DESC")
}

func TestRunList_SortTitleAsc(t *testing.T) {
	t.Parallel()
	deps := setupFakeDeps(t, "org")
	stubDefaultOpenTypes(deps)
	stubBatch(t, deps, false)

	var captured string
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			ids := []int{1}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &[]workitemtracking.WorkItemReference{{Id: &ids[0]}}}, nil
		},
	)

	err := runList(deps.cmd, &listOptions{
		scopeArg: "org/Fabrikam",
		sort:     []string{"title:asc"},
	})
	require.NoError(t, err)
	assert.Contains(t, captured, "ORDER BY [System.Title] ASC")
}

func TestRunList_SortInvalidField(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	deps := &fakeListDeps{
		cmd:    mocks.NewMockCmdContext(ctrlFromT(t)),
		stdout: &bytes.Buffer{},
	}
	deps.cmd.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

	err := runList(deps.cmd, &listOptions{
		scopeArg: "org/Fabrikam",
		sort:     []string{"banana"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --sort field")
}

func TestParseDateBound(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		flag    string
		want    string
		wantErr string
	}{
		{name: "empty returns empty", raw: "", flag: "--changed-after", want: ""},
		{name: "RFC3339", raw: "2025-01-18T12:34:56Z", flag: "--changed-after", want: "2025-01-18T12:34:56Z"},
		{name: "date only", raw: "2025-01-18", flag: "--changed-after", want: "2025-01-18T00:00:00Z"},
		{name: "today UTC midnight", raw: "today", flag: "--created-after", want: time.Now().UTC().Format("2006-01-02") + "T00:00:00Z"},
		{name: "TODAY case insensitive", raw: "TODAY", flag: "--created-after", want: time.Now().UTC().Format("2006-01-02") + "T00:00:00Z"},
		{name: "invalid string", raw: "not-a-date", flag: "--changed-after", wantErr: "invalid --changed-after"},
		{name: "flag name in error", raw: "garbage", flag: "--created-after", wantErr: "--created-after"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDateBound(tc.raw, tc.flag)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRunList_ChangedAfterRFC3339(t *testing.T) {
	t.Parallel()
	deps := setupFakeDeps(t, "org")
	stubDefaultOpenTypes(deps)
	stubBatch(t, deps, false)

	var captured string
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			ids := []int{1}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &[]workitemtracking.WorkItemReference{{Id: &ids[0]}}}, nil
		},
	)

	err := runList(deps.cmd, &listOptions{
		scopeArg:     "org/Fabrikam",
		changedAfter: "2025-01-18T00:00:00Z",
	})
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.ChangedDate] >= '2025-01-18T00:00:00Z'")
}

func TestRunList_InvalidDateFlag(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	deps := &fakeListDeps{
		cmd:    mocks.NewMockCmdContext(ctrl),
		stdout: &bytes.Buffer{},
	}
	deps.cmd.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

	err := runList(deps.cmd, &listOptions{
		scopeArg:     "org/Fabrikam",
		changedAfter: "not-a-date",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--changed-after")
}

func TestBuildTagPredicate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		tags []string
		want string
	}{
		{name: "empty returns empty", tags: nil, want: ""},
		{name: "single tag", tags: []string{"web"}, want: "[System.Tags] CONTAINS 'web'"},
		{name: "multiple tags AND", tags: []string{"web", "security"}, want: "[System.Tags] CONTAINS 'web' AND [System.Tags] CONTAINS 'security'"},
		{name: "trims whitespace", tags: []string{" web ", "  "}, want: "[System.Tags] CONTAINS 'web'"},
		{name: "empty in middle skips", tags: []string{"web", "  ", "sec"}, want: "[System.Tags] CONTAINS 'web' AND [System.Tags] CONTAINS 'sec'"},
		{name: "dedupes case-insensitively", tags: []string{"Web", "web"}, want: "[System.Tags] CONTAINS 'Web'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := buildTagPredicate(tc.tags)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestValidateTags(t *testing.T) {
	t.Parallel()
	assert.NoError(t, validateTags("--tag", nil))
	assert.NoError(t, validateTags("--tag", []string{"web"}))
	assert.Error(t, validateTags("--tag", []string{" "}))
}

func TestRunList_TagFilter(t *testing.T) {
	t.Parallel()
	deps := setupFakeDeps(t, "org")
	stubDefaultOpenTypes(deps)
	stubBatch(t, deps, false)

	var captured string
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			ids := []int{1}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &[]workitemtracking.WorkItemReference{{Id: &ids[0]}}}, nil
		},
	)

	err := runList(deps.cmd, &listOptions{
		scopeArg: "org/Fabrikam",
		tags:     []string{"web", "security"},
	})
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.Tags] CONTAINS 'web' AND [System.Tags] CONTAINS 'security'")
}

func TestRunList_CreatedByMe(t *testing.T) {
	t.Parallel()
	deps := setupFakeDeps(t, "org")
	stubDefaultOpenTypes(deps)
	stubBatch(t, deps, false)

	selfID := uuid.New()
	deps.clientFact.EXPECT().Extensions(gomock.Any(), "org").Return(deps.ext, nil)
	deps.ext.EXPECT().GetSelfID(gomock.Any()).Return(selfID, nil)
	deps.clientFact.EXPECT().Identity(gomock.Any(), "org").Return(deps.ident, nil)
	deps.ident.EXPECT().ReadIdentities(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			require.NotNil(t, args.IdentityIds)
			assert.Equal(t, selfID.String(), *args.IdentityIds)
			id := identity.Identity{
				Properties: map[string]any{
					"Account": map[string]any{"$value": "Alice <alice@x.com>"},
				},
			}
			return &[]identity.Identity{id}, nil
		},
	)

	var captured string
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			ids := []int{1}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &[]workitemtracking.WorkItemReference{{Id: &ids[0]}}}, nil
		},
	)

	err := runList(deps.cmd, &listOptions{
		scopeArg:  "org/Fabrikam",
		createdBy: []string{"@me"},
	})
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.CreatedBy] IN ('Alice <alice@x.com>')")
}

func TestRunList_AuthoredByAlias(t *testing.T) {
	t.Parallel()
	deps := setupFakeDeps(t, "org")
	stubDefaultOpenTypes(deps)
	stubBatch(t, deps, false)

	var captured string
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			ids := []int{1}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &[]workitemtracking.WorkItemReference{{Id: &ids[0]}}}, nil
		},
	)

	err := runList(deps.cmd, &listOptions{
		scopeArg:  "org/Fabrikam",
		createdBy: []string{"bob@x.com"},
	})
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.CreatedBy] IN ('bob@x.com')")
}

func TestValidateState(t *testing.T) {
	t.Parallel()
	assert.NoError(t, validateState(nil))
	assert.NoError(t, validateState([]string{"Active"}))
	assert.NoError(t, validateState([]string{"  Active  ", "Resolved"}))
	assert.Error(t, validateState([]string{"  "}))
	assert.Error(t, validateState([]string{"Active", "  "}))
}

func TestBuildStateClause(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		states  []string
		want    string
		wantErr string
	}{
		{name: "empty", states: nil, want: ""},
		{name: "single", states: []string{"Active"}, want: "[System.State] IN ('Active')"},
		{name: "multiple", states: []string{"Active", "Ready for Review"}, want: "[System.State] IN ('Active', 'Ready for Review')"},
		{name: "trims", states: []string{"  Active  "}, want: "[System.State] IN ('Active')"},
		{name: "empty in middle errors", states: []string{"Active", "  "}, wantErr: "--state value cannot be empty"},
		{name: "dedupes case-insensitively", states: []string{"Active", "ACTIVE"}, want: "[System.State] IN ('Active')"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildStateClause(tc.states)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRunList_StateExactOnly(t *testing.T) {
	t.Parallel()
	deps := setupFakeDeps(t, "org")
	// no stubDefaultOpenTypes — we don't want state resolution.
	stubBatch(t, deps, false)

	var captured string
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			ids := []int{1}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &[]workitemtracking.WorkItemReference{{Id: &ids[0]}}}, nil
		},
	)

	err := runList(deps.cmd, &listOptions{
		scopeArg: "org/Fabrikam",
		status:   []string{"all"},
		state:    []string{"Active"},
	})
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.State] IN ('Active')")
	// With status=all, the category predicate is empty, so no "(...) AND" wrapping.
	assert.NotContains(t, captured, ") AND")
}

func TestRunList_StatusAndStateIntersect(t *testing.T) {
	t.Parallel()
	deps := setupFakeDeps(t, "org")
	stubDefaultOpenTypes(deps) // needed because --status=open triggers state resolution
	stubBatch(t, deps, false)

	var captured string
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			ids := []int{1}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &[]workitemtracking.WorkItemReference{{Id: &ids[0]}}}, nil
		},
	)

	err := runList(deps.cmd, &listOptions{
		scopeArg: "org/Fabrikam",
		state:    []string{"Active"},
	})
	require.NoError(t, err)
	// We expect the category predicate (e.g. from "New","Active","Proposed","InProgress")
	// ANDed with the state predicate, both inside the state segment.
	// The exact form: ( [System.State] IN ('New','Active','Proposed','InProgress') ) AND ( [System.State] IN ('Active') )
	assert.Contains(t, captured, ") AND ([System.State] IN ('Active')")
}

func TestBuildWiqlQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		project        string
		stateCategory  string
		types          []string
		assignedTo     []string
		severity       []string
		priority       []int
		area           []string
		iteration      []string
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:           "project only",
			project:        "Fabrikam",
			mustContain:    []string{"[System.TeamProject] = 'Fabrikam'", "SELECT [System.Id] FROM WorkItems", "ORDER BY [System.ChangedDate] DESC"},
			mustNotContain: []string{"[System.State]", "[System.WorkItemType]", "[System.AssignedTo]", "[System.AreaPath]", "[System.IterationPath]"},
		},
		{
			name:          "all flags combined",
			project:       "Fabrikam",
			stateCategory: "[System.State] IN ('Active','New')",
			types:         []string{"User Story", "Task"},
			assignedTo:    []string{"alice@x.com", "Bob"},
			severity:      []string{"1 - Critical"},
			priority:      []int{1, 2},
			area:          []string{"Web/Payments", "Under:Web/Payments/Internal"},
			iteration:     []string{"Under:Release 2025/Sprint 1"},
			mustContain: []string{
				"[System.TeamProject] = 'Fabrikam'",
				"[System.State] IN ('Active','New')",
				"[System.WorkItemType] IN ('User Story', 'Task')",
				"[System.AssignedTo] IN ('alice@x.com', 'Bob')",
				"[Microsoft.VSTS.Common.Severity] IN ('1 - Critical')",
				"[Microsoft.VSTS.Common.Priority] IN (1, 2)",
				"[System.AreaPath] = 'Web/Payments'",
				"[System.AreaPath] UNDER 'Web/Payments/Internal'",
				"[System.IterationPath] UNDER 'Release 2025/Sprint 1'",
			},
		},
		{
			name:       "type and assignedTo list ordering",
			project:    "P",
			types:      []string{"Bug"},
			assignedTo: []string{"a@b.com"},
			mustContain: []string{
				"[System.WorkItemType] IN ('Bug')",
				"[System.AssignedTo] IN ('a@b.com')",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := buildWiqlQuery(tc.project, tc.stateCategory, tc.types, tc.assignedTo, tc.severity, tc.priority, tc.area, tc.iteration, "", "", "", "", nil)
			for _, want := range tc.mustContain {
				assert.Contains(t, got, want)
			}
			for _, notWant := range tc.mustNotContain {
				assert.NotContains(t, got, notWant)
			}
			assert.Equal(t, 1, strings.Count(got, "SELECT [System.Id] FROM WorkItems WHERE"))
		})
	}
}

func TestBuildUnderOrEqualsPredicate(t *testing.T) {
	t.Parallel()

	t.Run("single equals path", func(t *testing.T) {
		t.Parallel()
		got := buildUnderOrEqualsPredicate("[System.AreaPath]", []string{"Web/Payments"})
		assert.Equal(t, "[System.AreaPath] = 'Web/Payments'", got)
	})

	t.Run("single Under path", func(t *testing.T) {
		t.Parallel()
		got := buildUnderOrEqualsPredicate("[System.IterationPath]", []string{"Under:Release 2025/Sprint 1"})
		assert.Equal(t, "[System.IterationPath] UNDER 'Release 2025/Sprint 1'", got)
	})

	t.Run("multiple values get OR with parentheses", func(t *testing.T) {
		t.Parallel()
		got := buildUnderOrEqualsPredicate("[System.AreaPath]", []string{"Web/Payments", "Under:Mobile"})
		assert.Equal(t, "([System.AreaPath] = 'Web/Payments' OR [System.AreaPath] UNDER 'Mobile')", got)
	})

	t.Run("empty input returns empty string", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, buildUnderOrEqualsPredicate("[System.AreaPath]", nil))
		assert.Empty(t, buildUnderOrEqualsPredicate("[System.AreaPath]", []string{"", "  "}))
	})
}

func TestWiqlQuote(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "'foo'", wiqlQuote("foo"))
	assert.Equal(t, "'O''Brien'", wiqlQuote("O'Brien"))
}

func TestWiqlQuoteListAndIntList(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "'a', 'b'", wiqlQuoteList([]string{"a", "b", "  "}))
	assert.Equal(t, "1, 2, 3", wiqlIntList([]int{1, 2, 2, 3}))
}

func TestValidateClassification(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateClassification([]string{"1 - Critical", "3 - Medium"}))
	err := validateClassification([]string{"5 - Disaster"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value for --classification")
}

func TestValidatePriority(t *testing.T) {
	t.Parallel()

	require.NoError(t, validatePriority([]int{1, 2, 3, 4}))
	require.Error(t, validatePriority([]int{0}))
	require.Error(t, validatePriority([]int{5}))
}

func TestValidateUnderPaths(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateUnderPaths("--area", []string{"Web/Payments", "Under:Mobile/Auth"}))
	require.Error(t, validateUnderPaths("--area", []string{"Under:"}))
	require.Error(t, validateUnderPaths("--area", []string{"Under:   "}))
}

func TestNormalizeStatuses(t *testing.T) {
	t.Parallel()

	t.Run("nil defaults to open", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []string{"open"}, normalizeStatuses(nil))
	})

	t.Run("trims, lowercases, dedupes, falls back to open", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []string{"open", "closed"}, normalizeStatuses([]string{"  OPEN ", "closed", "CLOSED"}))
		assert.Equal(t, []string{"open"}, normalizeStatuses([]string{"", "   "}))
	})
}

func TestCanonCategory(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "inprogress", canonCategory("In Progress"))
	assert.Equal(t, "completed", canonCategory(" Completed "))
}

func TestShouldResolveIdentity(t *testing.T) {
	t.Parallel()

	assert.False(t, shouldResolveIdentity("alice@example.com"))
	assert.False(t, shouldResolveIdentity("Alice Smith"))
	assert.True(t, shouldResolveIdentity("vssgp.Uy0xLjI"))
	assert.True(t, shouldResolveIdentity("org/team"))
}

func TestExtractWorkItemIDs(t *testing.T) {
	t.Parallel()

	t.Run("nil result", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, extractWorkItemIDs(nil))
	})

	t.Run("skips nil IDs", func(t *testing.T) {
		t.Parallel()
		result := &workitemtracking.WorkItemQueryResult{
			WorkItems: &[]workitemtracking.WorkItemReference{
				{Id: types.ToPtr(1)},
				{Id: nil},
				{Id: types.ToPtr(2)},
			},
		}
		assert.Equal(t, []int{1, 2}, extractWorkItemIDs(result))
	})
}

func TestOrderWorkItemsByIDs(t *testing.T) {
	t.Parallel()

	items := []workitemtracking.WorkItem{
		{Id: types.ToPtr(1), Url: types.ToPtr("a")},
		{Id: types.ToPtr(2), Url: types.ToPtr("b")},
		{Id: types.ToPtr(3), Url: types.ToPtr("c")},
	}
	ordered := orderWorkItemsByIDs(items, []int{3, 1, 2})
	require.Len(t, ordered, 3)
	assert.Equal(t, 3, *ordered[0].Id)
	assert.Equal(t, 1, *ordered[1].Id)
	assert.Equal(t, 2, *ordered[2].Id)
}

func TestFieldString(t *testing.T) {
	t.Parallel()

	fields := map[string]any{
		"a": "hello",
		"b": 42,
	}
	assert.Equal(t, "hello", fieldString(fields, "a"))
	assert.Equal(t, "42", fieldString(fields, "b"))
	assert.Equal(t, "", fieldString(fields, "missing"))
	assert.Equal(t, "", fieldString(nil, "a"))
}

func TestFieldIdentityDisplay(t *testing.T) {
	t.Parallel()

	fields := map[string]any{
		"a": "Alice",
		"b": map[string]any{"displayName": "Bob", "uniqueName": "bob@x.com"},
		"c": map[string]any{"uniqueName": "carol@x.com"},
	}
	assert.Equal(t, "Alice", fieldIdentityDisplay(fields, "a"))
	assert.Equal(t, "Bob", fieldIdentityDisplay(fields, "b"))
	assert.Equal(t, "carol@x.com", fieldIdentityDisplay(fields, "c"))
	assert.Equal(t, "", fieldIdentityDisplay(fields, "missing"))
	assert.Equal(t, "", fieldIdentityDisplay(nil, "a"))
}

func TestIdentityAccountOrDisplay(t *testing.T) {
	t.Parallel()

	t.Run("Account property wins", func(t *testing.T) {
		t.Parallel()
		ident := identity.Identity{
			Properties: map[string]any{
				"Account": map[string]any{"$value": "Account.From.Properties"},
			},
			ProviderDisplayName: types.ToPtr("Display Name"),
		}
		assert.Equal(t, "Account.From.Properties", identityAccountOrDisplay(ident))
	})

	t.Run("falls back to ProviderDisplayName", func(t *testing.T) {
		t.Parallel()
		ident := identity.Identity{ProviderDisplayName: types.ToPtr("Display Name")}
		assert.Equal(t, "Display Name", identityAccountOrDisplay(ident))
	})

	t.Run("returns empty when nothing available", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", identityAccountOrDisplay(identity.Identity{}))
	})
}

func TestValidateListOptions(t *testing.T) {
	t.Parallel()

	t.Run("nil options", func(t *testing.T) {
		t.Parallel()
		require.Error(t, validateListOptions(nil))
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateListOptions(&listOptions{
			classification: []string{"1 - Critical"},
			priority:       []int{1},
			area:           []string{"Web/Payments"},
			iteration:      []string{"Under:Release 1/Sprint 1"},
		}))
	})

	t.Run("invalid priority", func(t *testing.T) {
		t.Parallel()
		err := validateListOptions(&listOptions{priority: []int{0}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--priority")
	})
}

func TestTrimStrings(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"A", "B"}, trimStrings([]string{"  A  ", "B", "", "  ", "a"}))
}

func TestAppendStateNamesByCategory(t *testing.T) {
	t.Parallel()

	states := []workitemtracking.WorkItemStateColor{
		{Name: types.ToPtr("Active"), Category: types.ToPtr("InProgress")},
		{Name: types.ToPtr("Closed"), Category: types.ToPtr("Completed")},
		{Name: types.ToPtr(""), Category: types.ToPtr("Proposed")},
		{Name: types.ToPtr("New"), Category: types.ToPtr("")},
	}

	out := []string{}
	categories := map[string]struct{}{"inprogress": {}, "completed": {}}
	appendStateNamesByCategory(&out, &states, categories)
	assert.ElementsMatch(t, []string{"Active", "Closed"}, out)

	appendStateNamesByCategory(nil, &states, categories)
	appendStateNamesByCategory(&out, nil, categories)
	empty := []workitemtracking.WorkItemStateColor{}
	appendStateNamesByCategory(&out, &empty, categories)
}

func TestNewCmd_FlagShortcuts(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ctx := mocks.NewMockCmdContext(ctrl)
	cmd := NewCmd(ctx)

	type flagCheck struct {
		name      string
		shorthand string
	}
	for _, want := range []flagCheck{
		// --type uses -T because --template (registered by util.AddJSONFlags)
		// already owns -t across all azdo commands.
		{"changed-after", ""},
		{"created-after", ""},
		{"created-by", ""},
		{"authored-by", ""},
		{"sort", ""},
		{"state", ""},
		{"tag", ""},
		{"status", "s"},
		{"type", "T"},
		{"assigned-to", "a"},
		{"classification", "c"},
		{"priority", "p"},
		{"limit", "L"},
	} {
		f := cmd.Flags().Lookup(want.name)
		require.NotNil(t, f, "flag %q must be registered", want.name)
		assert.Equal(t, want.shorthand, f.Shorthand, "flag --%s shorthand mismatch", want.name)
	}

	assert.Equal(t, "list [ORGANIZATION/]PROJECT", cmd.Use)
	assert.ElementsMatch(t, []string{"ls", "l"}, cmd.Aliases)
}

// ----- runList integration tests via gomock -----

type fakeListDeps struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	wit        *mocks.MockWorkItemTrackingClient
	ext        *mocks.MockAzDOExtension
	ident      *mocks.MockIdentityClient
	stdout     *bytes.Buffer
}

func setupFakeDeps(t *testing.T, organization string) *fakeListDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeListDeps{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		ext:        mocks.NewMockAzDOExtension(ctrl),
		ident:      mocks.NewMockIdentityClient(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(deps.wit, nil).AnyTimes()

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	return deps
}

func openStateColors() []workitemtracking.WorkItemStateColor {
	return []workitemtracking.WorkItemStateColor{
		{Name: types.ToPtr("New"), Category: types.ToPtr("Proposed")},
		{Name: types.ToPtr("Active"), Category: types.ToPtr("InProgress")},
		{Name: types.ToPtr("Resolved"), Category: types.ToPtr("Resolved")},
		{Name: types.ToPtr("Closed"), Category: types.ToPtr("Completed")},
		{Name: types.ToPtr("Removed"), Category: types.ToPtr("Removed")},
	}
}

func workItemTypesWithStates() []workitemtracking.WorkItemType {
	states := openStateColors()
	disabled := false
	return []workitemtracking.WorkItemType{
		{
			Name:       types.ToPtr("User Story"),
			IsDisabled: &disabled,
			States:     &states,
		},
	}
}

func sampleWorkItem(id int) workitemtracking.WorkItem {
	fields := map[string]any{
		"System.WorkItemType":  "User Story",
		"System.State":         "Active",
		"System.Title":         fmt.Sprintf("Item %d", id),
		"System.AssignedTo":    "Alice <alice@x.com>",
		"System.AreaPath":      "Fabrikam\\Web",
		"System.IterationPath": "Fabrikam\\Release 1\\Sprint 1",
	}
	return workitemtracking.WorkItem{Id: types.ToPtr(id), Fields: &fields}
}

func stubDefaultOpenTypes(deps *fakeListDeps) {
	typesList := workItemTypesWithStates()
	deps.wit.EXPECT().GetWorkItemTypes(gomock.Any(), gomock.Any()).Return(&typesList, nil)
}

func stubBatch(t *testing.T, deps *fakeListDeps, expandAll bool) {
	t.Helper()

	deps.wit.EXPECT().GetWorkItemsBatch(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemsBatchArgs) (*[]workitemtracking.WorkItem, error) {
			require.NotNil(t, args.WorkItemGetRequest)
			require.NotNil(t, args.WorkItemGetRequest.Ids)
			if expandAll {
				require.NotNil(t, args.WorkItemGetRequest.Expand)
				assert.Equal(t, workitemtracking.WorkItemExpandValues.All, *args.WorkItemGetRequest.Expand)
				assert.Nil(t, args.WorkItemGetRequest.Fields)
			} else {
				require.NotNil(t, args.WorkItemGetRequest.Fields)
				assert.Nil(t, args.WorkItemGetRequest.Expand)
			}
			require.NotNil(t, args.WorkItemGetRequest.ErrorPolicy)
			assert.Equal(t, workitemtracking.WorkItemErrorPolicyValues.Omit, *args.WorkItemGetRequest.ErrorPolicy)

			batch := *args.WorkItemGetRequest.Ids
			out := make([]workitemtracking.WorkItem, 0, len(batch))
			for _, id := range batch {
				out = append(out, sampleWorkItem(id))
			}
			return &out, nil
		},
	).AnyTimes()
}

func TestRunList_DefaultOpenStatus(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	stubDefaultOpenTypes(deps)
	stubBatch(t, deps, false)

	captured := ""
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			refs := []workitemtracking.WorkItemReference{{Id: types.ToPtr(1)}, {Id: types.ToPtr(2)}}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
		},
	)

	err := runList(deps.cmd, &listOptions{scopeArg: "org/Fabrikam"})
	require.NoError(t, err)

	assert.Contains(t, captured, "[System.TeamProject] = 'Fabrikam'")
	assert.Contains(t, captured, "[System.State] IN (")
	assert.Contains(t, deps.stdout.String(), "Item 1")
}

func TestRunList_StatusAllOmitsStatePredicate(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	captured := ""

	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			refs := []workitemtracking.WorkItemReference{{Id: types.ToPtr(7)}}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
		},
	)
	stubBatch(t, deps, false)

	err := runList(deps.cmd, &listOptions{scopeArg: "org/Fabrikam", status: []string{"all"}})
	require.NoError(t, err)

	assert.NotContains(t, captured, "[System.State]")
}

func TestRunList_LimitWiresTop(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	var top *int

	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			top = args.Top
			refs := []workitemtracking.WorkItemReference{{Id: types.ToPtr(1)}}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
		},
	)
	stubBatch(t, deps, false)

	err := runList(deps.cmd, &listOptions{scopeArg: "org/Fabrikam", status: []string{"all"}, limit: 25})
	require.NoError(t, err)
	require.NotNil(t, top)
	assert.Equal(t, 25, *top)
}

func TestRunList_BatchChunkingAt200(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")

	ids := make([]int, 0, 250)
	for i := 1; i <= 250; i++ {
		ids = append(ids, i)
	}

	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			refs := make([]workitemtracking.WorkItemReference, 0, len(ids))
			for _, id := range ids {
				refs = append(refs, workitemtracking.WorkItemReference{Id: types.ToPtr(id)})
			}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
		},
	)

	batchSizes := []int{}
	deps.wit.EXPECT().GetWorkItemsBatch(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemsBatchArgs) (*[]workitemtracking.WorkItem, error) {
			batchSizes = append(batchSizes, len(*args.WorkItemGetRequest.Ids))
			batch := *args.WorkItemGetRequest.Ids
			out := make([]workitemtracking.WorkItem, 0, len(batch))
			for _, id := range batch {
				out = append(out, sampleWorkItem(id))
			}
			return &out, nil
		},
	).Times(2)

	err := runList(deps.cmd, &listOptions{scopeArg: "org/Fabrikam", status: []string{"all"}})
	require.NoError(t, err)
	assert.Equal(t, []int{200, 50}, batchSizes)
}

func TestRunList_AssignedToMeResolvesIdentity(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")

	selfID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	deps.clientFact.EXPECT().Extensions(gomock.Any(), "org").Return(deps.ext, nil)
	deps.clientFact.EXPECT().Identity(gomock.Any(), "org").Return(deps.ident, nil)
	deps.ext.EXPECT().GetSelfID(gomock.Any()).Return(selfID, nil)

	idsArg := selfID.String()
	deps.ident.EXPECT().ReadIdentities(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			require.NotNil(t, args.IdentityIds)
			assert.Equal(t, idsArg, *args.IdentityIds)
			account := "Account.From.Properties"
			display := "Self User"
			out := []identity.Identity{{
				Properties:          map[string]any{"Account": map[string]any{"$value": account}},
				ProviderDisplayName: &display,
			}}
			return &out, nil
		},
	)

	captured := ""
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			refs := []workitemtracking.WorkItemReference{{Id: types.ToPtr(99)}}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
		},
	)
	stubBatch(t, deps, false)

	err := runList(deps.cmd, &listOptions{scopeArg: "org/Fabrikam", status: []string{"all"}, assignedTo: []string{"@me"}})
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.AssignedTo] IN ('Account.From.Properties')")
}

func TestRunList_AssignedToEmailSkipsLookup(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	captured := ""

	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			refs := []workitemtracking.WorkItemReference{{Id: types.ToPtr(5)}}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
		},
	)
	stubBatch(t, deps, false)

	err := runList(deps.cmd, &listOptions{scopeArg: "org/Fabrikam", status: []string{"all"}, assignedTo: []string{"alice@x.com"}})
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.AssignedTo] IN ('alice@x.com')")
}

func TestRunList_AreaUnderPrefix(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	captured := ""

	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
			captured = *args.Wiql.Query
			refs := []workitemtracking.WorkItemReference{{Id: types.ToPtr(1)}}
			return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
		},
	)
	stubBatch(t, deps, false)

	err := runList(deps.cmd, &listOptions{
		scopeArg: "org/Fabrikam",
		status:   []string{"all"},
		area:     []string{"Under:Web/Payments"},
	})
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.AreaPath] UNDER 'Web/Payments'")
}

func TestRunList_NoResultsReturnsNoResultsError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).Return(&workitemtracking.WorkItemQueryResult{
		WorkItems: &[]workitemtracking.WorkItemReference{},
	}, nil)

	err := runList(deps.cmd, &listOptions{scopeArg: "org/Fabrikam", status: []string{"all"}})
	require.Error(t, err)
	var noResults util.NoResultsError
	require.True(t, errors.As(err, &noResults), "expected NoResultsError, got %v", err)
}

func TestRunList_WiqlErrorPropagates(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runList(deps.cmd, &listOptions{scopeArg: "org/Fabrikam", status: []string{"all"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WIQL")
}

func TestRunList_JSONOutputUsesExpandAll(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")

	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).Return(&workitemtracking.WorkItemQueryResult{
		WorkItems: &[]workitemtracking.WorkItemReference{{Id: types.ToPtr(42)}},
	}, nil)

	expandSeen := false
	deps.wit.EXPECT().GetWorkItemsBatch(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemsBatchArgs) (*[]workitemtracking.WorkItem, error) {
			if args.WorkItemGetRequest.Expand != nil {
				expandSeen = true
				assert.Equal(t, workitemtracking.WorkItemExpandValues.All, *args.WorkItemGetRequest.Expand)
			}
			batch := *args.WorkItemGetRequest.Ids
			out := make([]workitemtracking.WorkItem, 0, len(batch))
			for _, id := range batch {
				out = append(out, sampleWorkItem(id))
			}
			return &out, nil
		},
	)

	opts := &listOptions{
		scopeArg: "org/Fabrikam",
		status:   []string{"all"},
		exporter: &stubExporter{},
	}

	err := runList(deps.cmd, opts)
	require.NoError(t, err)
	assert.True(t, expandSeen, "JSON path must request expand=All")
}

type stubExporter struct{}

func (s *stubExporter) Fields() []string { return nil }
func (s *stubExporter) Write(ios *iostreams.IOStreams, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = ios.Out.Write(payload)
	return err
}

func TestRunList_ValidationErrorBubbles(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ctx := mocks.NewMockCmdContext(ctrl)
	ios, _, _, _ := iostreams.Test()
	ctx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

	err := runList(ctx, &listOptions{scopeArg: "org/Fabrikam", classification: []string{"5 - Disaster"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value for --classification")
}
