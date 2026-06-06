package create

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

func TestNewCmd_RequiresNameFlag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{"myproject"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required flag(s) \"name\" not set")
}

func TestNewCmd_MissingProjectArg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project argument required")
}

type fakeCreateDeps struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	core       *mocks.MockCoreClient
	config     *mocks.MockConfig
	authCfg    *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func setupFakeCreateDeps(t *testing.T, organization string) *fakeCreateDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeCreateDeps{
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

func TestCreate_ProjectScope_ParsesOrgSlashProject(t *testing.T) {
	deps := setupFakeCreateDeps(t, "myOrg")

	teamID := uuid.New()
	teamName := "MyTeam"
	teamDesc := "desc"
	projectName := "myProject"
	teamURL := "https://dev.azure.com/myOrg/_apis/teams/" + teamID.String()

	deps.core.EXPECT().CreateTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.CreateTeamArgs) (*core.WebApiTeam, error) {
			require.NotNil(t, args.ProjectId)
			assert.Equal(t, "myProject", *args.ProjectId)
			return &core.WebApiTeam{
				Id:          &teamID,
				Name:        &teamName,
				Description: &teamDesc,
				ProjectName: &projectName,
				Url:         &teamURL,
			}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--name", "MyTeam", "--description", "desc"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestCreate_DefaultsToConfiguredOrganization(t *testing.T) {
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
	teamName := "MyTeam"

	mockCoreClient.EXPECT().CreateTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.CreateTeamArgs) (*core.WebApiTeam, error) {
			require.NotNil(t, args.ProjectId)
			assert.Equal(t, "myProject", *args.ProjectId)
			return &core.WebApiTeam{
				Id:   &teamID,
				Name: &teamName,
			}, nil
		})

	tp, err := printer.NewListPrinter(out)
	require.NoError(t, err)
	mockCmdCtx.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{"myProject", "--name", "MyTeam"})
	err = cmd.Execute()
	require.NoError(t, err)
}

func TestCreate_PayloadContainsNameAndDescription(t *testing.T) {
	deps := setupFakeCreateDeps(t, "myOrg")

	teamID := uuid.New()

	deps.core.EXPECT().CreateTeam(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.CreateTeamArgs) (*core.WebApiTeam, error) {
			require.NotNil(t, args.Team)
			require.NotNil(t, args.Team.Name)
			require.NotNil(t, args.Team.Description)
			assert.Equal(t, "MyTeam", *args.Team.Name)
			assert.Equal(t, "my desc", *args.Team.Description)
			return &core.WebApiTeam{
				Id:   &teamID,
				Name: types.ToPtr("MyTeam"),
			}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--name", "MyTeam", "--description", "my desc"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestCreate_JSONOutput(t *testing.T) {
	deps := setupFakeCreateDeps(t, "myOrg")

	teamID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	teamName := "MyTeam"
	teamURL := "https://dev.azure.com/myOrg/_apis/teams/11111111-1111-1111-1111-111111111111"

	deps.core.EXPECT().CreateTeam(gomock.Any(), gomock.Any()).
		Return(&core.WebApiTeam{
			Id:   &teamID,
			Name: &teamName,
			Url:  &teamURL,
		}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--name", "MyTeam", "--json=id,name,description"})
	err := cmd.Execute()
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &parsed))
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", parsed["id"])
	assert.Equal(t, "MyTeam", parsed["name"])
	assert.Equal(t, nil, parsed["description"])
	assert.NotContains(t, parsed, "url")
}

func TestCreate_TableOutput_ContainsAllColumns(t *testing.T) {
	deps := setupFakeCreateDeps(t, "myOrg")

	teamID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	teamName := "MyTeam"
	teamDesc := "my team desc"
	projectName := "myProject"
	teamURL := "https://dev.azure.com/myOrg/_apis/teams/22222222-2222-2222-2222-222222222222"

	deps.core.EXPECT().CreateTeam(gomock.Any(), gomock.Any()).
		Return(&core.WebApiTeam{
			Id:          &teamID,
			Name:        &teamName,
			Description: &teamDesc,
			ProjectName: &projectName,
			Url:         &teamURL,
		}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--name", "MyTeam", "--description", "my team desc"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "ID:")
	assert.Contains(t, output, "NAME:")
	assert.Contains(t, output, "DESCRIPTION:")
	assert.Contains(t, output, "PROJECT:")
	assert.Contains(t, output, "URL:")
	assert.Contains(t, output, "22222222-2222-2222-2222-222222222222")
	assert.Contains(t, output, "MyTeam")
	assert.Contains(t, output, "my team desc")
	assert.Contains(t, output, "myProject")
}

func TestCreate_PropagatesSDKError(t *testing.T) {
	deps := setupFakeCreateDeps(t, "myOrg")

	deps.core.EXPECT().CreateTeam(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("API error"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--name", "MyTeam"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create team: API error")
}
