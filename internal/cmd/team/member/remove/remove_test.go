package remove

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
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
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

func apiError(code int) error {
	return &azuredevops.WrappedError{
		StatusCode: types.ToPtr(code),
	}
}

// --- Single-member tests (1–11) ---

func TestRemove_SingleMember_HappyPath_YesFlag(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	teamDesc := "vssgp.Uy0xLTkt"
	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult(teamDesc, "My Team"), nil)

	deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "alice@c.com").
		Return(subject("aad.1", "Alice", "aad", "la"), nil)

	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(nil)
	deps.graphClient.EXPECT().RemoveMembership(gomock.Any(), gomock.Any()).
		Return(nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-y"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "aad.1")
	assert.Contains(t, output, "removed")
	assert.NotContains(t, output, "Group")
}

func TestRemove_SingleMember_InteractiveCancel(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "alice@c.com").
		Return(subject("aad.1", "Alice", "aad", "la"), nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(nil)
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).Times(1)
	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.True(t, errors.Is(err, util.ErrCancel), "expected ErrCancel")
}

func TestRemove_SingleMember_NotAMember(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "alice@c.com").
		Return(subject("aad.1", "Alice", "aad", "la"), nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound())

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-y"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "not a member")
}

func TestRemove_SingleMember_Race_404OnRemove(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "alice@c.com").
		Return(subject("aad.1", "Alice", "aad", "la"), nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(nil)
	deps.graphClient.EXPECT().RemoveMembership(gomock.Any(), gomock.Any()).
		Return(notFound())

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-y"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "not a member")
}

func TestRemove_TeamNotFound(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("team not found"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/NoTeam", "-u", "alice@c.com", "-y"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "team not found")
}

func TestRemove_TeamHasNoIdentity(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(&core.WebApiTeam{Name: types.ToPtr("NoIdentity")}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-y"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no underlying descriptor")
	assert.NotContains(t, err.Error(), "Group")
}

func TestRemove_MemberNotFound(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "ghost@x.com").
		Return(nil, errors.New("not found"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "ghost@x.com", "-y"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, buf.String(), "not found")
	assert.Contains(t, err.Error(), "failure(s)")
}

func TestRemove_TargetArg_ParsesOrgSlashProjectSlashTeam(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myCustomOrg")

	var captured core.GetTeamArgs
	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamArgs) (*core.WebApiTeam, error) {
			captured = args
			return teamResult("vssgp.X", "Team"), nil
		})
	deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "u@x.com").
		Return(subject("aad.Y", "U", "aad", "l"), nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(nil)
	deps.graphClient.EXPECT().RemoveMembership(gomock.Any(), gomock.Any()).
		Return(nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myCustomOrg/MyProject/MyTeam", "-u", "u@x.com", "-y"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, "MyProject", *captured.ProjectId)
	assert.Equal(t, "MyTeam", *captured.TeamId)
}

func TestRemove_JSONOutput_EnvelopeShape(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.Uy0xLTkt", "My Team"), nil)
	deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "alice@c.com").
		Return(subject("aad.YR5kM", "Alice", "aad", "la"), nil)
	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(nil)
	deps.graphClient.EXPECT().RemoveMembership(gomock.Any(), gomock.Any()).
		Return(nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-y", "--json"})
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

// --- Bulk tests (12–22) ---

func TestRemove_Bulk_DedupePreservesInputOrder(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)

	gomock.InOrder(
		deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "alice@c.com").
			Return(subject("aad.1", "Alice", "aad", "la"), nil),
		deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "bob@c.com").
			Return(subject("aad.2", "Bob", "aad", "lb"), nil),
	)

	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(nil).Times(2)
	deps.graphClient.EXPECT().RemoveMembership(gomock.Any(), gomock.Any()).
		Return(nil).Times(2)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-u", "bob@c.com", "-u", "alice@c.com", "-y"})
	err := cmd.Execute()
	require.NoError(t, err)

	aliceCount := strings.Count(buf.String(), "Alice")
	assert.Equal(t, 1, aliceCount, "Alice should appear only once")

	alicePos := bytes.Index(buf.Bytes(), []byte("Alice"))
	bobPos := bytes.Index(buf.Bytes(), []byte("Bob"))
	assert.True(t, alicePos < bobPos, "input order not preserved")
}

func TestRemove_Bulk_ExitCodePartialSuccess(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)

	gomock.InOrder(
		deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "alice@c.com").
			Return(subject("aad.1", "Alice", "aad", "la"), nil),
		deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "error@c.com").
			Return(subject("aad.2", "Error", "aad", "le"), nil),
		deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
			Return(nil),
		deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
			Return(nil),
		deps.graphClient.EXPECT().RemoveMembership(gomock.Any(), gomock.Any()).
			Return(nil),
		deps.graphClient.EXPECT().RemoveMembership(gomock.Any(), gomock.Any()).
			Return(apiError(http.StatusInternalServerError)),
	)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-u", "error@c.com", "-y"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failure(s)")
}

func TestRemove_Bulk_Prompt_NotShownWhenAllNotAMember(t *testing.T) {
	deps, buf := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeam(gomock.Any(), gomock.Any()).
		Return(teamResult("vssgp.X", "My Team"), nil)

	gomock.InOrder(
		deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "alice@c.com").
			Return(subject("aad.1", "Alice", "aad", "la"), nil),
		deps.extClient.EXPECT().ResolveSubject(gomock.Any(), "bob@c.com").
			Return(subject("aad.2", "Bob", "aad", "lb"), nil),
	)

	deps.graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(notFound()).Times(2)

	// No Prompter call expected
	// No RemoveMembership calls expected

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "-u", "alice@c.com", "-u", "bob@c.com"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "not a member")
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "Bob")
}

func TestRemove_MissingUserFlag(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}
