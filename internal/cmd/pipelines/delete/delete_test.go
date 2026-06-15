package delete

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deleteDeps struct {
	cmd         *mocks.MockCmdContext
	clientFact  *mocks.MockClientFactory
	buildClient *mocks.MockBuildClient
	prompter    *mocks.MockPrompter
	stdout      *bytes.Buffer
	t           *testing.T
}

func setupDeleteDeps(t *testing.T, organization string) *deleteDeps {
	return setupDeleteDepsWithPrompt(t, organization, true)
}

func setupDeleteDepsWithPrompt(t *testing.T, organization string, canPrompt bool) *deleteDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdinTTY(canPrompt)
	io.SetStdoutTTY(canPrompt)
	io.SetStderrTTY(canPrompt)

	deps := &deleteDeps{
		cmd:         mocks.NewMockCmdContext(ctrl),
		clientFact:  mocks.NewMockClientFactory(ctrl),
		buildClient: mocks.NewMockBuildClient(ctrl),
		prompter:    mocks.NewMockPrompter(ctrl),
		stdout:      out,
		t:           t,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).AnyTimes()
	deps.clientFact.EXPECT().Build(gomock.Any(), organization).Return(deps.buildClient, nil).AnyTimes()

	return deps
}

func setupDefaultOrgDeps(t *testing.T, organization string) *deleteDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	cfg := mocks.NewMockConfig(ctrl)
	authCfg := mocks.NewMockAuthConfig(ctrl)
	deps := &deleteDeps{
		cmd:         mocks.NewMockCmdContext(ctrl),
		clientFact:  mocks.NewMockClientFactory(ctrl),
		buildClient: mocks.NewMockBuildClient(ctrl),
		prompter:    mocks.NewMockPrompter(ctrl),
		stdout:      out,
		t:           t,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).AnyTimes()
	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(authCfg).AnyTimes()
	authCfg.EXPECT().GetDefaultOrganization().Return(organization, nil).AnyTimes()
	deps.clientFact.EXPECT().Build(gomock.Any(), organization).Return(deps.buildClient, nil).AnyTimes()

	return deps
}

func TestNewCmd_RegistersAsDeleteLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)

	assert.Equal(t, "delete [ORGANIZATION/]PROJECT/PIPELINE", cmd.Use)
	assert.ElementsMatch(t, []string{"d", "del", "rm"}, cmd.Aliases)
	require.NotNil(t, cmd.RunE)
	assert.Nil(t, cmd.Flags().Lookup("json"))
}

func TestNewCmd_RequiresOneArg(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline target is required")
}

func TestRunDelete_ByPositiveID(t *testing.T) {
	t.Parallel()
	deps := setupDeleteDeps(t, "MyOrg")
	deps.expectDeleteDefinition("Fabrikam", 42, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42", "--yes"})
	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Pipeline 42 was deleted successfully.\n", deps.stdout.String())
}

func TestRunDelete_ByName(t *testing.T) {
	t.Parallel()
	deps := setupDeleteDeps(t, "MyOrg")
	deps.expectGetDefinitions("Fabrikam", "My Pipeline", &build.GetDefinitionsResponseValue{
		Value: []build.BuildDefinitionReference{{Id: types.ToPtr(42), Name: types.ToPtr("My Pipeline")}},
	}, nil)
	deps.expectDeleteDefinition("Fabrikam", 42, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/My Pipeline"})
	deps.prompter.EXPECT().Confirm("Are you sure you want to delete this pipeline?", false).Return(true, nil)
	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Pipeline 42 was deleted successfully.\n", deps.stdout.String())
}

func TestRunDelete_DefaultsToConfiguredOrganization(t *testing.T) {
	t.Parallel()
	deps := setupDefaultOrgDeps(t, "DefaultOrg")
	deps.expectDeleteDefinition("Fabrikam", 42, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"Fabrikam/42", "--yes"})
	err := cmd.Execute()

	require.NoError(t, err)
	assert.Equal(t, "Pipeline 42 was deleted successfully.\n", deps.stdout.String())
}

func TestRunDelete_RejectsNonPositiveNumericID(t *testing.T) {
	t.Parallel()
	deps := setupDeleteDeps(t, "MyOrg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/0", "--yes"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline id must be greater than zero")
	assert.Empty(t, deps.stdout.String())
}

func TestRunDelete_NameNotFound(t *testing.T) {
	t.Parallel()
	deps := setupDeleteDeps(t, "MyOrg")
	deps.expectGetDefinitions("Fabrikam", "Ghost", &build.GetDefinitionsResponseValue{}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/Ghost", "--yes"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Empty(t, deps.stdout.String())
}

func TestRunDelete_NameAmbiguous(t *testing.T) {
	t.Parallel()
	deps := setupDeleteDeps(t, "MyOrg")
	deps.expectGetDefinitions("Fabrikam", "SameName", &build.GetDefinitionsResponseValue{
		Value: []build.BuildDefinitionReference{
			{Id: types.ToPtr(1), Name: types.ToPtr("SameName")},
			{Id: types.ToPtr(2), Name: types.ToPtr("SameName")},
		},
	}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/SameName", "--yes"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
	assert.Empty(t, deps.stdout.String())
}

func TestRunDelete_RequiresYesWhenNotInteractive(t *testing.T) {
	t.Parallel()
	deps := setupDeleteDepsWithPrompt(t, "MyOrg", false)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes required when not running interactively")
	assert.Empty(t, deps.stdout.String())
}

func TestRunDelete_ConfirmationCanceled(t *testing.T) {
	t.Parallel()
	deps := setupDeleteDeps(t, "MyOrg")
	deps.prompter.EXPECT().Confirm("Are you sure you want to delete this pipeline?", false).Return(false, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42"})
	err := cmd.Execute()

	require.ErrorIs(t, err, util.ErrCancel)
	assert.Empty(t, deps.stdout.String())
}

func TestRunDelete_PropagatesDeleteError(t *testing.T) {
	t.Parallel()
	deps := setupDeleteDeps(t, "MyOrg")
	deps.expectDeleteDefinition("Fabrikam", 42, errors.New("API error"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42", "--yes"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete pipeline 42: API error")
}

func (d *deleteDeps) expectGetDefinitions(project, name string, resp *build.GetDefinitionsResponseValue, err error) {
	d.t.Helper()
	d.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
			require.NotNil(d.t, args.Project)
			require.Equal(d.t, project, *args.Project)
			require.NotNil(d.t, args.Name)
			require.Equal(d.t, name, *args.Name)
			return resp, err
		},
	).Times(1)
}

func (d *deleteDeps) expectDeleteDefinition(project string, id int, err error) {
	d.t.Helper()
	d.buildClient.EXPECT().DeleteDefinition(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.DeleteDefinitionArgs) error {
			require.NotNil(d.t, args.Project)
			require.Equal(d.t, project, *args.Project)
			require.NotNil(d.t, args.DefinitionId)
			require.Equal(d.t, id, *args.DefinitionId)
			return err
		},
	).Times(1)
}
