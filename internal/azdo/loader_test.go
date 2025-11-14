package azdo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestGetProjects(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockClient := mocks.NewMockCoreClient(ctrl)
	ctx := context.Background()

	t.Run("success - single page", func(t *testing.T) {
		// Arrange
		args := core.GetProjectsArgs{}
		expectedProjects := []core.TeamProjectReference{
			{Name: types.ToPtr("Project1")},
			{Name: types.ToPtr("Project2")},
		}
		response := &core.GetProjectsResponseValue{
			Value: expectedProjects,
		}
		mockClient.EXPECT().GetProjects(ctx, args).Return(response, nil)

		// Act
		projects, err := azdo.GetProjects(ctx, mockClient, args)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, expectedProjects, projects)
	})

	t.Run("success - multiple pages", func(t *testing.T) {
		// Arrange
		argsPage1 := core.GetProjectsArgs{}
		page1Projects := []core.TeamProjectReference{{Name: types.ToPtr("Project1")}}
		response1 := &core.GetProjectsResponseValue{
			Value:             page1Projects,
			ContinuationToken: "1",
		}
		mockClient.EXPECT().GetProjects(ctx, argsPage1).Return(response1, nil)

		argsPage2 := core.GetProjectsArgs{ContinuationToken: types.ToPtr(1)}
		page2Projects := []core.TeamProjectReference{{Name: types.ToPtr("Project2")}}
		response2 := &core.GetProjectsResponseValue{
			Value: page2Projects,
		}
		mockClient.EXPECT().GetProjects(ctx, argsPage2).Return(response2, nil)

		// Act
		projects, err := azdo.GetProjects(ctx, mockClient, argsPage1)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, append(page1Projects, page2Projects...), projects)
	})

	t.Run("success - no projects", func(t *testing.T) {
		// Arrange
		args := core.GetProjectsArgs{}
		response := &core.GetProjectsResponseValue{
			Value: []core.TeamProjectReference{},
		}
		mockClient.EXPECT().GetProjects(ctx, args).Return(response, nil)

		// Act
		projects, err := azdo.GetProjects(ctx, mockClient, args)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, projects)
	})

	t.Run("success - nil response", func(t *testing.T) {
		// Arrange
		args := core.GetProjectsArgs{}
		mockClient.EXPECT().GetProjects(ctx, args).Return(nil, nil)

		// Act
		projects, err := azdo.GetProjects(ctx, mockClient, args)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, projects)
	})

	t.Run("success - nil value in response", func(t *testing.T) {
		// Arrange
		args := core.GetProjectsArgs{}
		response := &core.GetProjectsResponseValue{
			Value: nil,
		}
		mockClient.EXPECT().GetProjects(ctx, args).Return(response, nil)

		// Act
		projects, err := azdo.GetProjects(ctx, mockClient, args)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, projects)
	})

	t.Run("negative - api error on first page", func(t *testing.T) {
		// Arrange
		args := core.GetProjectsArgs{}
		expectedErr := errors.New("api error")
		mockClient.EXPECT().GetProjects(ctx, args).Return(nil, expectedErr)

		// Act
		projects, err := azdo.GetProjects(ctx, mockClient, args)

		// Assert
		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Empty(t, projects)
	})

	t.Run("negative - api error on second page", func(t *testing.T) {
		// Arrange
		argsPage1 := core.GetProjectsArgs{}
		response1 := &core.GetProjectsResponseValue{
			Value:             []core.TeamProjectReference{{Name: types.ToPtr("Project1")}},
			ContinuationToken: "1",
		}
		mockClient.EXPECT().GetProjects(ctx, argsPage1).Return(response1, nil)

		argsPage2 := core.GetProjectsArgs{ContinuationToken: types.ToPtr(1)}
		expectedErr := errors.New("api error on page 2")
		mockClient.EXPECT().GetProjects(ctx, argsPage2).Return(nil, expectedErr)

		// Act
		projects, err := azdo.GetProjects(ctx, mockClient, argsPage1)

		// Assert
		require.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Empty(t, projects)
	})

	t.Run("negative - invalid continuation token", func(t *testing.T) {
		// Arrange
		args := core.GetProjectsArgs{}
		response := &core.GetProjectsResponseValue{
			Value:             []core.TeamProjectReference{{Name: types.ToPtr("Project1")}},
			ContinuationToken: "not-an-integer",
		}
		mockClient.EXPECT().GetProjects(ctx, args).Return(response, nil)

		// Act
		projects, err := azdo.GetProjects(ctx, mockClient, args)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse continuation token")
		assert.Empty(t, projects)
	})
}
