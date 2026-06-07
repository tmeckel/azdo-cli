package add

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

type fakeDeps struct {
	cmd         *mocks.MockCmdContext
	clientFact  *mocks.MockClientFactory
	coreClient  *mocks.MockCoreClient
	graphClient *mocks.MockGraphClient
	extClient   *mocks.MockAzDOExtension
	prompter    *mocks.MockPrompter
	config      *mocks.MockConfig
	authCfg     *mocks.MockAuthConfig
}

func setupFakeDeps(t *testing.T, organization string) (*fakeDeps, *bytes.Buffer) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ios, _, out, _ := iostreams.Test()
	ios.SetStdoutTTY(false)
	ios.SetStderrTTY(false)

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)

	deps := &fakeDeps{
		cmd:         mocks.NewMockCmdContext(ctrl),
		clientFact:  mocks.NewMockClientFactory(ctrl),
		coreClient:  mocks.NewMockCoreClient(ctrl),
		graphClient: mocks.NewMockGraphClient(ctrl),
		extClient:   mocks.NewMockAzDOExtension(ctrl),
		prompter:    mocks.NewMockPrompter(ctrl),
		config:      mocks.NewMockConfig(ctrl),
		authCfg:     mocks.NewMockAuthConfig(ctrl),
	}

	deps.cmd.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Printer(gomock.Any()).Return(tp, nil).AnyTimes()
	deps.clientFact.EXPECT().Core(gomock.Any(), organization).Return(deps.coreClient, nil).AnyTimes()
	deps.clientFact.EXPECT().Graph(gomock.Any(), organization).Return(deps.graphClient, nil).AnyTimes()
	deps.clientFact.EXPECT().Extensions(gomock.Any(), organization).Return(deps.extClient, nil).AnyTimes()

	return deps, out
}

func teamResult(desc, name string) *core.WebApiTeam {
	return &core.WebApiTeam{
		Name: &name,
		Identity: &identity.Identity{
			SubjectDescriptor: &desc,
		},
	}
}

func subject(desc, displayName, origin, legacyDesc string) *graph.GraphSubject {
	return &graph.GraphSubject{
		Descriptor:       &desc,
		DisplayName:      &displayName,
		Origin:           &origin,
		LegacyDescriptor: &legacyDesc,
	}
}

func notFound() error {
	return &azuredevops.WrappedError{
		StatusCode: types.ToPtr(http.StatusNotFound),
	}
}

func conflict() error {
	return &azuredevops.WrappedError{
		StatusCode: types.ToPtr(http.StatusConflict),
	}
}

func apiError(code int) error {
	return &azuredevops.WrappedError{
		StatusCode: types.ToPtr(code),
	}
}

// --- Single-member tests ---

func TestAdd_SingleMember_HappyPath(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	teamDesc := "vssgp.Uy0xLTkt"
	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult(teamDesc, "My Team"), nil)

	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, members []string) (map[string]*graph.GraphSubject, error) {
			assert.Equal(t, []string{"user@example.com"}, members)
			return map[string]*graph.GraphSubject{
				"user@example.com": subject("aad.YR5kM", "Alice", "aad", "legacy-aad"),
			}, nil
		})

	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound())
	deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
		Return(&graph.GraphMembership{}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--user", "user@example.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "aad.YR5kM")
	assert.Contains(t, output, "added")
	// No "Group" word in output
	assert.NotContains(t, output, "Group")
}

func TestAdd_SingleMember_AlreadyMember(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{
			"user@example.com": subject("aad.Y", "Alice", "aad", "l-aad"),
		}, nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--user", "user@example.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "already member")
}

func TestAdd_SingleMember_Race_409OnAdd(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{
			"user@example.com": subject("aad.Y", "Alice", "aad", "l-aad"),
		}, nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound())
	deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
		Return(nil, conflict())

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--user", "user@example.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "already member")
}

func TestAdd_TeamNotFound(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("team not found"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/NoTeam", "--user", "user@example.com"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team not found")
}

func TestAdd_TeamHasNoIdentity(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(&core.WebApiTeam{Name: types.ToPtr("NoIdentity")}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--user", "user@example.com"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no underlying descriptor")
	assert.NotContains(t, err.Error(), "Group")
}

func TestAdd_MemberNotFound(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	// Empty map signals the member could not be resolved (per-input absence, no catastrophic error).
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--user", "ghost@x.com"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve")
	assert.Contains(t, err.Error(), "ghost@x.com")
}

func TestAdd_MemberResolutionError(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	// Catastrophic error: add command must bubble it up.
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("network down"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--user", "ghost@x.com"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve")
}

func TestAdd_TargetArg_ParsesOrgSlashProjectSlashTeam(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myCustomOrg")

	var captured core.GetTeamArgs
	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamArgs) (*core.WebApiTeam, error) {
			captured = args
			return teamResult("vssgp.X", "Team"), nil
		})
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{
			"u@x.com": subject("aad.Y", "U", "aad", "l"),
		}, nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound())
	deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
		Return(&graph.GraphMembership{}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myCustomOrg/MyProject/MyTeam", "--user", "u@x.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, "MyProject", *captured.ProjectId)
	assert.Equal(t, "MyTeam", *captured.TeamId)
}

func TestAdd_JSONOutput_EnvelopeShape(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.Uy0xLTkt", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{
			"user@example.com": subject("aad.YR5kM", "Alice", "aad", "legacy-aad"),
		}, nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound())
	deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
		Return(&graph.GraphMembership{}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--user", "user@example.com", "--json"})
	err := cmd.Execute()
	require.NoError(t, err)

	var parsed struct {
		TeamName string            `json:"teamName"`
		Results  []json.RawMessage `json:"results"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "My Team", parsed.TeamName)
	assert.Len(t, parsed.Results, 1)
}

func TestAdd_JSONFieldFilter(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{
			"user@example.com": subject("aad.Y", "Alice", "aad", "l"),
		}, nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound())
	deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
		Return(&graph.GraphMembership{}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--user", "user@example.com", "--json=teamName"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "teamName")
	assert.Contains(t, output, "My Team")
	assert.NotContains(t, output, "results")
	assert.NotContains(t, output, "memberDescriptor")
}

func TestAdd_MissingUserFlag(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

// --- Bulk tests ---

func TestAdd_Bulk_MultipleMembersAdded(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)

	// Single batched call covering all three members
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, members []string) (map[string]*graph.GraphSubject, error) {
			assert.ElementsMatch(t, []string{"alice@c.com", "bob@c.com", "charlie@c.com"}, members)
			return map[string]*graph.GraphSubject{
				"alice@c.com":   subject("aad.1", "Alice", "aad", "la"),
				"bob@c.com":     subject("aad.2", "Bob", "aad", "lb"),
				"charlie@c.com": subject("aad.3", "Charlie", "aad", "lc"),
			}, nil
		})

	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound()).Times(3)
	deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
		Return(&graph.GraphMembership{}, nil).Times(3)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-u", "bob@c.com", "-u", "charlie@c.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "Bob")
	assert.Contains(t, output, "Charlie")

	// Input order preserved
	alicePos := bytes.Index(buf.Bytes(), []byte("Alice"))
	bobPos := bytes.Index(buf.Bytes(), []byte("Bob"))
	charliePos := bytes.Index(buf.Bytes(), []byte("Charlie"))
	assert.True(t, alicePos < bobPos && bobPos < charliePos, "input order not preserved")
}

func TestAdd_Bulk_MixedResults(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)

	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{
			"alice@c.com": subject("aad.1", "Alice", "aad", "la"),
			"bob@c.com":   subject("aad.2", "Bob", "aad", "lb"),
			"error@c.com": subject("aad.3", "Error", "aad", "le"),
		}, nil)

	gomock.InOrder(
		deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
			Return(notFound()),
		deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
			Return(&graph.GraphMembership{}, nil),
		deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
			Return(nil),
		deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
			Return(notFound()),
		deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
			Return(nil, apiError(http.StatusInternalServerError)),
	)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-u", "bob@c.com", "-u", "error@c.com"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to add member")
}

func TestAdd_Bulk_DedupePreservesInputOrder(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)

	// ResolveSubjects receives only the deduplicated inputs (alice once, bob once)
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, members []string) (map[string]*graph.GraphSubject, error) {
			assert.ElementsMatch(t, []string{"alice@c.com", "bob@c.com", "alice@c.com"}, members)
			return map[string]*graph.GraphSubject{
				"alice@c.com": subject("aad.1", "Alice", "aad", "la"),
				"bob@c.com":   subject("aad.2", "Bob", "aad", "lb"),
			}, nil
		})

	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound()).Times(2)
	deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
		Return(&graph.GraphMembership{}, nil).Times(2)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-u", "bob@c.com", "-u", "alice@c.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	// Only Alice and Bob in output, in order
	aliceCount := strings.Count(buf.String(), "Alice")
	assert.Equal(t, 1, aliceCount, "Alice should appear only once")

	alicePos := bytes.Index(buf.Bytes(), []byte("Alice"))
	bobPos := bytes.Index(buf.Bytes(), []byte("Bob"))
	assert.True(t, alicePos < bobPos, "input order not preserved")
}

func TestAdd_Bulk_ExitCodePartialSuccess(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{
			"alice@c.com": subject("aad.1", "Alice", "aad", "la"),
			"error@c.com": subject("aad.2", "Error", "aad", "le"),
		}, nil)

	gomock.InOrder(
		deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
			Return(notFound()),
		deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
			Return(&graph.GraphMembership{}, nil),
		deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
			Return(notFound()),
		deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
			Return(nil, apiError(http.StatusInternalServerError)),
	)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-u", "error@c.com"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to add member")
}

func TestAdd_Bulk_JSONEnvelopeShape(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)

	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{
			"alice@c.com": subject("aad.1", "Alice", "aad", "la"),
			"bob@c.com":   subject("aad.2", "Bob", "aad", "lb"),
		}, nil)

	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound()).Times(2)
	deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
		Return(&graph.GraphMembership{}, nil).Times(2)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-u", "bob@c.com", "--json"})
	err := cmd.Execute()
	require.NoError(t, err)

	var parsed struct {
		TeamName string            `json:"teamName"`
		Results  []json.RawMessage `json:"results"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "My Team", parsed.TeamName)
	assert.Len(t, parsed.Results, 2)
}

func TestAdd_Bulk_TableHasNoGroupColumn(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubjects(gomock.Any(), gomock.Any()).
		Return(map[string]*graph.GraphSubject{
			"user@example.com": subject("aad.Y", "Alice", "aad", "la"),
		}, nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound())
	deps.graphClient.EXPECT().AddMembership(gomock.Any(), gomock.Any()).
		Return(&graph.GraphMembership{}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--user", "user@example.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// No "Group" word anywhere in output (regression guard for decision #13)
	assert.NotContains(t, output, "Group")
	// Output has data (Alice, descriptor, status)
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "aad.Y")
	assert.Contains(t, output, "added")
}
