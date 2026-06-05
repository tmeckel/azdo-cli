package show

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestShowCmd_NumericTargets(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, outBuf, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(taskClient, nil)

	taskClient.EXPECT().GetAgent(gomock.Any(), taskagent.GetAgentArgs{
		PoolId:  types.ToPtr(1),
		AgentId: types.ToPtr(42),
	}).Return(&taskagent.TaskAgent{
		Id:      types.ToPtr(42),
		Name:    types.ToPtr("my-agent"),
		Version: types.ToPtr("4.0.0"),
		Enabled: types.ToPtr(true),
		Status:  &taskagent.TaskAgentStatusValues.Online,
	}, nil)

	cmd := NewCmd(cmdCtx)
	cmd.SetArgs([]string{"myorg/1/42"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)

	out := outBuf.String()
	assert.Contains(t, out, "my-agent")
	assert.Contains(t, out, "42")
	assert.Contains(t, out, "4.0.0")
	assert.Contains(t, out, "online")
}

func TestShowCmd_JSONOutput(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, outBuf, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(taskClient, nil)

	taskClient.EXPECT().GetAgent(gomock.Any(), taskagent.GetAgentArgs{
		PoolId:  types.ToPtr(1),
		AgentId: types.ToPtr(42),
	}).Return(&taskagent.TaskAgent{
		Id:      types.ToPtr(42),
		Name:    types.ToPtr("my-agent"),
		Version: types.ToPtr("4.0.0"),
		Enabled: types.ToPtr(true),
		Status:  &taskagent.TaskAgentStatusValues.Online,
	}, nil)

	cmd := NewCmd(cmdCtx)
	cmd.SetArgs([]string{"myorg/1/42", "--json"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)

	out := outBuf.String()
	assert.Contains(t, out, `"id":42`)
	assert.Contains(t, out, `"name":"my-agent"`)
	assert.Contains(t, out, `"version":"4.0.0"`)
}

func TestShowCmd_MissingArg(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()

	cmd := NewCmd(cmdCtx)
	cmd.SetArgs([]string{})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent target is required")
}

func TestShowCmd_GetAgentError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "myorg").Return(taskClient, nil)

	taskClient.EXPECT().GetAgent(gomock.Any(), gomock.Any()).Return(nil, errors.New("API failure"))

	cmd := NewCmd(cmdCtx)
	cmd.SetArgs([]string{"myorg/1/42"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get agent")
	assert.Contains(t, err.Error(), "API failure")
}

func TestShowCmd_NoDefaultOrganization(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	mockConfig := mocks.NewMockConfig(ctrl)
	mockAuth := mocks.NewMockAuthConfig(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Config().Return(mockConfig, nil)
	mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
	mockAuth.EXPECT().GetDefaultOrganization().Return("", nil)

	cmd := NewCmd(cmdCtx)
	cmd.SetArgs([]string{"1/42"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	var flagErr *util.FlagError
	assert.True(t, errors.As(err, &flagErr))
	assert.Contains(t, err.Error(), "no organization specified")
}

func TestShowCmd_TooManyArgs(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()

	cmd := NewCmd(cmdCtx)
	cmd.SetArgs([]string{"myorg/1/42", "extra"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many arguments")
}
