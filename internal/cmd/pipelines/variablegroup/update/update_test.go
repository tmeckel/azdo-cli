package update_test

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelinepermissions"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	updatecmd "github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/update"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestUpdateCmd_SuccessCases(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		setupMocks func(t *testing.T, taskClient *mocks.MockTaskAgentClient, permClient *mocks.MockPipelinePermissionsClient, clientFactory *mocks.MockClientFactory)
		wantOut    string
		wantErrOut string
	}{
		{
			name: "updates name and description",
			args: []string{"org/project/123", "--name", "new name", "--description", "new desc"},
			setupMocks: func(t *testing.T, taskClient *mocks.MockTaskAgentClient, permClient *mocks.MockPipelinePermissionsClient, clientFactory *mocks.MockClientFactory) {
				clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
				taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, args taskagent.GetVariableGroupsByIdArgs) (*[]taskagent.VariableGroup, error) {
						require.NotNil(t, args.Project)
						assert.Equal(t, "project", *args.Project)
						require.NotNil(t, args.GroupIds)
						require.Len(t, *args.GroupIds, 1)
						assert.Equal(t, 123, (*args.GroupIds)[0])
						return &[]taskagent.VariableGroup{{
							Id:          types.ToPtr(123),
							Name:        types.ToPtr("old name"),
							Description: types.ToPtr("old desc"),
						}}, nil
					},
				)
				taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, args taskagent.UpdateVariableGroupArgs) (*taskagent.VariableGroup, error) {
						require.NotNil(t, args.GroupId)
						assert.Equal(t, 123, *args.GroupId)
						require.NotNil(t, args.VariableGroupParameters)
						assert.Equal(t, "new name", types.GetValue(args.VariableGroupParameters.Name, ""))
						assert.Equal(t, "new desc", types.GetValue(args.VariableGroupParameters.Description, ""))
						return &taskagent.VariableGroup{
							Id:          types.ToPtr(123),
							Name:        types.ToPtr("new name"),
							Description: types.ToPtr("new desc"),
						}, nil
					},
				)

				clientFactory.EXPECT().PipelinePermissions(gomock.Any(), "org").Return(permClient, nil)
				permClient.EXPECT().GetPipelinePermissionsForResource(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, args pipelinepermissions.GetPipelinePermissionsForResourceArgs) (*pipelinepermissions.ResourcePipelinePermissions, error) {
						require.NotNil(t, args.Project)
						assert.Equal(t, "project", *args.Project)
						require.NotNil(t, args.ResourceType)
						assert.Equal(t, "variablegroup", *args.ResourceType)
						require.NotNil(t, args.ResourceId)
						assert.Equal(t, "123", *args.ResourceId)
						return nil, nil
					},
				)
			},
			wantOut:    "Updated variable group \"new name\" (id: 123)\n",
			wantErrOut: "",
		},
		{
			name: "authorize only",
			args: []string{"org/project/123", "--authorize"},
			setupMocks: func(t *testing.T, taskClient *mocks.MockTaskAgentClient, permClient *mocks.MockPipelinePermissionsClient, clientFactory *mocks.MockClientFactory) {
				clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
				taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
					&[]taskagent.VariableGroup{{
						Id:   types.ToPtr(123),
						Name: types.ToPtr("group-one"),
					}}, nil,
				)
				taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)

				clientFactory.EXPECT().PipelinePermissions(gomock.Any(), "org").Return(permClient, nil)
				permClient.EXPECT().UpdatePipelinePermisionsForResource(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, args pipelinepermissions.UpdatePipelinePermisionsForResourceArgs) (*pipelinepermissions.ResourcePipelinePermissions, error) {
						require.NotNil(t, args.ResourceAuthorization)
						require.NotNil(t, args.ResourceAuthorization.AllPipelines)
						assert.Equal(t, true, types.GetValue(args.ResourceAuthorization.AllPipelines.Authorized, false))
						require.NotNil(t, args.ResourceId)
						assert.Equal(t, "123", *args.ResourceId)
						require.NotNil(t, args.Project)
						assert.Equal(t, "project", *args.Project)
						return &pipelinepermissions.ResourcePipelinePermissions{
							AllPipelines: &pipelinepermissions.Permission{Authorized: types.ToPtr(true)},
						}, nil
					},
				)
			},
			wantOut: "Updated variable group \"group-one\" (id: 123)\n" +
				"Authorize for all pipelines: true\n",
			wantErrOut: "",
		},
		{
			name: "clears project references",
			args: []string{"org/project/123", "--clear-project-references"},
			setupMocks: func(t *testing.T, taskClient *mocks.MockTaskAgentClient, permClient *mocks.MockPipelinePermissionsClient, clientFactory *mocks.MockClientFactory) {
				clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
				taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
					&[]taskagent.VariableGroup{{
						Id:   types.ToPtr(123),
						Name: types.ToPtr("group-two"),
					}}, nil,
				)
				taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, args taskagent.UpdateVariableGroupArgs) (*taskagent.VariableGroup, error) {
						require.NotNil(t, args.VariableGroupParameters)
						require.NotNil(t, args.VariableGroupParameters.VariableGroupProjectReferences)
						assert.Len(t, *args.VariableGroupParameters.VariableGroupProjectReferences, 0)
						return &taskagent.VariableGroup{
							Id:   types.ToPtr(123),
							Name: types.ToPtr("group-two"),
						}, nil
					},
				)

				clientFactory.EXPECT().PipelinePermissions(gomock.Any(), "org").Return(permClient, nil)
				permClient.EXPECT().GetPipelinePermissionsForResource(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantOut:    "Updated variable group \"group-two\" (id: 123)\n",
			wantErrOut: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			io, _, outBuf, errBuf := iostreams.Test()

			cmdCtx := mocks.NewMockCmdContext(ctrl)
			clientFactory := mocks.NewMockClientFactory(ctrl)
			taskClient := mocks.NewMockTaskAgentClient(ctrl)
			permClient := mocks.NewMockPipelinePermissionsClient(ctrl)

			cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
			cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
			cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()

			tt.setupMocks(t, taskClient, permClient, clientFactory)

			cmd := updatecmd.NewCmd(cmdCtx)
			cmd.SetArgs(tt.args)

			_, err := cmd.ExecuteC()
			require.NoError(t, err)
			assert.Equal(t, tt.wantOut, outBuf.String())
			assert.Equal(t, tt.wantErrOut, errBuf.String())
		})
	}
}

func TestUpdateCmd_ValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantErr    string
		setupMocks func(t *testing.T, cmdCtx *mocks.MockCmdContext, clientFactory *mocks.MockClientFactory, taskClient *mocks.MockTaskAgentClient)
	}{
		{
			name:    "provider data flags mutually exclusive",
			args:    []string{"org/project/123", "--provider-data-json", "{}", "--clear-provider-data"},
			wantErr: "--provider-data-json and --clear-provider-data are mutually exclusive",
		},
		{
			name:    "requires mutating flag",
			args:    []string{"org/project/123"},
			wantErr: "at least one mutating flag must be supplied",
		},
		{
			name:    "invalid provider data json",
			args:    []string{"org/project/123", "--provider-data-json", "not-json"},
			wantErr: "invalid provider-data-json",
			setupMocks: func(t *testing.T, cmdCtx *mocks.MockCmdContext, clientFactory *mocks.MockClientFactory, taskClient *mocks.MockTaskAgentClient) {
				cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
				clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
				taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(&[]taskagent.VariableGroup{{
					Id:   types.ToPtr(123),
					Name: types.ToPtr("group-one"),
				}}, nil)
				taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			io, _, outBuf, errBuf := iostreams.Test()

			cmdCtx := mocks.NewMockCmdContext(ctrl)
			clientFactory := mocks.NewMockClientFactory(ctrl)
			taskClient := mocks.NewMockTaskAgentClient(ctrl)
			cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
			cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
			if tt.setupMocks != nil {
				tt.setupMocks(t, cmdCtx, clientFactory, taskClient)
			}

			cmd := updatecmd.NewCmd(cmdCtx)
			cmd.SetArgs(tt.args)

			_, err := cmd.ExecuteC()
			require.Error(t, err)
			var flagErr *util.FlagError
			assert.True(t, errors.As(err, &flagErr))
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Equal(t, "", outBuf.String())
			// errBuf may contain Cobra usage; ensure it is not empty only when usage printed
			_ = errBuf
		})
	}
}

func TestUpdateCmd_VariableGroupNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, outBuf, errBuf := iostreams.Test()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()

	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(&[]taskagent.VariableGroup{}, nil)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "new"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Equal(t, "", outBuf.String())
	_ = errBuf
}

func TestUpdateCmd_AuthorizeFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, outBuf, errBuf := iostreams.Test()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)
	permClient := mocks.NewMockPipelinePermissionsClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()

	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:   types.ToPtr(123),
			Name: types.ToPtr("group-one"),
		}}, nil,
	)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)

	clientFactory.EXPECT().PipelinePermissions(gomock.Any(), "org").Return(permClient, nil)
	permClient.EXPECT().UpdatePipelinePermisionsForResource(gomock.Any(), gomock.Any()).Return(nil, errors.New("perm failure"))

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--authorize"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "perm failure")
	assert.Equal(t, "", outBuf.String())
	_ = errBuf
}
