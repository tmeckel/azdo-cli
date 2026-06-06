package update

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestUpdate_RequiresNameOrDescription(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of --name or --description is required")
}

func TestUpdate_MissingTeamArg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "team argument required")
}

type fakeUpdateDeps struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	core       *mocks.MockCoreClient
	config     *mocks.MockConfig
	authCfg    *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func setupFakeUpdateDeps(t *testing.T, organization string) *fakeUpdateDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeUpdateDeps{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		core:       mocks.NewMockCoreClient(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		authCfg:    mocks.NewMockAuthConfig(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().Core(gomock.Any(), organization).Return(deps.core, nil).AnyTimes()

	tp, err := printer.NewListPrinter(out)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	return deps
}

func TestUpdate_TargetArg_ParsesOrgSlashProjectSlashTeam(t *testing.T) {
	deps := setupFakeUpdateDeps(t, "myOrg")

	teamID := uuid.New()
	teamName := "UpdatedTeam"

	deps.core.EXPECT().UpdateTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.UpdateTeamArgs) (*core.WebApiTeam, error) {
			require.NotNil(t, args.ProjectId)
			assert.Equal(t, "myProject", *args.ProjectId)
			require.NotNil(t, args.TeamId)
			assert.Equal(t, "My Team", *args.TeamId)
			return &core.WebApiTeam{
				Id:   &teamID,
				Name: &teamName,
			}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/My Team", "--name", "UpdatedTeam"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestUpdate_DefaultsToConfiguredOrganization(t *testing.T) {
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

	defaultOrg := "defaultOrg"

	mockCmdCtx.EXPECT().Config().Return(mockConfig, nil).AnyTimes()
	mockConfig.EXPECT().Authentication().Return(mockAuthCfg).AnyTimes()
	mockAuthCfg.EXPECT().GetDefaultOrganization().Return(defaultOrg, nil).AnyTimes()
	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockClientFactory.EXPECT().Core(gomock.Any(), defaultOrg).Return(mockCoreClient, nil).AnyTimes()

	teamID := uuid.New()

	mockCoreClient.EXPECT().UpdateTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.UpdateTeamArgs) (*core.WebApiTeam, error) {
			require.NotNil(t, args.ProjectId)
			assert.Equal(t, "myProject", *args.ProjectId)
			return &core.WebApiTeam{
				Id:   &teamID,
				Name: types.ToPtr("UpdatedTeam"),
			}, nil
		})

	tp, err := printer.NewListPrinter(out)
	require.NoError(t, err)
	mockCmdCtx.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{"myProject/My Team", "--name", "UpdatedTeam"})
	err = cmd.Execute()
	require.NoError(t, err)
}

func TestUpdate_PayloadContainsNameOnly(t *testing.T) {
	deps := setupFakeUpdateDeps(t, "myOrg")

	teamID := uuid.New()

	deps.core.EXPECT().UpdateTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.UpdateTeamArgs) (*core.WebApiTeam, error) {
			require.NotNil(t, args.TeamData)
			require.NotNil(t, args.TeamData.Name)
			assert.Equal(t, "NewName", *args.TeamData.Name)
			return &core.WebApiTeam{
				Id:   &teamID,
				Name: types.ToPtr("NewName"),
			}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--name", "NewName"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestUpdate_PayloadContainsDescriptionOnly(t *testing.T) {
	deps := setupFakeUpdateDeps(t, "myOrg")

	teamID := uuid.New()

	deps.core.EXPECT().UpdateTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.UpdateTeamArgs) (*core.WebApiTeam, error) {
			require.NotNil(t, args.TeamData)
			require.NotNil(t, args.TeamData.Description)
			assert.Equal(t, "NewDesc", *args.TeamData.Description)
			return &core.WebApiTeam{
				Id:   &teamID,
				Name: types.ToPtr("MyTeam"),
			}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--description", "NewDesc"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestUpdate_PayloadContainsBoth(t *testing.T) {
	deps := setupFakeUpdateDeps(t, "myOrg")

	teamID := uuid.New()

	deps.core.EXPECT().UpdateTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.UpdateTeamArgs) (*core.WebApiTeam, error) {
			require.NotNil(t, args.TeamData)
			require.NotNil(t, args.TeamData.Name)
			require.NotNil(t, args.TeamData.Description)
			assert.Equal(t, "N", *args.TeamData.Name)
			assert.Equal(t, "D", *args.TeamData.Description)
			return &core.WebApiTeam{
				Id:   &teamID,
				Name: types.ToPtr("N"),
			}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--name", "N", "--description", "D"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestUpdate_JSONOutput(t *testing.T) {
	deps := setupFakeUpdateDeps(t, "myOrg")

	teamID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	teamName := "MyTeam"
	teamURL := "https://dev.azure.com/myOrg/_apis/teams/11111111-1111-1111-1111-111111111111"

	deps.core.EXPECT().UpdateTeam(gomock.Any(), gomock.Any()).
		Return(&core.WebApiTeam{
			Id:   &teamID,
			Name: &teamName,
			Url:  &teamURL,
		}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--name", "MyTeam", "--json=id,name,description"})
	err := cmd.Execute()
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &parsed))
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", parsed["id"])
	assert.Equal(t, "MyTeam", parsed["name"])
	assert.Equal(t, nil, parsed["description"])
	assert.NotContains(t, parsed, "url")
}

func TestUpdate_TableOutput_ContainsAllColumns(t *testing.T) {
	deps := setupFakeUpdateDeps(t, "myOrg")

	teamID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	teamName := "UpdatedTeam"
	teamDesc := "updated desc"
	projectName := "myProject"
	teamURL := "https://dev.azure.com/myOrg/_apis/teams/22222222-2222-2222-2222-222222222222"

	deps.core.EXPECT().UpdateTeam(gomock.Any(), gomock.Any()).
		Return(&core.WebApiTeam{
			Id:          &teamID,
			Name:        &teamName,
			Description: &teamDesc,
			ProjectName: &projectName,
			Url:         &teamURL,
		}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--name", "UpdatedTeam", "--description", "updated desc"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "ID:")
	assert.Contains(t, output, "NAME:")
	assert.Contains(t, output, "DESCRIPTION:")
	assert.Contains(t, output, "PROJECT:")
	assert.Contains(t, output, "URL:")
	assert.Contains(t, output, "22222222-2222-2222-2222-222222222222")
	assert.Contains(t, output, "UpdatedTeam")
	assert.Contains(t, output, "updated desc")
	assert.Contains(t, output, "myProject")
}

func TestUpdate_PropagatesSDKError(t *testing.T) {
	deps := setupFakeUpdateDeps(t, "myOrg")

	deps.core.EXPECT().UpdateTeam(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("API error"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--name", "MyTeam"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update team: API error")
}
