package list

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

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

			got := buildWiqlQuery(tc.project, tc.stateCategory, tc.types, tc.assignedTo, tc.severity, tc.priority, tc.area, tc.iteration)
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
