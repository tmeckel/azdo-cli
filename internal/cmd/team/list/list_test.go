package list

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
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

type fakeListDeps struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	core       *mocks.MockCoreClient
	config     *mocks.MockConfig
	authCfg    *mocks.MockAuthConfig
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
		core:       mocks.NewMockCoreClient(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		authCfg:    mocks.NewMockAuthConfig(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().Core(gomock.Any(), organization).Return(deps.core, nil).AnyTimes()

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	return deps
}

func sampleTeams() []core.WebApiTeam {
	beta := uuid.MustParse("10000000-0000-0000-0000-000000000002")
	alpha := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	gamma := uuid.MustParse("10000000-0000-0000-0000-000000000003")

	return []core.WebApiTeam{
		{
			Id:          &beta,
			Name:        types.ToPtr("Beta Team"),
			Description: types.ToPtr("Second team"),
			ProjectName: types.ToPtr("Fabrikam"),
		},
		{
			Id:          &alpha,
			Name:        types.ToPtr("Alpha Team"),
			Description: types.ToPtr("First team"),
			ProjectName: types.ToPtr("Fabrikam"),
		},
		{
			Id:          &gamma,
			Name:        types.ToPtr("Gamma Team"),
			Description: types.ToPtr("Third team"),
			ProjectName: types.ToPtr("Fabrikam"),
		},
	}
}

func TestList_EmptyResult(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	deps.core.EXPECT().GetTeams(gomock.Any(), gomock.Any()).
		Return(&[]core.WebApiTeam{}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_NoFilters(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	teams := sampleTeams()
	deps.core.EXPECT().GetTeams(gomock.Any(), gomock.Any()).
		Return(&teams, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "Alpha Team")
	assert.Contains(t, output, "Beta Team")
	assert.Contains(t, output, "Gamma Team")

	rIdx := strings.Index(output, "Alpha Team")
	bIdx := strings.Index(output, "Beta Team")
	gIdx := strings.Index(output, "Gamma Team")
	assert.True(t, rIdx < bIdx, "Alpha Team should appear before Beta Team")
	assert.True(t, bIdx < gIdx, "Beta Team should appear before Gamma Team")
}

func TestList_FiltersPassedToSDK(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	deps.core.EXPECT().GetTeams(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamsArgs) (*[]core.WebApiTeam, error) {
			assert.NotNil(t, args.Top)
			assert.Equal(t, 10, *args.Top)
			assert.NotNil(t, args.Skip)
			assert.Equal(t, 5, *args.Skip)
			assert.NotNil(t, args.Mine)
			assert.True(t, *args.Mine)
			return &[]core.WebApiTeam{}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--top", "10", "--skip", "5", "--mine"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_MaxItemsCap(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	teams := sampleTeams()
	deps.core.EXPECT().GetTeams(gomock.Any(), gomock.Any()).
		Return(&teams, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--max-items", "2"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "Alpha Team")
	assert.Contains(t, output, "Beta Team")
	assert.NotContains(t, output, "Gamma Team")
}

func TestList_JSONOutput(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	beta := uuid.MustParse("10000000-0000-0000-0000-000000000002")
	teams := []core.WebApiTeam{
		{Id: &beta, Name: types.ToPtr("Beta Team"), Description: types.ToPtr("Second team")},
	}
	deps.core.EXPECT().GetTeams(gomock.Any(), gomock.Any()).
		Return(&teams, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--json=id,name"})
	err := cmd.Execute()
	require.NoError(t, err)

	var parsed []map[string]any
	require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &parsed))
	require.Len(t, parsed, 1)
	assert.Equal(t, "10000000-0000-0000-0000-000000000002", parsed[0]["id"])
	assert.Equal(t, "Beta Team", parsed[0]["name"])
	assert.NotContains(t, parsed[0], "description")
}

func TestList_PaginatesUntilShortPage(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	page1 := []core.WebApiTeam{
		{Id: types.ToPtr(uuid.MustParse("10000000-0000-0000-0000-000000000001")), Name: types.ToPtr("Team A"), ProjectName: types.ToPtr("P")},
		{Id: types.ToPtr(uuid.MustParse("10000000-0000-0000-0000-000000000002")), Name: types.ToPtr("Team B"), ProjectName: types.ToPtr("P")},
	}
	page2 := []core.WebApiTeam{
		{Id: types.ToPtr(uuid.MustParse("10000000-0000-0000-0000-000000000003")), Name: types.ToPtr("Team C"), ProjectName: types.ToPtr("P")},
	}

	var callCount int
	deps.core.EXPECT().GetTeams(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamsArgs) (*[]core.WebApiTeam, error) {
			callCount++
			switch callCount {
			case 1:
				assert.Nil(t, args.Skip)
				return &page1, nil
			case 2:
				require.NotNil(t, args.Skip)
				assert.Equal(t, 2, *args.Skip)
				return &page2, nil
			default:
				return &[]core.WebApiTeam{}, nil
			}
		}).Times(2)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--top", "2"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, 2, callCount)

	output := deps.stdout.String()
	assert.Contains(t, output, "Team A")
	assert.Contains(t, output, "Team B")
	assert.Contains(t, output, "Team C")
}

func TestList_ScopeArg_ParsesOrgSlashProject(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)
	mockCoreClient := mocks.NewMockCoreClient(ctrl)

	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockClientFactory.EXPECT().Core(gomock.Any(), "myOrg").Return(mockCoreClient, nil).AnyTimes()

	mockCoreClient.EXPECT().GetTeams(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetTeamsArgs) (*[]core.WebApiTeam, error) {
			require.NotNil(t, args.ProjectId)
			assert.Equal(t, "myProject", *args.ProjectId)
			return &[]core.WebApiTeam{}, nil
		})

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	mockCmdCtx.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{"myOrg/myProject"})
	err = cmd.Execute()
	require.NoError(t, err)
}

func TestList_PropagatesSDKError(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	deps.core.EXPECT().GetTeams(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("API error"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list teams: API error")
}

func TestList_MaxItemsNegative_ReturnsError(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--max-items", "-1"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--max-items must be >= 0")
}

func TestList_DefaultsToConfiguredOrganization(t *testing.T) {
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

	mockCoreClient.EXPECT().GetTeams(gomock.Any(), gomock.Any()).
		Return(&[]core.WebApiTeam{}, nil)

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	mockCmdCtx.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{"myProject"})
	err = cmd.Execute()
	require.NoError(t, err)
}
