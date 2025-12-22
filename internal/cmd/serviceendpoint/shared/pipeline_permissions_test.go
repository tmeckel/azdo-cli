package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelinepermissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/mocks"
)

func TestSetAllPipelinesAccessToEndpoint_ValidationErrors(t *testing.T) {
	t.Parallel()

	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	endpointID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	tests := []struct {
		name       string
		nilCmdCtx  bool
		org        string
		projectID  uuid.UUID
		endpointID uuid.UUID
		wantErr    string
	}{
		{
			name:       "nil command context",
			nilCmdCtx:  true,
			org:        "fabrikam",
			projectID:  projectID,
			endpointID: endpointID,
			wantErr:    "nil command context",
		},
		{
			name:       "organization required",
			org:        "",
			projectID:  projectID,
			endpointID: endpointID,
			wantErr:    "organization is required",
		},
		{
			name:       "project ID required",
			org:        "fabrikam",
			projectID:  uuid.Nil,
			endpointID: endpointID,
			wantErr:    "project ID is required",
		},
		{
			name:       "endpoint ID required",
			org:        "fabrikam",
			projectID:  projectID,
			endpointID: uuid.Nil,
			wantErr:    "endpoint ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.nilCmdCtx {
				err := SetAllPipelinesAccessToEndpoint(nil, tt.org, tt.projectID, tt.endpointID, true, nil)
				require.EqualError(t, err, tt.wantErr)
				return
			}

			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)
			cmdCtx := mocks.NewMockCmdContext(ctrl)

			err := SetAllPipelinesAccessToEndpoint(cmdCtx, tt.org, tt.projectID, tt.endpointID, true, nil)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestSetAllPipelinesAccessToEndpoint_UsesPipelinePermissionsClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authorized bool
	}{
		{name: "authorize", authorized: true},
		{name: "unauthorize", authorized: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			ctx := context.Background()
			organization := "fabrikam"
			projectID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
			endpointID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

			cmdCtx := mocks.NewMockCmdContext(ctrl)
			clientFactory := mocks.NewMockClientFactory(ctrl)
			permissionsClient := mocks.NewMockPipelinePermissionsClient(ctrl)

			cmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
			cmdCtx.EXPECT().ClientFactory().Return(clientFactory).Times(1)
			clientFactory.EXPECT().PipelinePermissions(ctx, organization).Return(permissionsClient, nil).Times(1)

			permissionsClient.EXPECT().
				UpdatePipelinePermisionsForResource(ctx, gomock.Any()).
				DoAndReturn(func(_ context.Context, args pipelinepermissions.UpdatePipelinePermisionsForResourceArgs) (*pipelinepermissions.ResourcePipelinePermissions, error) {
					require.NotNil(t, args.Project)
					require.Equal(t, projectID.String(), *args.Project)

					require.NotNil(t, args.ResourceType)
					require.Equal(t, EndpointResourceType, *args.ResourceType)

					require.NotNil(t, args.ResourceId)
					require.Equal(t, endpointID.String(), *args.ResourceId)

					require.NotNil(t, args.ResourceAuthorization)
					require.NotNil(t, args.ResourceAuthorization.AllPipelines)
					require.NotNil(t, args.ResourceAuthorization.AllPipelines.Authorized)
					assert.Equal(t, tt.authorized, *args.ResourceAuthorization.AllPipelines.Authorized)

					return &pipelinepermissions.ResourcePipelinePermissions{}, nil
				}).
				Times(1)

			cleanupCalled := false
			err := SetAllPipelinesAccessToEndpoint(cmdCtx, organization, projectID, endpointID, tt.authorized, func() error {
				cleanupCalled = true
				return nil
			})
			require.NoError(t, err)
			assert.False(t, cleanupCalled)
		})
	}
}

func TestSetAllPipelinesAccessToEndpoint_CallsCleanupOnClientInitError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ctx := context.Background()
	organization := "fabrikam"
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	endpointID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)

	cmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).Times(1)
	clientFactory.EXPECT().PipelinePermissions(ctx, organization).Return(nil, errors.New("boom")).Times(1)

	cleanupCalled := false
	err := SetAllPipelinesAccessToEndpoint(cmdCtx, organization, projectID, endpointID, true, func() error {
		cleanupCalled = true
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize pipeline permissions client")
	assert.Contains(t, err.Error(), "boom")
	assert.True(t, cleanupCalled)
}

func TestSetAllPipelinesAccessToEndpoint_CallsCleanupOnUpdateError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ctx := context.Background()
	organization := "fabrikam"
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	endpointID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	authorized := true

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	permissionsClient := mocks.NewMockPipelinePermissionsClient(ctrl)

	cmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).Times(1)
	clientFactory.EXPECT().PipelinePermissions(ctx, organization).Return(permissionsClient, nil).Times(1)
	permissionsClient.EXPECT().
		UpdatePipelinePermisionsForResource(ctx, gomock.Any()).
		Return(nil, errors.New("boom")).
		Times(1)

	cleanupCalled := false
	err := SetAllPipelinesAccessToEndpoint(cmdCtx, organization, projectID, endpointID, authorized, func() error {
		cleanupCalled = true
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set authorization for endpoint")
	assert.Contains(t, err.Error(), endpointID.String())
	assert.Contains(t, err.Error(), "authorized=true")
	assert.Contains(t, err.Error(), "boom")
	assert.True(t, cleanupCalled)
}

func TestSetAllPipelinesAccessToEndpoint_ReturnsCleanupErrorWhenCleanupFails(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ctx := context.Background()
	organization := "fabrikam"
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	endpointID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)

	cmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).Times(1)
	clientFactory.EXPECT().PipelinePermissions(ctx, organization).Return(nil, errors.New("boom")).Times(1)

	err := SetAllPipelinesAccessToEndpoint(cmdCtx, organization, projectID, endpointID, true, func() error {
		return errors.New("cleanup blew up")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to initialize pipeline permissions client")
	assert.Contains(t, err.Error(), "boom")
	assert.Contains(t, err.Error(), "cleanup failed: cleanup blew up")
}
