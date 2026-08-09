package list

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"go.uber.org/mock/gomock"
)

type fakeListDeps struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	core       *mocks.MockCoreClient
	config     *mocks.MockConfig
	authCfg    *mocks.MockAuthConfig
}

func setupFakeListDeps(t *testing.T, organization string) *fakeListDeps {
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
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().Core(gomock.Any(), organization).Return(deps.core, nil).AnyTimes()

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer(gomock.Any()).Return(tp, nil).AnyTimes()

	return deps
}

func sampleProject(name string) core.TeamProjectReference {
	id := uuid.New()
	return core.TeamProjectReference{
		Id:    &id,
		Name:  &name,
		State: &core.ProjectStateValues.WellFormed,
	}
}

func TestList_DefaultOrganization(t *testing.T) {
	deps := setupFakeListDeps(t, "defaultOrg")

	deps.cmd.EXPECT().Config().Return(deps.config, nil).AnyTimes()
	deps.config.EXPECT().Authentication().Return(deps.authCfg).AnyTimes()
	deps.authCfg.EXPECT().GetDefaultOrganization().Return("defaultOrg", nil).AnyTimes()

	projects := []core.TeamProjectReference{sampleProject("Project A")}
	deps.core.EXPECT().GetProjects(gomock.Any(), gomock.Any()).
		Return(&core.GetProjectsResponseValue{Value: projects}, nil)

	cmd := NewCmdProjectList(deps.cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_BareOrgForm(t *testing.T) {
	deps := setupFakeListDeps(t, "myOrg")

	projects := []core.TeamProjectReference{sampleProject("Project A")}
	deps.core.EXPECT().GetProjects(gomock.Any(), gomock.Any()).
		Return(&core.GetProjectsResponseValue{Value: projects}, nil)

	cmd := NewCmdProjectList(deps.cmd)
	cmd.SetArgs([]string{"myOrg"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_ColonOrgForm(t *testing.T) {
	deps := setupFakeListDeps(t, "myOrg")

	projects := []core.TeamProjectReference{sampleProject("Project A")}
	deps.core.EXPECT().GetProjects(gomock.Any(), gomock.Any()).
		Return(&core.GetProjectsResponseValue{Value: projects}, nil)

	cmd := NewCmdProjectList(deps.cmd)
	cmd.SetArgs([]string{"myOrg:"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_LegacyOrgSlashProjectRejected(t *testing.T) {
	deps := setupFakeListDeps(t, "myOrg")

	cmd := NewCmdProjectList(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project scope not allowed for this command")
}

func TestList_NoProjectMarkerRejected(t *testing.T) {
	deps := setupFakeListDeps(t, "myOrg")

	cmd := NewCmdProjectList(deps.cmd)
	cmd.SetArgs([]string{"/myOrg"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected ORG or ORG:")
}
