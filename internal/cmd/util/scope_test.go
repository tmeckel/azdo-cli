package util_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	util "github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestParseScope(t *testing.T) {
	t.Run("explicit organization only", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)

		scope, err := util.ParseScope(mockCtx, "myorg")
		require.NoError(t, err)
		assert.Equal(t, "myorg", scope.Organization)
		assert.Empty(t, scope.Project)
	})

	t.Run("explicit organization and project", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)

		scope, err := util.ParseScope(mockCtx, "myorg/myproject")
		require.NoError(t, err)
		assert.Equal(t, "myorg", scope.Organization)
		assert.Equal(t, "myproject", scope.Project)
	})

	t.Run("default organization from config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)
		mockConfig := mocks.NewMockConfig(ctrl)
		mockAuth := mocks.NewMockAuthConfig(ctrl)

		mockCtx.EXPECT().Config().Return(mockConfig, nil)
		mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
		mockAuth.EXPECT().GetDefaultOrganization().Return("default-org", nil)

		scope, err := util.ParseScope(mockCtx, "")
		require.NoError(t, err)
		assert.Equal(t, "default-org", scope.Organization)
		assert.Empty(t, scope.Project)
	})

	t.Run("invalid scope format", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)

		_, err := util.ParseScope(mockCtx, "org/")
		require.Error(t, err)
	})
}

func TestParseOrganizationArg(t *testing.T) {
	t.Run("explicit organization", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)

		org, err := util.ParseOrganizationArg(mockCtx, "myorg")
		require.NoError(t, err)
		assert.Equal(t, "myorg", org)
	})

	t.Run("default organization from config", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)
		mockConfig := mocks.NewMockConfig(ctrl)
		mockAuth := mocks.NewMockAuthConfig(ctrl)

		mockCtx.EXPECT().Config().Return(mockConfig, nil)
		mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
		mockAuth.EXPECT().GetDefaultOrganization().Return("default-org", nil)

		org, err := util.ParseOrganizationArg(mockCtx, "")
		require.NoError(t, err)
		assert.Equal(t, "default-org", org)
	})

	t.Run("project segment not allowed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)

		_, err := util.ParseOrganizationArg(mockCtx, "org/project")
		require.Error(t, err)
	})
}

func TestParseProjectScope(t *testing.T) {
	t.Run("explicit organization and project", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		scope, err := util.ParseProjectScope(mocks.NewMockCmdContext(ctrl), "org/project")
		require.NoError(t, err)
		assert.Equal(t, "org", scope.Organization)
		assert.Equal(t, "project", scope.Project)
	})

	t.Run("default organization for project only input", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)
		mockConfig := mocks.NewMockConfig(ctrl)
		mockAuth := mocks.NewMockAuthConfig(ctrl)

		mockCtx.EXPECT().Config().Return(mockConfig, nil)
		mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
		mockAuth.EXPECT().GetDefaultOrganization().Return("default-org", nil)

		scope, err := util.ParseProjectScope(mockCtx, "project")
		require.NoError(t, err)
		assert.Equal(t, "default-org", scope.Organization)
		assert.Equal(t, "project", scope.Project)
	})

	t.Run("invalid project argument", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		_, err := util.ParseProjectScope(mocks.NewMockCmdContext(ctrl), "")
		require.Error(t, err)
	})
}

func TestParseTarget(t *testing.T) {
	t.Run("organization and group", func(t *testing.T) {
		result, err := util.ParseTarget("org/group")
		require.NoError(t, err)
		assert.Equal(t, "org", result.Organization)
		assert.Empty(t, result.Project)
		assert.Equal(t, "group", result.Target)
	})

	t.Run("organization project group", func(t *testing.T) {
		result, err := util.ParseTarget("org/project/group")
		require.NoError(t, err)
		assert.Equal(t, "org", result.Organization)
		assert.Equal(t, "project", result.Project)
		assert.Equal(t, "group", result.Target)
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := util.ParseTarget("justone")
		require.Error(t, err)
	})
}

func TestParseTargetWithDefaultOrganization(t *testing.T) {
	t.Run("implicit organization", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)
		mockConfig := mocks.NewMockConfig(ctrl)
		mockAuth := mocks.NewMockAuthConfig(ctrl)

		mockCtx.EXPECT().Config().Return(mockConfig, nil)
		mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
		mockAuth.EXPECT().GetDefaultOrganization().Return("default-org", nil)

		result, err := util.ParseTargetWithDefaultOrganization(mockCtx, "group")
		require.NoError(t, err)
		assert.Equal(t, "default-org", result.Organization)
		assert.Equal(t, "group", result.Target)
	})

	t.Run("explicit organization", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		result, err := util.ParseTargetWithDefaultOrganization(mocks.NewMockCmdContext(ctrl), "org/group")
		require.NoError(t, err)
		assert.Equal(t, "org", result.Organization)
		assert.Equal(t, "group", result.Target)
	})

	t.Run("organization project group", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		result, err := util.ParseTargetWithDefaultOrganization(mocks.NewMockCmdContext(ctrl), "org/project/group")
		require.NoError(t, err)
		assert.Equal(t, "org", result.Organization)
		assert.Equal(t, "project", result.Project)
		assert.Equal(t, "group", result.Target)
	})

	t.Run("missing default organization", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)
		mockConfig := mocks.NewMockConfig(ctrl)
		mockAuth := mocks.NewMockAuthConfig(ctrl)

		mockCtx.EXPECT().Config().Return(mockConfig, nil)
		mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
		mockAuth.EXPECT().GetDefaultOrganization().Return("", nil)

		_, err := util.ParseTargetWithDefaultOrganization(mockCtx, "group")
		require.Error(t, err)
	})

	t.Run("invalid format", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		_, err := util.ParseTargetWithDefaultOrganization(mocks.NewMockCmdContext(ctrl), "")
		require.Error(t, err)
	})
}

func TestParseProjectTargetWithDefaultOrganization(t *testing.T) {
	t.Run("implicit organization", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)
		mockConfig := mocks.NewMockConfig(ctrl)
		mockAuth := mocks.NewMockAuthConfig(ctrl)

		mockCtx.EXPECT().Config().Return(mockConfig, nil)
		mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
		mockAuth.EXPECT().GetDefaultOrganization().Return("default-org", nil)

		result, err := util.ParseProjectTargetWithDefaultOrganization(mockCtx, "project/target")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "default-org", result.Organization)
		assert.Equal(t, "project", result.Project)
		assert.Equal(t, "target", result.Target)
	})

	t.Run("explicit organization", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		result, err := util.ParseProjectTargetWithDefaultOrganization(mocks.NewMockCmdContext(ctrl), "org/project/target")
		require.NoError(t, err)
		assert.Equal(t, "org", result.Organization)
		assert.Equal(t, "project", result.Project)
		assert.Equal(t, "target", result.Target)
	})

	t.Run("missing default organization", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockCtx := mocks.NewMockCmdContext(ctrl)
		mockConfig := mocks.NewMockConfig(ctrl)
		mockAuth := mocks.NewMockAuthConfig(ctrl)

		mockCtx.EXPECT().Config().Return(mockConfig, nil)
		mockConfig.EXPECT().Authentication().Return(mockAuth).AnyTimes()
		mockAuth.EXPECT().GetDefaultOrganization().Return("", nil)

		_, err := util.ParseProjectTargetWithDefaultOrganization(mockCtx, "project/target")
		require.Error(t, err)
	})

	t.Run("missing project segment", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		_, err := util.ParseProjectTargetWithDefaultOrganization(mocks.NewMockCmdContext(ctrl), "justtarget")
		require.Error(t, err)
	})

	t.Run("invalid format", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		_, err := util.ParseProjectTargetWithDefaultOrganization(mocks.NewMockCmdContext(ctrl), "")
		require.Error(t, err)
	})
}

func TestResolveScopeDescriptor_NoProject(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)

	descriptor, projectID, err := util.ResolveScopeDescriptor(mockCtx, "org", "")
	require.NoError(t, err)
	assert.Nil(t, descriptor)
	assert.Nil(t, projectID)
}

func TestResolveScopeDescriptor_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)
	mockCoreClient := mocks.NewMockCoreClient(ctrl)
	mockGraphClient := mocks.NewMockGraphClient(ctrl)

	mockCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
	mockCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	projectID := uuid.New()
	projectRef := &core.TeamProject{
		Id: types.ToPtr(projectID),
	}
	mockClientFactory.EXPECT().
		Core(gomock.Any(), "org").
		Return(mockCoreClient, nil)
	mockCoreClient.EXPECT().
		GetProject(gomock.Any(), gomock.AssignableToTypeOf(core.GetProjectArgs{})).
		Return(projectRef, nil)

	descriptorValue := "vssgp.Descriptor"
	mockClientFactory.EXPECT().
		Graph(gomock.Any(), "org").
		Return(mockGraphClient, nil)
	mockGraphClient.EXPECT().
		GetDescriptor(gomock.Any(), gomock.AssignableToTypeOf(graph.GetDescriptorArgs{})).
		Return(&graph.GraphDescriptorResult{Value: &descriptorValue}, nil)

	descriptor, projectIDPtr, err := util.ResolveScopeDescriptor(mockCtx, "org", "project")
	require.NoError(t, err)
	require.NotNil(t, descriptor)
	assert.Equal(t, descriptorValue, *descriptor)
	require.NotNil(t, projectIDPtr)
	assert.Equal(t, projectID.String(), *projectIDPtr)
}

func TestResolveScopeDescriptor_CoreClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCtx := mocks.NewMockCmdContext(ctrl)
	mockClientFactory := mocks.NewMockClientFactory(ctrl)

	mockCtx.EXPECT().ClientFactory().Return(mockClientFactory)
	mockCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mockClientFactory.EXPECT().
		Core(gomock.Any(), "org").
		Return(nil, errors.New("boom"))

	descriptor, projectID, err := util.ResolveScopeDescriptor(mockCtx, "org", "project")
	require.Error(t, err)
	assert.Nil(t, descriptor)
	assert.Nil(t, projectID)
}
