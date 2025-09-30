package create

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/stretchr/testify/assert"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"go.uber.org/mock/gomock"
)

func TestNewCmd_mutuallyExclusiveFlags(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)

	cmd := NewCmd(mockCmdCtx)
	cmd.SetArgs([]string{"myorg/myproject", "--no-wait", "--max-wait", "10"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Equal(t, "--no-wait and --max-wait are mutually exclusive", err.Error())
}

func TestRunCommand_Negative(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		setupMocks    func(*mocks.MockCmdContext, *mocks.MockClientFactory, *mocks.MockCoreClient, *mocks.MockOperationsClient)
		expectedError string
	}{
		{
			name:          "Invalid source control",
			args:          []string{"myorg/myproject", "--source-control", "invalid"},
			expectedError: "invalid source control type: invalid",
		},
		{
			name:          "Invalid visibility",
			args:          []string{"myorg/myproject", "--visibility", "invalid"},
			expectedError: "invalid visibility: invalid",
		},
		{
			name: "Process not found",
			args: []string{"myorg/myproject", "--process", "nonexistent"},
			setupMocks: func(_ *mocks.MockCmdContext, _ *mocks.MockClientFactory, mockCoreClient *mocks.MockCoreClient, _ *mocks.MockOperationsClient) {
				mockCoreClient.EXPECT().GetProcesses(gomock.Any(), gomock.Any()).Return(&[]core.Process{}, nil)
			},
			expectedError: "process 'nonexistent' not found",
		},
		{
			name: "GetProcesses API error",
			args: []string{"myorg/myproject", "--process", "Agile"},
			setupMocks: func(_ *mocks.MockCmdContext, _ *mocks.MockClientFactory, mockCoreClient *mocks.MockCoreClient, _ *mocks.MockOperationsClient) {
				mockCoreClient.EXPECT().GetProcesses(gomock.Any(), gomock.Any()).Return(nil, errors.New("API error"))
			},
			expectedError: "failed to get processes: API error",
		},
		{
			name: "QueueCreateProject API error",
			args: []string{"myorg/myproject"},
			setupMocks: func(_ *mocks.MockCmdContext, _ *mocks.MockClientFactory, mockCoreClient *mocks.MockCoreClient, _ *mocks.MockOperationsClient) {
				processId := uuid.New()
				agileString := "Agile"
				mockCoreClient.EXPECT().GetProcesses(gomock.Any(), gomock.Any()).Return(&[]core.Process{{Id: &processId, Name: &agileString}}, nil)
				mockCoreClient.EXPECT().QueueCreateProject(gomock.Any(), gomock.Any()).Return(nil, errors.New("API error"))
			},
			expectedError: "failed to queue project creation: API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockCmdCtx := mocks.NewMockCmdContext(ctrl)
			mockClientFactory := mocks.NewMockClientFactory(ctrl)
			mockCoreClient := mocks.NewMockCoreClient(ctrl)
			mockOperationsClient := mocks.NewMockOperationsClient(ctrl)

			io, _, _, _ := iostreams.Test()
			mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
			mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
			mockCmdCtx.EXPECT().ClientFactory().Return(mockClientFactory).AnyTimes()
			mockClientFactory.EXPECT().Core(gomock.Any(), gomock.Any()).Return(mockCoreClient, nil).AnyTimes()
			mockClientFactory.EXPECT().Operations(gomock.Any(), gomock.Any()).Return(mockOperationsClient, nil).AnyTimes()

			if tt.setupMocks != nil {
				tt.setupMocks(mockCmdCtx, mockClientFactory, mockCoreClient, mockOperationsClient)
			}

			cmd := NewCmd(mockCmdCtx)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}
