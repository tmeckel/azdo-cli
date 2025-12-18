package github

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestRunCreate_WithTokenFlag(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	ios, _, _, _ := iostreams.Test()
	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	// Core client is used by ResolveProjectReference to fetch project metadata
	mockCore := mocks.NewMockCoreClient(ctrl)
	mClientFactory.EXPECT().Core(gomock.Any(), "org1").Return(mockCore, nil).AnyTimes()
	mockCore.EXPECT().GetProject(gomock.Any(), gomock.Any()).Return(&core.TeamProject{Id: types.ToPtr(uuid.New())}, nil).AnyTimes()

	mockSEClient := mocks.NewMockServiceEndpointClient(ctrl)
	// The connection factory mock exposes ServiceEndpoint via ClientFactory mock
	mClientFactory.EXPECT().ServiceEndpoint(gomock.Any(), "org1").Return(mockSEClient, nil).AnyTimes()

	// Printer mock for table output
	mPrinter := mocks.NewMockPrinter(ctrl)
	mCmdCtx.EXPECT().Printer(gomock.Any()).Return(mPrinter, nil).AnyTimes()
	mPrinter.EXPECT().AddColumns(gomock.Any()).AnyTimes()
	mPrinter.EXPECT().AddField(gomock.Any()).AnyTimes()
	mPrinter.EXPECT().EndRow().AnyTimes()
	mPrinter.EXPECT().Render().AnyTimes()

	// Expect CreateServiceEndpoint to be called and return a created endpoint
	created := &serviceendpoint.ServiceEndpoint{
		Id:   types.ToPtr(uuid.New()),
		Name: types.ToPtr("ep-name"),
		Type: types.ToPtr("github"),
		Url:  types.ToPtr("https://github.com"),
	}
	mockSEClient.EXPECT().CreateServiceEndpoint(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args serviceendpoint.CreateServiceEndpointArgs) (*serviceendpoint.ServiceEndpoint, error) {
			// Validate basic shape
			if args.Endpoint == nil {
				t.Fatalf("expected endpoint payload")
			}
			return created, nil
		},
	).Times(1)

	mCmdCtx.EXPECT().Prompter().Return(nil, nil).AnyTimes()

	opts := &createOptions{
		project: "org1/proj1",
		name:    "ep-name",
		url:     "",
		token:   "tok-flag",
	}

	// run
	err := runCreate(mCmdCtx, opts)
	assert.NoError(t, err)
}

func TestRunCreate_PromptForToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	ios, _, _, _ := iostreams.Test()
	// enable prompting
	ios.SetStdinTTY(true)
	ios.SetStdoutTTY(true)
	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mockCore := mocks.NewMockCoreClient(ctrl)
	mClientFactory.EXPECT().Core(gomock.Any(), "org1").Return(mockCore, nil).AnyTimes()
	mockCore.EXPECT().GetProject(gomock.Any(), gomock.Any()).Return(&core.TeamProject{Id: types.ToPtr(uuid.New())}, nil).AnyTimes()

	mockSEClient := mocks.NewMockServiceEndpointClient(ctrl)
	mClientFactory.EXPECT().ServiceEndpoint(gomock.Any(), "org1").Return(mockSEClient, nil).AnyTimes()

	// Printer mock for table output
	mPrinter := mocks.NewMockPrinter(ctrl)
	mCmdCtx.EXPECT().Printer(gomock.Any()).Return(mPrinter, nil).AnyTimes()
	mPrinter.EXPECT().AddColumns(gomock.Any()).AnyTimes()
	mPrinter.EXPECT().AddField(gomock.Any()).AnyTimes()
	mPrinter.EXPECT().EndRow().AnyTimes()
	mPrinter.EXPECT().Render().AnyTimes()

	created := &serviceendpoint.ServiceEndpoint{
		Id:   types.ToPtr(uuid.New()),
		Name: types.ToPtr("ep-name"),
		Type: types.ToPtr("github"),
		Url:  types.ToPtr("https://github.com"),
	}
	mockSEClient.EXPECT().CreateServiceEndpoint(gomock.Any(), gomock.Any()).Return(created, nil).Times(1)

	// prompter mock: return a password
	prom := mocks.NewMockPrompter(ctrl)
	prom.EXPECT().Password(gomock.Any()).Return("sometoken", nil).Times(1)
	mCmdCtx.EXPECT().Prompter().Return(prom, nil).AnyTimes()

	opts := &createOptions{
		project: "org1/proj1",
		name:    "ep-name",
		url:     "",
		token:   "",
	}

	err := runCreate(mCmdCtx, opts)
	assert.NoError(t, err)
}

func TestRunCreate_WithConfigurationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	ios, _, _, _ := iostreams.Test()
	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mockCore := mocks.NewMockCoreClient(ctrl)
	mClientFactory.EXPECT().Core(gomock.Any(), "org1").Return(mockCore, nil).AnyTimes()
	mockCore.EXPECT().GetProject(gomock.Any(), gomock.Any()).Return(&core.TeamProject{Id: types.ToPtr(uuid.New())}, nil).AnyTimes()

	mockSEClient := mocks.NewMockServiceEndpointClient(ctrl)
	mClientFactory.EXPECT().ServiceEndpoint(gomock.Any(), "org1").Return(mockSEClient, nil).AnyTimes()

	// Printer mock for table output
	mPrinter := mocks.NewMockPrinter(ctrl)
	mCmdCtx.EXPECT().Printer(gomock.Any()).Return(mPrinter, nil).AnyTimes()
	mPrinter.EXPECT().AddColumns(gomock.Any()).AnyTimes()
	mPrinter.EXPECT().AddField(gomock.Any()).AnyTimes()
	mPrinter.EXPECT().EndRow().AnyTimes()
	mPrinter.EXPECT().Render().AnyTimes()

	// Expect CreateServiceEndpoint and validate Authorization scheme/params
	mockSEClient.EXPECT().CreateServiceEndpoint(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args serviceendpoint.CreateServiceEndpointArgs) (*serviceendpoint.ServiceEndpoint, error) {
			if args.Endpoint == nil || args.Endpoint.Authorization == nil {
				t.Fatalf("expected endpoint with authorization")
			}
			if types.GetValue(args.Endpoint.Authorization.Scheme, "") != "InstallationToken" {
				t.Fatalf("expected InstallationToken scheme, got %v", types.GetValue(args.Endpoint.Authorization.Scheme, ""))
			}
			if args.Endpoint.Authorization.Parameters == nil {
				t.Fatalf("expected parameters map")
			}
			if val, ok := (*args.Endpoint.Authorization.Parameters)["ConfigurationId"]; !ok || val != "cfg-123" {
				t.Fatalf("expected ConfigurationId=cfg-123, got %v", (*args.Endpoint.Authorization.Parameters)["ConfigurationId"])
			}
			created := &serviceendpoint.ServiceEndpoint{
				Id:   types.ToPtr(uuid.New()),
				Name: types.ToPtr("ep-name"),
				Type: types.ToPtr("github"),
				Url:  types.ToPtr("https://github.com"),
			}
			return created, nil
		},
	).Times(1)

	mCmdCtx.EXPECT().Prompter().Return(nil, nil).AnyTimes()

	opts := &createOptions{
		project:         "org1/proj1",
		name:            "ep-name",
		url:             "",
		configurationID: "cfg-123",
	}

	err := runCreate(mCmdCtx, opts)
	assert.NoError(t, err)
}
