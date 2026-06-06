package delete

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"go.uber.org/mock/gomock"
)

type fakeDeleteDeps struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	core       *mocks.MockCoreClient
	prompter   *mocks.MockPrompter
	stdout     *bytes.Buffer
}

func setupFakeDeleteDeps(t *testing.T, organization string) *fakeDeleteDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeDeleteDeps{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		core:       mocks.NewMockCoreClient(ctrl),
		prompter:   mocks.NewMockPrompter(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).AnyTimes()
	deps.clientFact.EXPECT().Core(gomock.Any(), organization).Return(deps.core, nil).AnyTimes()

	return deps
}

func TestDelete_MissingTeamArg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "team argument required")
}

func TestDelete_RequiresConfirmationByDefault(t *testing.T) {
	deps := setupFakeDeleteDeps(t, "myOrg")

	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	assert.ErrorIs(t, err, util.ErrCancel)
}

func TestDelete_YesFlagSkipsConfirmation(t *testing.T) {
	deps := setupFakeDeleteDeps(t, "myOrg")

	deps.core.EXPECT().DeleteTeam(gomock.Any(), gomock.Any()).Return(nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--yes"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Empty(t, deps.stdout.String())
}

func TestDelete_ConfirmationAccept_InvokesDeleteTeam(t *testing.T) {
	deps := setupFakeDeleteDeps(t, "myOrg")

	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)
	deps.core.EXPECT().DeleteTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.DeleteTeamArgs) error {
			require.NotNil(t, args.ProjectId)
			require.NotNil(t, args.TeamId)
			assert.Equal(t, "myProject", *args.ProjectId)
			assert.Equal(t, "My Team", *args.TeamId)
			return nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/My Team"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestDelete_TargetArg_ParsesOrgSlashProjectSlashTeam(t *testing.T) {
	deps := setupFakeDeleteDeps(t, "myOrg")

	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)
	deps.core.EXPECT().DeleteTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.DeleteTeamArgs) error {
			require.NotNil(t, args.ProjectId)
			assert.Equal(t, "myProject", *args.ProjectId)
			require.NotNil(t, args.TeamId)
			assert.Equal(t, "My Team", *args.TeamId)
			return nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/My Team"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestDelete_DefaultsToConfiguredOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)
	mockCoreClient := mocks.NewMockCoreClient(ctrl)
	mockConfig := mocks.NewMockConfig(ctrl)
	mockAuthCfg := mocks.NewMockAuthConfig(ctrl)
	mockPrompter := mocks.NewMockPrompter(ctrl)

	defaultOrg := "defaultOrg"

	mockCmdCtx.EXPECT().Config().Return(mockConfig, nil).AnyTimes()
	mockConfig.EXPECT().Authentication().Return(mockAuthCfg).AnyTimes()
	mockAuthCfg.EXPECT().GetDefaultOrganization().Return(defaultOrg, nil).AnyTimes()
	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockCmdCtx.EXPECT().Prompter().Return(mockPrompter, nil).AnyTimes()
	mockClientFactory.EXPECT().Core(gomock.Any(), defaultOrg).Return(mockCoreClient, nil).AnyTimes()
	mockCoreClient.EXPECT().DeleteTeam(gomock.Any(), gomock.Any()).Return(nil)
	mockPrompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{"myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Empty(t, out.String())
}

func TestDelete_TTYOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(true)
	io.SetStderrTTY(false)

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)
	mockCoreClient := mocks.NewMockCoreClient(ctrl)
	mockPrompter := mocks.NewMockPrompter(ctrl)

	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockCmdCtx.EXPECT().Prompter().Return(mockPrompter, nil).AnyTimes()
	mockClientFactory.EXPECT().Core(gomock.Any(), "myOrg").Return(mockCoreClient, nil).AnyTimes()
	mockCoreClient.EXPECT().DeleteTeam(gomock.Any(), gomock.Any()).Return(nil)
	mockPrompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "deleted successfully")
	assert.Contains(t, output, "MyTeam")
}

func TestDelete_NonTTYOutput(t *testing.T) {
	deps := setupFakeDeleteDeps(t, "myOrg")

	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)
	deps.core.EXPECT().DeleteTeam(gomock.Any(), gomock.Any()).Return(nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Empty(t, deps.stdout.String())
}

func TestDelete_PropagatesSDKError(t *testing.T) {
	deps := setupFakeDeleteDeps(t, "myOrg")

	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)
	deps.core.EXPECT().DeleteTeam(gomock.Any(), gomock.Any()).Return(errors.New("API error"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete team: API error")
}
