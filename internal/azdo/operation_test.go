package azdo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/operations"
	"github.com/stretchr/testify/assert"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestPollOperationResult_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockOperationsClient(ctrl)
	opID := uuid.New()
	pluginID := uuid.New()

	opRef := &operations.OperationReference{
		Id:       &opID,
		PluginId: &pluginID,
	}

	inProgressOp := &operations.Operation{
		Id:     &opID,
		Status: &operations.OperationStatusValues.InProgress,
	}
	succeededOp := &operations.Operation{
		Id:     &opID,
		Status: &operations.OperationStatusValues.Succeeded,
	}

	args := operations.GetOperationArgs{
		OperationId: &opID,
		PluginId:    &pluginID,
	}

	mockClient.EXPECT().GetOperation(gomock.Any(), args).Return(inProgressOp, nil).Times(1)
	mockClient.EXPECT().GetOperation(gomock.Any(), args).Return(succeededOp, nil).Times(1)

	op, err := azdo.PollOperationResult(context.Background(), mockClient, opRef, 5*time.Second)

	assert.NoError(t, err)
	assert.NotNil(t, op)
	assert.Equal(t, operations.OperationStatusValues.Succeeded, *op.Status)
}

func TestPollOperationResult_Timeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockOperationsClient(ctrl)
	opID := uuid.New()
	pluginID := uuid.New()

	opRef := &operations.OperationReference{
		Id:       &opID,
		PluginId: &pluginID,
	}

	inProgressOp := &operations.Operation{
		Id:     &opID,
		Status: &operations.OperationStatusValues.InProgress,
	}

	args := operations.GetOperationArgs{
		OperationId: &opID,
		PluginId:    &pluginID,
	}

	mockClient.EXPECT().GetOperation(gomock.Any(), args).Return(inProgressOp, nil).AnyTimes()

	_, err := azdo.PollOperationResult(context.Background(), mockClient, opRef, 1*time.Second)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestPollOperationResult_Failure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockOperationsClient(ctrl)
	opID := uuid.New()
	pluginID := uuid.New()

	opRef := &operations.OperationReference{
		Id:       &opID,
		PluginId: &pluginID,
	}

	failedOp := &operations.Operation{
		Id:              &opID,
		Status:          &operations.OperationStatusValues.Failed,
		DetailedMessage: types.ToPtr("Something went wrong"),
	}

	args := operations.GetOperationArgs{
		OperationId: &opID,
		PluginId:    &pluginID,
	}

	mockClient.EXPECT().GetOperation(gomock.Any(), args).Return(failedOp, nil).Times(1)

	_, err := azdo.PollOperationResult(context.Background(), mockClient, opRef, 5*time.Second)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operation")
	assert.Contains(t, err.Error(), "did not succeed")
	assert.Contains(t, err.Error(), "Something went wrong")
}

func TestPollOperationResult_GetOperationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockOperationsClient(ctrl)
	opID := uuid.New()
	pluginID := uuid.New()

	opRef := &operations.OperationReference{
		Id:       &opID,
		PluginId: &pluginID,
	}

	args := operations.GetOperationArgs{
		OperationId: &opID,
		PluginId:    &pluginID,
	}

	mockClient.EXPECT().GetOperation(gomock.Any(), args).Return(nil, errors.New("API error")).Times(1)

	_, err := azdo.PollOperationResult(context.Background(), mockClient, opRef, 5*time.Second)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get operation status")
	assert.Contains(t, err.Error(), "API error")
}

func TestPollOperationResultWithState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockOperationsClient(ctrl)
	opID := uuid.New()
	pluginID := uuid.New()

	opRef := &operations.OperationReference{
		Id:       &opID,
		PluginId: &pluginID,
	}

	args := operations.GetOperationArgs{
		OperationId: &opID,
		PluginId:    &pluginID,
	}

	tests := []struct {
		name           string
		targetState    operations.OperationStatus
		setupMocks     func()
		expectError    bool
		expectedStatus operations.OperationStatus
	}{
		{
			name:        "Success on QueuedState",
			targetState: operations.OperationStatusValues.Queued,
			setupMocks: func() {
				inProgressOp := &operations.Operation{
					Id:     &opID,
					Status: &operations.OperationStatusValues.InProgress,
				}
				queuedOp := &operations.Operation{
					Id:     &opID,
					Status: &operations.OperationStatusValues.Queued,
				}
				mockClient.EXPECT().GetOperation(gomock.Any(), args).Return(inProgressOp, nil).Times(1)
				mockClient.EXPECT().GetOperation(gomock.Any(), args).Return(queuedOp, nil).Times(1)
			},
			expectError:    false,
			expectedStatus: operations.OperationStatusValues.Queued,
		},
		{
			name:        "Failure before reaching target state",
			targetState: operations.OperationStatusValues.Succeeded,
			setupMocks: func() {
				failedOp := &operations.Operation{
					Id:              &opID,
					Status:          &operations.OperationStatusValues.Failed,
					DetailedMessage: types.ToPtr("Something went wrong"),
				}
				mockClient.EXPECT().GetOperation(gomock.Any(), args).Return(failedOp, nil).Times(1)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			op, err := azdo.PollOperationResultWithState(context.Background(), mockClient, opRef, 5*time.Second, tt.targetState)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, op)
				assert.Equal(t, tt.expectedStatus, *op.Status)
			}
		})
	}
}
