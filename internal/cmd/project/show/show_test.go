package show

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/stretchr/testify/assert"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestProjectShow(t *testing.T) {
	type D = map[string]interface{}
	projectID := uuid.New()
	projectName := "TestProject"
	orgName := "TestOrg"

	defaultProject := &core.TeamProject{
		Id:          &projectID,
		Name:        &projectName,
		Description: types.ToPtr("A test project"),
		State:       &core.ProjectStateValues.WellFormed,
		Visibility:  &core.ProjectVisibilityValues.Private,
		Revision:    types.ToPtr[uint64](123),
		Url:         types.ToPtr("https://dev.azure.com/TestOrg/_apis/projects/TestProject"),
		LastUpdateTime: &azuredevops.Time{
			Time: time.Now(),
		},
		Capabilities: &map[string]map[string]string{
			"processTemplate": {
				"templateName": "Agile",
			},
			"versioncontrol": {
				"sourceControlType": "Git",
			},
		},
		DefaultTeam: &core.WebApiTeamRef{
			Name: types.ToPtr("TestProject Team"),
		},
	}

	projectWithNilFields := &core.TeamProject{
		Id:   &projectID,
		Name: &projectName,
		LastUpdateTime: &azuredevops.Time{
			Time: time.Now(),
		},
		// All other fields are nil
	}

	tests := []struct {
		name         string
		args         []string
		setupMocks   func(*mocks.MockCmdContext, *mocks.MockClientFactory, *mocks.MockCoreClient, *mocks.MockPrinter)
		expectError  string
		expectOutput string
		expectJSON   func(t *testing.T, output string)
		useJSON      bool
	}{
		{
			name: "Success case (Table Output)",
			args: []string{orgName + "/" + projectName},
			setupMocks: func(mCmdCtx *mocks.MockCmdContext, mClientFactory *mocks.MockClientFactory, mCoreClient *mocks.MockCoreClient, mPrinter *mocks.MockPrinter) {
				mClientFactory.EXPECT().Core(gomock.Any(), orgName).Return(mCoreClient, nil)
				mCoreClient.EXPECT().GetProject(gomock.Any(), gomock.Any()).Return(defaultProject, nil)
				mCmdCtx.EXPECT().Printer("list").Return(mPrinter, nil)
				mPrinter.EXPECT().AddColumns(gomock.Any()).AnyTimes()
				mPrinter.EXPECT().AddField(gomock.Any()).AnyTimes()
				mPrinter.EXPECT().EndRow().AnyTimes()
				mPrinter.EXPECT().Render().Return(nil)
			},
		},
		{
			name:    "Success case (JSON Output)",
			args:    []string{orgName + "/" + projectName},
			useJSON: true,
			setupMocks: func(mCmdCtx *mocks.MockCmdContext, mClientFactory *mocks.MockClientFactory, mCoreClient *mocks.MockCoreClient, mPrinter *mocks.MockPrinter) {
				mClientFactory.EXPECT().Core(gomock.Any(), orgName).Return(mCoreClient, nil)
				mCoreClient.EXPECT().GetProject(gomock.Any(), gomock.Any()).Return(defaultProject, nil)
			},
			expectJSON: func(t *testing.T, output string) {
				var result projectShowResult
				err := json.Unmarshal([]byte(output), &result)
				assert.NoError(t, err)
				assert.Equal(t, projectID.String(), *result.ID)
				assert.Equal(t, projectName, *result.Name)
				assert.Equal(t, "A test project", *result.Description)
			},
		},
		{
			name: "Edge Case (nil optional fields)",
			args: []string{orgName + "/" + projectName},
			setupMocks: func(mCmdCtx *mocks.MockCmdContext, mClientFactory *mocks.MockClientFactory, mCoreClient *mocks.MockCoreClient, mPrinter *mocks.MockPrinter) {
				mClientFactory.EXPECT().Core(gomock.Any(), orgName).Return(mCoreClient, nil)
				mCoreClient.EXPECT().GetProject(gomock.Any(), gomock.Any()).Return(projectWithNilFields, nil)
				mCmdCtx.EXPECT().Printer("list").Return(mPrinter, nil)
				mPrinter.EXPECT().AddColumns(gomock.Any()).AnyTimes()
				mPrinter.EXPECT().AddField(gomock.Any()).AnyTimes()
				mPrinter.EXPECT().EndRow().AnyTimes()
				mPrinter.EXPECT().Render().Return(nil)
			},
		},
		{
			name: "Error case (Project Not Found)",
			args: []string{orgName + "/" + "NonExistentProject"},
			setupMocks: func(mCmdCtx *mocks.MockCmdContext, mClientFactory *mocks.MockClientFactory, mCoreClient *mocks.MockCoreClient, mPrinter *mocks.MockPrinter) {
				mClientFactory.EXPECT().Core(gomock.Any(), orgName).Return(mCoreClient, nil)
				mCoreClient.EXPECT().GetProject(gomock.Any(), gomock.Any()).Return(nil, errors.New("project not found"))
			},
			expectError: "project not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			io, _, out, _ := iostreams.Test()
			mCmdCtx := mocks.NewMockCmdContext(ctrl)
			mClientFactory := mocks.NewMockClientFactory(ctrl)
			mCoreClient := mocks.NewMockCoreClient(ctrl)
			mPrinter := mocks.NewMockPrinter(ctrl)
			mConfig := mocks.NewMockConfig(ctrl)
			mAuth := mocks.NewMockAuthConfig(ctrl)

			// Baseline expectations
			mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
			mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
			mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
			mCmdCtx.EXPECT().Config().Return(mConfig, nil).AnyTimes()
			mConfig.EXPECT().Authentication().Return(mAuth).AnyTimes()
			mAuth.EXPECT().GetDefaultOrganization().Return(orgName, nil).AnyTimes()

			tt.setupMocks(mCmdCtx, mClientFactory, mCoreClient, mPrinter)

			cmd := NewCmd(mCmdCtx)
			cmd.SetArgs(tt.args)

			var exporter util.Exporter
			if tt.useJSON {
				exporter = util.NewJSONExporter()
			}

			o := &opts{
				project:  tt.args[0],
				exporter: exporter,
			}

			err := runCommand(mCmdCtx, o)

			if tt.expectError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectJSON != nil {
				tt.expectJSON(t, out.String())
			}
		})
	}
}
