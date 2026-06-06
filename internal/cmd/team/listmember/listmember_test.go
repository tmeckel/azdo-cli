package listmember

import (
	"bytes"
	"context"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"go.uber.org/mock/gomock"
)

type fakeDeps struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	coreClient *mocks.MockCoreClient
	prompter   *mocks.MockPrompter
	config     *mocks.MockConfig
	authCfg    *mocks.MockAuthConfig
}

func setupFakeDeps(t *testing.T, organization string) (*fakeDeps, *bytes.Buffer) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)

	deps := &fakeDeps{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		coreClient: mocks.NewMockCoreClient(ctrl),
		prompter:   mocks.NewMockPrompter(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		authCfg:    mocks.NewMockAuthConfig(ctrl),
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Printer(gomock.Any()).Return(tp, nil).AnyTimes()
	deps.clientFact.EXPECT().Core(gomock.Any(), organization).Return(deps.coreClient, nil).AnyTimes()

	return deps, out
}

func member(id, displayName, uniqueName string, isAdmin bool) webapi.TeamMember {
	m := webapi.TeamMember{
		Identity: &webapi.IdentityRef{
			Id:          &id,
			DisplayName: &displayName,
			UniqueName:  &uniqueName,
		},
		IsTeamAdmin: &isAdmin,
	}
	return m
}

func TestList_EmptyResult(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	deps.coreClient.EXPECT().GetTeamMembersWithExtendedProperties(gomock.Any(), gomock.Any()).
		Return(&[]webapi.TeamMember{}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_NoFilters(t *testing.T) {
	deps, out := setupFakeDeps(t, "myOrg")

	members := []webapi.TeamMember{
		member("3", "Charlie", "c@x", false),
		member("1", "Alice", "a@x", true),
		member("2", "Bob", "b@x", false),
	}

	deps.coreClient.EXPECT().GetTeamMembersWithExtendedProperties(gomock.Any(), gomock.Any()).
		Return(&members, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "Bob")
	assert.Contains(t, output, "Charlie")

	aliceIdx := bytes.Index(out.Bytes(), []byte("Alice"))
	bobIdx := bytes.Index(out.Bytes(), []byte("Bob"))
	charlieIdx := bytes.Index(out.Bytes(), []byte("Charlie"))
	assert.True(t, aliceIdx < bobIdx && bobIdx < charlieIdx, "expected sorted order")
}

func TestList_FiltersPassedToSDK(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	var capturedArgs core.GetTeamMembersWithExtendedPropertiesArgs
	deps.coreClient.EXPECT().GetTeamMembersWithExtendedProperties(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamMembersWithExtendedPropertiesArgs) (*[]webapi.TeamMember, error) {
			capturedArgs = args
			return &[]webapi.TeamMember{}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--top", "10", "--skip", "5"})
	err := cmd.Execute()
	require.NoError(t, err)

	require.NotNil(t, capturedArgs.Top)
	assert.Equal(t, 10, *capturedArgs.Top)
	require.NotNil(t, capturedArgs.Skip)
	assert.Equal(t, 5, *capturedArgs.Skip)
}

func TestList_MaxItemsCap(t *testing.T) {
	deps, out := setupFakeDeps(t, "myOrg")

	members := []webapi.TeamMember{
		member("1", "A", "a@x", false),
		member("2", "B", "b@x", false),
		member("3", "C", "c@x", false),
		member("4", "D", "d@x", false),
		member("5", "E", "e@x", false),
	}

	deps.coreClient.EXPECT().GetTeamMembersWithExtendedProperties(gomock.Any(), gomock.Any()).
		Return(&members, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--max-items", "3"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, out.String(), "A")
	assert.Contains(t, out.String(), "B")
	assert.Contains(t, out.String(), "C")
	assert.NotContains(t, out.String(), "D")
	assert.NotContains(t, out.String(), "E")
}

func TestList_JSONOutput(t *testing.T) {
	deps, out := setupFakeDeps(t, "myOrg")

	display := "Alice"
	members := []webapi.TeamMember{
		{
			Identity: &webapi.IdentityRef{
				DisplayName: &display,
			},
			IsTeamAdmin: boolPtr(true),
		},
	}

	deps.coreClient.EXPECT().GetTeamMembersWithExtendedProperties(gomock.Any(), gomock.Any()).
		Return(&members, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--json=identity,isTeamAdmin"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "displayName")
	assert.Contains(t, output, "isTeamAdmin")
	assert.NotContains(t, output, "ID")
	assert.NotContains(t, output, "DISPLAY NAME")
}

func TestList_PaginatesUntilShortPage(t *testing.T) {
	deps, out := setupFakeDeps(t, "myOrg")

	page1 := []webapi.TeamMember{
		member("1", "A", "a@x", false),
		member("2", "B", "b@x", false),
	}
	page2 := []webapi.TeamMember{
		member("3", "C", "c@x", false),
	}

	var callCount int
	deps.coreClient.EXPECT().GetTeamMembersWithExtendedProperties(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamMembersWithExtendedPropertiesArgs) (*[]webapi.TeamMember, error) {
			callCount++
			if callCount == 1 {
				if args.Skip != nil {
					assert.Equal(t, 0, *args.Skip)
				}
				return &page1, nil
			}
			require.NotNil(t, args.Skip)
			assert.Equal(t, 2, *args.Skip)
			return &page2, nil
		}).Times(2)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--top", "2"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
	assert.Contains(t, out.String(), "A")
	assert.Contains(t, out.String(), "B")
	assert.Contains(t, out.String(), "C")
}

func TestList_TargetArg_ParsesOrgSlashProjectSlashTeam(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	var capturedArgs core.GetTeamMembersWithExtendedPropertiesArgs
	deps.coreClient.EXPECT().GetTeamMembersWithExtendedProperties(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamMembersWithExtendedPropertiesArgs) (*[]webapi.TeamMember, error) {
			capturedArgs = args
			return &[]webapi.TeamMember{}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/My Team"})
	err := cmd.Execute()
	require.NoError(t, err)

	require.NotNil(t, capturedArgs.ProjectId)
	assert.Equal(t, "myProject", *capturedArgs.ProjectId)
	require.NotNil(t, capturedArgs.TeamId)
	assert.Equal(t, "My Team", *capturedArgs.TeamId)
}

func TestList_IdentityRefs_NilSafe(t *testing.T) {
	deps, _ := setupFakeDeps(t, "myOrg")

	members := []webapi.TeamMember{
		{Identity: nil, IsTeamAdmin: nil},
	}

	deps.coreClient.EXPECT().GetTeamMembersWithExtendedProperties(gomock.Any(), gomock.Any()).
		Return(&members, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_IdentityDisplayFallsBackToUniqueName(t *testing.T) {
	deps, out := setupFakeDeps(t, "myOrg")

	uniqueName := "user@example.com"
	members := []webapi.TeamMember{
		{
			Identity: &webapi.IdentityRef{
				Id:         strPtr("1"),
				UniqueName: &uniqueName,
			},
			IsTeamAdmin: boolPtr(false),
		},
	}

	deps.coreClient.EXPECT().GetTeamMembersWithExtendedProperties(gomock.Any(), gomock.Any()).
		Return(&members, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, out.String(), "user@example.com")
}

func boolPtr(b bool) *bool {
	return &b
}

func strPtr(s string) *string {
	return &s
}
