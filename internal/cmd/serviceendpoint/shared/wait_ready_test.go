package shared

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
    "github.com/tmeckel/azdo-cli/internal/mocks"
    "go.uber.org/mock/gomock"
)

func TestWaitForReadySuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

    mock := mocks.NewMockServiceEndpointClient(ctrl)
    // first call returns not ready, second returns ready
    id := uuid.New()
    ep := serviceendpoint.ServiceEndpoint{Id: &id}

    mock.EXPECT().GetServiceEndpointDetails(gomock.Any(), gomock.Any()).Return(&serviceendpoint.ServiceEndpoint{Id: &id, IsReady: newTrue()}, nil).AnyTimes()

    _, err := WaitForReady(context.Background(), mock, "proj", &ep, 1*time.Second)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }
}

func TestWaitForReadyFailedState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

    mock := mocks.NewMockServiceEndpointClient(ctrl)
    id := uuid.New()
    op := map[string]any{"state": "failed"}
    mock.EXPECT().GetServiceEndpointDetails(gomock.Any(), gomock.Any()).Return(&serviceendpoint.ServiceEndpoint{Id: &id, OperationStatus: op}, nil).AnyTimes()

    ep := serviceendpoint.ServiceEndpoint{Id: &id}
    _, err := WaitForReady(context.Background(), mock, "proj", &ep, 500*time.Millisecond)
    if err == nil {
        t.Fatalf("expected error for failed state")
    }
}

func newTrue() *bool { b := true; return &b }

// utilStub is the minimal CmdContext used by tests.
// no utilStub required since WaitForReady accepts context.Context
