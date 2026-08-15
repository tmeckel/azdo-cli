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

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
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

// listOpts returns run options with the Cobra default limit (50) applied.
// Direct runList calls bypass flag parsing defaults, so tests share this
// helper instead of repeating the literal limit.
func listOpts(scopeArg string) *listOptions {
	return &listOptions{scopeArg: scopeArg, limit: 50}
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

	err := runList(deps.cmd, listOpts("org:Fabrikam"))
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

	opts := listOpts("org:Fabrikam")
	opts.sort = []string{"title:asc"}
	err := runList(deps.cmd, opts)
	require.NoError(t, err)
	assert.Contains(t, captured, "ORDER BY [System.Title] ASC")
}

func TestRunList_SortInvalidField(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	deps := &fakeListDeps{
		cmd: mocks.NewMockCmdContext(ctrlFromT(t)),
	}
	deps.cmd.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

	opts := listOpts("org:Fabrikam")
	opts.sort = []string{"banana"}
	err := runList(deps.cmd, opts)
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

	opts := listOpts("org:Fabrikam")
	opts.changedAfter = "2025-01-18T00:00:00Z"
	err := runList(deps.cmd, opts)
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.ChangedDate] >= '2025-01-18T00:00:00Z'")
}

func TestRunList_InvalidDateFlag(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	deps := &fakeListDeps{
		cmd: mocks.NewMockCmdContext(ctrl),
	}
	deps.cmd.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

	opts := listOpts("org:Fabrikam")
	opts.changedAfter = "not-a-date"
	err := runList(deps.cmd, opts)
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
		{name: "dedupes exact duplicates only, preserves case", tags: []string{"Web", "web", "Web"}, want: "[System.Tags] CONTAINS 'Web' AND [System.Tags] CONTAINS 'web'"},
		{name: "Foo and foo are distinct tags", tags: []string{"Foo", "foo"}, want: "[System.Tags] CONTAINS 'Foo' AND [System.Tags] CONTAINS 'foo'"},
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

	t.Run("valid tags pass", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, shared.ValidateTags("--tag", nil))
		assert.NoError(t, shared.ValidateTags("--tag", []string{"web"}))
		assert.NoError(t, shared.ValidateTags("--tag", []string{"web", "security"}))
	})

	t.Run("empty or whitespace-only rejected", func(t *testing.T) {
		t.Parallel()
		err := shared.ValidateTags("--tag", []string{" "})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("comma separator rejected", func(t *testing.T) {
		t.Parallel()
		err := shared.ValidateTags("--tag", []string{"web,security"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "',' or ';'")
	})

	t.Run("semicolon separator rejected", func(t *testing.T) {
		t.Parallel()
		err := shared.ValidateTags("--tag", []string{"web;security"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "',' or ';'")
	})

	t.Run("over 400 characters rejected", func(t *testing.T) {
		t.Parallel()
		err := shared.ValidateTags("--tag", []string{strings.Repeat("a", 401)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "400")
		assert.Contains(t, err.Error(), "characters")
	})

	t.Run("exactly 400 characters accepted", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, shared.ValidateTags("--tag", []string{strings.Repeat("a", 400)}))
	})

	t.Run("multibyte tag of 400 characters accepted", func(t *testing.T) {
		t.Parallel()
		// 'é' is 2 bytes in UTF-8, so 400 characters occupy 800 bytes; the
		// limit counts characters, not bytes.
		assert.NoError(t, shared.ValidateTags("--tag", []string{strings.Repeat("é", 400)}))
	})

	t.Run("multibyte tag over 400 characters rejected", func(t *testing.T) {
		t.Parallel()
		err := shared.ValidateTags("--tag", []string{strings.Repeat("é", 401)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "400")
		assert.Contains(t, err.Error(), "characters")
	})

	t.Run("at sign allowed", func(t *testing.T) {
		t.Parallel()
		// Microsoft recommends avoiding '@' in tag names but does not forbid
		// it, so it must not be rejected client-side.
		assert.NoError(t, shared.ValidateTags("--tag", []string{"@web"}))
		assert.NoError(t, shared.ValidateTags("--tag", []string{"web@2025"}))
	})
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

	opts := listOpts("org:Fabrikam")
	opts.tags = []string{"web", "security"}
	err := runList(deps.cmd, opts)
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.Tags] CONTAINS 'web' AND [System.Tags] CONTAINS 'security'")
}

func TestRunList_CreatedByMe(t *testing.T) {
	t.Parallel()
	deps := setupFakeDeps(t, "org")
	stubDefaultOpenTypes(deps)
	stubBatch(t, deps, false)
	deps.clientFact.EXPECT().Extensions(gomock.Any(), "org").Return(deps.ext, nil)
	deps.ext.EXPECT().ResolveCurrentIdentity(gomock.Any()).DoAndReturn(
		func(_ context.Context) (*identity.Identity, error) {
			return &identity.Identity{
				Properties: map[string]any{
					"Account": map[string]any{"$value": "Alice <alice@x.com>"},
				},
			}, nil
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

	opts := listOpts("org:Fabrikam")
	opts.createdBy = []string{"@me"}
	err := runList(deps.cmd, opts)
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

	opts := listOpts("org:Fabrikam")
	opts.createdBy = []string{"bob@x.com"}
	err := runList(deps.cmd, opts)
	require.NoError(t, err)
	assert.Contains(t, captured, "[System.CreatedBy] IN ('bob@x.com')")
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

	opts := listOpts("org:Fabrikam")
	opts.status = []string{"all"}
	opts.state = []string{"Active"}
	err := runList(deps.cmd, opts)
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

	opts := listOpts("org:Fabrikam")
	opts.state = []string{"Active"}
	err := runList(deps.cmd, opts)
	require.NoError(t, err)
	// We expect the category predicate (e.g. from "New","Active","Proposed","InProgress")
	// ANDead with the state predicate, both inside the state segment.
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
				`[System.AreaPath] = 'Fabrikam\Web\Payments'`,
				`[System.AreaPath] UNDER 'Fabrikam\Web\Payments\Internal'`,
				`[System.IterationPath] UNDER 'Fabrikam\Release 2025\Sprint 1'`,
			},
		},
		{
			name:      "rooted paths preserved",
			project:   "Fabrikam Fiber",
			area:      []string{`Fabrikam Fiber\Area\Voice`, "Under:Fabrikam Fiber/Area/Voice/Internal"},
			iteration: []string{`fabrikam fiber\Release 2\Sprint 36`},
			mustContain: []string{
				`[System.AreaPath] = 'Fabrikam Fiber\Area\Voice'`,
				`[System.AreaPath] UNDER 'Fabrikam Fiber\Area\Voice\Internal'`,
				`[System.IterationPath] = 'fabrikam fiber\Release 2\Sprint 36'`,
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
		got := buildUnderOrEqualsPredicate("[System.AreaPath]", "Fabrikam", []string{"Web/Payments"})
		assert.Equal(t, `[System.AreaPath] = 'Fabrikam\Web\Payments'`, got)
	})

	t.Run("single Under path", func(t *testing.T) {
		t.Parallel()
		got := buildUnderOrEqualsPredicate("[System.IterationPath]", "Fabrikam", []string{"Under:Release 2025/Sprint 1"})
		assert.Equal(t, `[System.IterationPath] UNDER 'Fabrikam\Release 2025\Sprint 1'`, got)
	})

	t.Run("Under prefix is case-insensitive and path casing preserved", func(t *testing.T) {
		t.Parallel()
		for _, prefix := range []string{"Under:", "under:", "UNDER:", "uNdeR:"} { // spellchecker:disable-line
			got := buildUnderOrEqualsPredicate("[System.AreaPath]", "Fabrikam", []string{prefix + "Web/Payments"})
			assert.Equal(t, `[System.AreaPath] UNDER 'Fabrikam\Web\Payments'`, got)
		}
		got := buildUnderOrEqualsPredicate("[System.AreaPath]", "Fabrikam", []string{"under:Web/Payments/Internal"})
		assert.Equal(t, `[System.AreaPath] UNDER 'Fabrikam\Web\Payments\Internal'`, got)
	})

	t.Run("multiple values get OR with parentheses", func(t *testing.T) {
		t.Parallel()
		got := buildUnderOrEqualsPredicate("[System.AreaPath]", "Fabrikam", []string{"Web/Payments", "Under:Mobile"})
		assert.Equal(t, `([System.AreaPath] = 'Fabrikam\Web\Payments' OR [System.AreaPath] UNDER 'Fabrikam\Mobile')`, got)
	})

	t.Run("rooted paths preserved", func(t *testing.T) {
		t.Parallel()
		got := buildUnderOrEqualsPredicate("[System.AreaPath]", "Fabrikam", []string{`Fabrikam\Area\Voice`, "Under:fabrikam/Area/Voice"})
		assert.Equal(t, `([System.AreaPath] = 'Fabrikam\Area\Voice' OR [System.AreaPath] UNDER 'fabrikam\Area\Voice')`, got)
	})

	t.Run("empty input returns empty string", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, buildUnderOrEqualsPredicate("[System.AreaPath]", "Fabrikam", nil))
		assert.Empty(t, buildUnderOrEqualsPredicate("[System.AreaPath]", "Fabrikam", []string{"", "  "}))
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

func TestValidateUnderPaths(t *testing.T) {
	t.Parallel()

	require.NoError(t, shared.ValidateUnderPaths("--area", []string{"Web/Payments", "Under:Mobile/Auth"}))
	require.NoError(t, shared.ValidateUnderPaths("--area", []string{"under:Mobile/Auth", "UNDER:Web"}))
	require.Error(t, shared.ValidateUnderPaths("--area", []string{"Under:"}))
	require.Error(t, shared.ValidateUnderPaths("--area", []string{"under:"}))
	require.Error(t, shared.ValidateUnderPaths("--area", []string{"Under:   "}))
	require.Error(t, shared.ValidateUnderPaths("--area", []string{"UNDER:  "}))
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
	assert.Equal(t, "hello", shared.FieldString(fields, "a"))
	assert.Equal(t, "42", shared.FieldString(fields, "b"))
	assert.Equal(t, "", shared.FieldString(fields, "missing"))
	assert.Equal(t, "", shared.FieldString(nil, "a"))
}

func TestFieldIdentityDisplay(t *testing.T) {
	t.Parallel()

	fields := map[string]any{
		"a": "Alice",
		"b": map[string]any{"displayName": "Bob", "uniqueName": "bob@x.com"},
		"c": map[string]any{"uniqueName": "carol@x.com"},
	}
	assert.Equal(t, "Alice", shared.FieldIdentityDisplay(fields, "a"))
	assert.Equal(t, "Bob", shared.FieldIdentityDisplay(fields, "b"))
	assert.Equal(t, "carol@x.com", shared.FieldIdentityDisplay(fields, "c"))
	assert.Equal(t, "", shared.FieldIdentityDisplay(fields, "missing"))
	assert.Equal(t, "", shared.FieldIdentityDisplay(nil, "a"))
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
			limit:          50,
			classification: []string{"1 - Critical"},
			priority:       []int{1},
			area:           []string{"Web/Payments"},
			iteration:      []string{"Under:Release 1/Sprint 1"},
		}))
	})

	t.Run("malformed under path errors", func(t *testing.T) {
		t.Parallel()
		err := validateListOptions(&listOptions{limit: 50, area: []string{"Under:"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--area")
	})

	t.Run("limit zero rejected", func(t *testing.T) {
		t.Parallel()
		err := validateListOptions(&listOptions{limit: 0})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--limit")
	})

	t.Run("negative limit rejected", func(t *testing.T) {
		t.Parallel()
		err := validateListOptions(&listOptions{limit: -5})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--limit")
	})

	t.Run("positive limit accepted", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateListOptions(&listOptions{limit: 1}))
		require.NoError(t, validateListOptions(&listOptions{limit: 50}))
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

	assert.Equal(t, "list [ORG:]PROJECT", cmd.Use)
	assert.ElementsMatch(t, []string{"ls", "l"}, cmd.Aliases)
}

func TestNewCmd_TagFlag_NonCSV(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	err := cmd.ParseFlags([]string{
		"--tag", "web,security",
		"--tag", "release;v1",
	})
	require.NoError(t, err)

	tags, err := cmd.Flags().GetStringArray("tag")
	require.NoError(t, err)
	// Comma/semicolon values must survive flag parsing untouched so
	// ValidateTags can reject them; CSV splitting would silently accept them.
	assert.Equal(t, []string{"web,security", "release;v1"}, tags)
}

// ----- runList integration tests via gomock -----

type fakeListDeps struct {
	ctrl       *gomock.Controller
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
		ctrl:       ctrl,
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

func TestRunList_OrgRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		depsOrg    string
		scopeArg   string
		defaultOrg string // when set, the default organization is stubbed in config
	}{
		{name: "explicit ORG: prefix routes to that organization", depsOrg: "myorg", scopeArg: "myorg:Fabrikam"},
		{name: "bare project routes to configured default organization", depsOrg: "default-org", scopeArg: "Fabrikam", defaultOrg: "default-org"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := setupFakeDeps(t, tc.depsOrg)
			stubDefaultOpenTypes(deps)
			stubBatch(t, deps, false)

			if tc.defaultOrg != "" {
				cfg := mocks.NewMockConfig(deps.ctrl)
				auth := mocks.NewMockAuthConfig(deps.ctrl)
				deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
				cfg.EXPECT().Authentication().Return(auth).AnyTimes()
				auth.EXPECT().GetDefaultOrganization().Return(tc.defaultOrg, nil).AnyTimes()
			}

			var captured string
			deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
					captured = *args.Wiql.Query
					refs := []workitemtracking.WorkItemReference{{Id: types.ToPtr(1)}}
					return &workitemtracking.WorkItemQueryResult{WorkItems: &refs}, nil
				},
			)

			// The WorkItemTracking client expectation is bound to tc.depsOrg, so
			// the run only succeeds when the organization routed to it matches.
			err := runList(deps.cmd, listOpts(tc.scopeArg))
			require.NoError(t, err)
			assert.Contains(t, captured, "[System.TeamProject] = 'Fabrikam'")
		})
	}
}

func TestRunList_LegacyOrgSlashIsRejected(t *testing.T) {
	t.Parallel()

	// A legacy ORGANIZATION/PROJECT input carries no ORG: prefix and is
	// structurally detectable in this no-target mode, so it must be rejected
	// with ORG: guidance instead of being reinterpreted.
	deps := setupFakeDeps(t, "org")

	err := runList(deps.cmd, listOpts("org/Fabrikam"))

	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")
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

	err := runList(deps.cmd, listOpts("org:Fabrikam"))
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

	opts := listOpts("org:Fabrikam")
	opts.status = []string{"all"}
	err := runList(deps.cmd, opts)
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

	err := runList(deps.cmd, &listOptions{scopeArg: "org:Fabrikam", status: []string{"all"}, limit: 25})
	require.NoError(t, err)
	require.NotNil(t, top)
	assert.Equal(t, 25, *top)
}

func TestRunList_InvalidLimitRejectedBeforeAPICall(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		limit int
	}{
		{name: "zero", limit: 0},
		{name: "negative", limit: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ios, _, _, _ := iostreams.Test()
			deps := &fakeListDeps{
				cmd: mocks.NewMockCmdContext(ctrlFromT(t)),
			}
			deps.cmd.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

			// No QueryByWiql expectation: validation must fail before any API
			// call, so a zero/negative limit can never become an unbounded query.
			err := runList(deps.cmd, &listOptions{scopeArg: "org:Fabrikam", limit: tc.limit})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--limit")
		})
	}
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

	opts := listOpts("org:Fabrikam")
	opts.status = []string{"all"}
	err := runList(deps.cmd, opts)
	require.NoError(t, err)
	assert.Equal(t, []int{200, 50}, batchSizes)
}

func TestRunList_AssignedToMeResolvesIdentity(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.clientFact.EXPECT().Extensions(gomock.Any(), "org").Return(deps.ext, nil)
	deps.ext.EXPECT().ResolveCurrentIdentity(gomock.Any()).DoAndReturn(
		func(_ context.Context) (*identity.Identity, error) {
			account := "Account.From.Properties"
			display := "Self User"
			return &identity.Identity{
				Properties:          map[string]any{"Account": map[string]any{"$value": account}},
				ProviderDisplayName: &display,
			}, nil
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

	opts := listOpts("org:Fabrikam")
	opts.status = []string{"all"}
	opts.assignedTo = []string{"@me"}
	err := runList(deps.cmd, opts)
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

	opts := listOpts("org:Fabrikam")
	opts.status = []string{"all"}
	opts.assignedTo = []string{"alice@x.com"}
	err := runList(deps.cmd, opts)
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

	opts := listOpts("org:Fabrikam")
	opts.status = []string{"all"}
	opts.area = []string{"Under:Web/Payments"}
	err := runList(deps.cmd, opts)
	require.NoError(t, err)
	assert.Contains(t, captured, `[System.AreaPath] UNDER 'Fabrikam\Web\Payments'`)
}

func TestRunList_NoResultsReturnsNoResultsError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).Return(&workitemtracking.WorkItemQueryResult{
		WorkItems: &[]workitemtracking.WorkItemReference{},
	}, nil)

	opts := listOpts("org:Fabrikam")
	opts.status = []string{"all"}
	err := runList(deps.cmd, opts)
	require.Error(t, err)
	var noResults util.NoResultsError
	require.True(t, errors.As(err, &noResults), "expected NoResultsError, got %v", err)
}

func TestRunList_WiqlErrorPropagates(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	deps.wit.EXPECT().QueryByWiql(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	opts := listOpts("org:Fabrikam")
	opts.status = []string{"all"}
	err := runList(deps.cmd, opts)
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

	opts := listOpts("org:Fabrikam")
	opts.status = []string{"all"}
	opts.exporter = &stubExporter{}

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

	opts := listOpts("org:Fabrikam")
	opts.area = []string{"Under:"}
	err := runList(ctx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--area")
}
