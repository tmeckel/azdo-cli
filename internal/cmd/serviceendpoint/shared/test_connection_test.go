package shared

import (
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/test"
	typespkg "github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestTestConnectionSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mocks.NewMockServiceEndpointClient(ctrl)
	name := typespkg.ToPtr("ok")
	mock.EXPECT().ExecuteServiceEndpointRequest(gomock.Any(), gomock.Any()).Return(&serviceendpoint.ServiceEndpointRequestResult{StatusCode: name}, nil)

	// seed metadata cache so TestConnection finds TestConnection data source
	dt := serviceendpoint.DataSource{Name: typespkg.ToPtr("TestConnection")}
	st := serviceendpoint.ServiceEndpointType{Name: typespkg.ToPtr("github"), DataSources: &[]serviceendpoint.DataSource{dt}}
	setTypesCacheForTest("org", []serviceendpoint.ServiceEndpointType{st})

	ep := &serviceendpoint.ServiceEndpoint{Type: typespkg.ToPtr("github"), Url: typespkg.ToPtr("https://github.com")}
	// minimal cmdCtx stub implementing util.CmdContext methods
	cmdCtx := test.NewTestContext(t)
	err := TestConnection(cmdCtx, mock, "org", "proj", ep, 1*time.Second)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestTestConnectionNotSupported(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mocks.NewMockServiceEndpointClient(ctrl)
	ep := &serviceendpoint.ServiceEndpoint{Type: typespkg.ToPtr("unknown-type")}
	// ensure types cache does not contain this type
	clearTypesCacheForTest()
	cmdCtx := test.NewTestContext(t)
	err := TestConnection(cmdCtx, mock, "org", "proj", ep, 100*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error for unsupported type")
	}
}

func TestTestConnectionRetriesThenFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mocks.NewMockServiceEndpointClient(ctrl)
	notOk := typespkg.ToPtr("error")
	// always return error status
	mock.EXPECT().ExecuteServiceEndpointRequest(gomock.Any(), gomock.Any()).Return(&serviceendpoint.ServiceEndpointRequestResult{StatusCode: notOk}, nil).AnyTimes()

	// seed metadata cache for github
	dt := serviceendpoint.DataSource{Name: typespkg.ToPtr("TestConnection")}
	st := serviceendpoint.ServiceEndpointType{Name: typespkg.ToPtr("github"), DataSources: &[]serviceendpoint.DataSource{dt}}
	setTypesCacheForTest("org", []serviceendpoint.ServiceEndpointType{st})

	ep := &serviceendpoint.ServiceEndpoint{Type: typespkg.ToPtr("github"), Url: typespkg.ToPtr("https://github.com")}
	cmdCtx := test.NewTestContext(t)
	err := TestConnection(cmdCtx, mock, "org", "proj", ep, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected failure")
	}
}
