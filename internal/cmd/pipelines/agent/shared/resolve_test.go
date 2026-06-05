package shared

import (
	"context"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func newCtrl(t *testing.T) *gomock.Controller {
	t.Helper()
	c := gomock.NewController(t)
	t.Cleanup(c.Finish)
	return c
}

func TestResolvePoolAgent_NumericPoolNumericAgent(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgent(gomock.Any(), taskagent.GetAgentArgs{
		PoolId:  types.ToPtr(1),
		AgentId: types.ToPtr(42),
	}).Return(&taskagent.TaskAgent{
		Id:      types.ToPtr(42),
		Name:    types.ToPtr("my-agent"),
		Version: types.ToPtr("4.0.0"),
		Enabled: types.ToPtr(true),
	}, nil)

	agent, err := ResolvePoolAgent(ctx, client, "org", "1", "42")
	require.NoError(t, err)
	require.NotNil(t, agent)
	assert.Equal(t, 42, *agent.Id)
	assert.Equal(t, "my-agent", *agent.Name)
}

func TestResolvePoolAgent_NamePoolNameAgent(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgentPools(gomock.Any(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr("Default"),
	}).Return(&[]taskagent.TaskAgentPool{
		{Id: types.ToPtr(1), Name: types.ToPtr("Default")},
	}, nil)

	client.EXPECT().GetAgents(gomock.Any(), taskagent.GetAgentsArgs{
		PoolId:    types.ToPtr(1),
		AgentName: types.ToPtr("my-agent"),
	}).Return(&[]taskagent.TaskAgent{
		{Id: types.ToPtr(42), Name: types.ToPtr("my-agent")},
	}, nil)

	client.EXPECT().GetAgent(gomock.Any(), taskagent.GetAgentArgs{
		PoolId:  types.ToPtr(1),
		AgentId: types.ToPtr(42),
	}).Return(&taskagent.TaskAgent{
		Id:      types.ToPtr(42),
		Name:    types.ToPtr("my-agent"),
		Version: types.ToPtr("4.0.0"),
		Enabled: types.ToPtr(true),
	}, nil)

	agent, err := ResolvePoolAgent(ctx, client, "org", "Default", "my-agent")
	require.NoError(t, err)
	require.NotNil(t, agent)
	assert.Equal(t, 42, *agent.Id)
	assert.Equal(t, "my-agent", *agent.Name)
}

func TestResolvePoolAgent_MixedNumericName(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgents(gomock.Any(), taskagent.GetAgentsArgs{
		PoolId:    types.ToPtr(1),
		AgentName: types.ToPtr("my-agent"),
	}).Return(&[]taskagent.TaskAgent{
		{Id: types.ToPtr(42), Name: types.ToPtr("my-agent")},
	}, nil)

	client.EXPECT().GetAgent(gomock.Any(), taskagent.GetAgentArgs{
		PoolId:  types.ToPtr(1),
		AgentId: types.ToPtr(42),
	}).Return(&taskagent.TaskAgent{
		Id:      types.ToPtr(42),
		Name:    types.ToPtr("my-agent"),
		Version: types.ToPtr("4.0.0"),
		Enabled: types.ToPtr(true),
	}, nil)

	agent, err := ResolvePoolAgent(ctx, client, "org", "1", "my-agent")
	require.NoError(t, err)
	require.NotNil(t, agent)
	assert.Equal(t, 42, *agent.Id)
}

func TestResolvePoolAgent_PoolNotFound(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgentPools(gomock.Any(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr("Nonexistent"),
	}).Return(&[]taskagent.TaskAgentPool{}, nil)

	_, err := ResolvePoolAgent(ctx, client, "org", "Nonexistent", "42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolvePoolAgent_AmbiguousPool(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgentPools(gomock.Any(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr("Default"),
	}).Return(&[]taskagent.TaskAgentPool{
		{Id: types.ToPtr(1), Name: types.ToPtr("Default")},
		{Id: types.ToPtr(2), Name: types.ToPtr("Default")},
	}, nil)

	_, err := ResolvePoolAgent(ctx, client, "org", "Default", "42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple pools")
}

func TestResolvePoolAgent_AgentNotFound(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgentPools(gomock.Any(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr("Default"),
	}).Return(&[]taskagent.TaskAgentPool{
		{Id: types.ToPtr(1), Name: types.ToPtr("Default")},
	}, nil)

	client.EXPECT().GetAgents(gomock.Any(), taskagent.GetAgentsArgs{
		PoolId:    types.ToPtr(1),
		AgentName: types.ToPtr("ghost"),
	}).Return(&[]taskagent.TaskAgent{}, nil)

	_, err := ResolvePoolAgent(ctx, client, "org", "Default", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolvePoolAgent_AmbiguousAgent(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgentPools(gomock.Any(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr("Default"),
	}).Return(&[]taskagent.TaskAgentPool{
		{Id: types.ToPtr(1), Name: types.ToPtr("Default")},
	}, nil)

	client.EXPECT().GetAgents(gomock.Any(), taskagent.GetAgentsArgs{
		PoolId:    types.ToPtr(1),
		AgentName: types.ToPtr("dup"),
	}).Return(&[]taskagent.TaskAgent{
		{Id: types.ToPtr(10), Name: types.ToPtr("dup")},
		{Id: types.ToPtr(11), Name: types.ToPtr("dup")},
	}, nil)

	_, err := ResolvePoolAgent(ctx, client, "org", "Default", "dup")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple agents")
}

func TestResolvePool_Numeric(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	id, err := ResolvePool(ctx, client, "42")
	require.NoError(t, err)
	assert.Equal(t, 42, id)
}

func TestResolvePool_NegativeNumeric(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)

	_, err := ResolvePool(ctx, client, "-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pool id")
}

func TestResolvePool_ByName(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgentPools(gomock.Any(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr("Default"),
	}).Return(&[]taskagent.TaskAgentPool{
		{Id: types.ToPtr(1), Name: types.ToPtr("Default")},
	}, nil)

	id, err := ResolvePool(ctx, client, "Default")
	require.NoError(t, err)
	assert.Equal(t, 1, id)
}

func TestResolvePool_NotFound(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgentPools(gomock.Any(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr("Ghost"),
	}).Return(&[]taskagent.TaskAgentPool{}, nil)

	_, err := ResolvePool(ctx, client, "Ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolvePool_Ambiguous(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgentPools(gomock.Any(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr("Default"),
	}).Return(&[]taskagent.TaskAgentPool{
		{Id: types.ToPtr(1), Name: types.ToPtr("Default")},
		{Id: types.ToPtr(2), Name: types.ToPtr("Default")},
	}, nil)

	_, err := ResolvePool(ctx, client, "Default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple pools")
}

func TestResolveAgent_Numeric(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)

	id, err := ResolveAgent(ctx, client, 1, "42")
	require.NoError(t, err)
	assert.Equal(t, 42, id)
}

func TestResolveAgent_NegativeNumeric(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)

	_, err := ResolveAgent(ctx, client, 1, "-5")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid agent id")
}

func TestResolveAgent_ByName(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgents(gomock.Any(), taskagent.GetAgentsArgs{
		PoolId:    types.ToPtr(1),
		AgentName: types.ToPtr("my-agent"),
	}).Return(&[]taskagent.TaskAgent{
		{Id: types.ToPtr(42), Name: types.ToPtr("my-agent")},
	}, nil)

	id, err := ResolveAgent(ctx, client, 1, "my-agent")
	require.NoError(t, err)
	assert.Equal(t, 42, id)
}

func TestResolveAgent_NotFound(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgents(gomock.Any(), taskagent.GetAgentsArgs{
		PoolId:    types.ToPtr(1),
		AgentName: types.ToPtr("ghost"),
	}).Return(&[]taskagent.TaskAgent{}, nil)

	_, err := ResolveAgent(ctx, client, 1, "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveAgent_Ambiguous(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgents(gomock.Any(), taskagent.GetAgentsArgs{
		PoolId:    types.ToPtr(1),
		AgentName: types.ToPtr("dup"),
	}).Return(&[]taskagent.TaskAgent{
		{Id: types.ToPtr(10), Name: types.ToPtr("dup")},
		{Id: types.ToPtr(11), Name: types.ToPtr("dup")},
	}, nil)

	_, err := ResolveAgent(ctx, client, 1, "dup")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple agents")
}

func TestResolvePoolAgent_NegativePoolID(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	_, err := ResolvePoolAgent(ctx, client, "org", "-1", "42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pool id")
}

func TestResolvePoolAgent_NegativeAgentID(t *testing.T) {
	t.Parallel()
	ctrl := newCtrl(t)
	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockTaskAgentClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).AnyTimes()

	client.EXPECT().GetAgentPools(gomock.Any(), taskagent.GetAgentPoolsArgs{
		PoolName: types.ToPtr("Default"),
	}).Return(&[]taskagent.TaskAgentPool{
		{Id: types.ToPtr(1), Name: types.ToPtr("Default")},
	}, nil)

	_, err := ResolvePoolAgent(ctx, client, "org", "Default", "-5")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid agent id")
}
