package delete

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

type fakeDeleteDeps struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	core       *mocks.MockCoreClient
	operations *mocks.MockOperationsClient
	prompter   *mocks.MockPrompter
	config     *mocks.MockConfig
	authCfg    *mocks.MockAuthConfig
}

func setupFakeDeps(t *testing.T, organization string) *fakeDeleteDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeDeleteDeps{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		core:       mocks.NewMockCoreClient(ctrl),
		operations: mocks.NewMockOperationsClient(ctrl),
		prompter:   mocks.NewMockPrompter(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		authCfg:    mocks.NewMockAuthConfig(ctrl),
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).AnyTimes()
	deps.clientFact.EXPECT().Core(gomock.Any(), organization).Return(deps.core, nil).AnyTimes()
	deps.clientFact.EXPECT().Operations(gomock.Any(), organization).Return(deps.operations, nil).AnyTimes()

	return deps
}

func queueDeleteProject(t *testing.T, deps *fakeDeleteDeps, projectID uuid.UUID) {
	t.Helper()

	deps.core.EXPECT().GetProject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetProjectArgs) (*core.TeamProject, error) {
			require.NotNil(t, args.ProjectId)
			return &core.TeamProject{Id: &projectID}, nil
		})
	deps.core.EXPECT().QueueDeleteProject(gomock.Any(), gomock.Any()).
		Return(&operations.OperationReference{
			Id:     &projectID,
			Status: &operations.OperationStatusValues.Succeeded,
			Url:    types.ToPtr("https://dev.azure.com/org/_apis/operations/" + projectID.String()),
		}, nil)
}

func TestDelete_ExplicitOrgColonForm(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	projectID := uuid.New()
	queueDeleteProject(t, deps, projectID)
	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg:myProject", "--no-wait"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestDelete_DefaultsToConfiguredOrganization(t *testing.T) {
	deps := setupFakeDeps(t, "defaultOrg")

	deps.cmd.EXPECT().Config().Return(deps.config, nil).AnyTimes()
	deps.config.EXPECT().Authentication().Return(deps.authCfg).AnyTimes()
	deps.authCfg.EXPECT().GetDefaultOrganization().Return("defaultOrg", nil).AnyTimes()

	projectID := uuid.New()
	queueDeleteProject(t, deps, projectID)
	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myProject", "--no-wait"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestDelete_LegacyOrgSlashFormRejected(t *testing.T) {
	deps := setupFakeDeps(t, "myOrg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject", "--yes"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")
}
