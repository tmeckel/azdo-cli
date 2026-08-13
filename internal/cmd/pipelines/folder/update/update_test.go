package update

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	buildCli   *mocks.MockBuildClient
	config     *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func newDependencies(t *testing.T, organization string) *dependencies {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &dependencies{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		buildCli:   mocks.NewMockBuildClient(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	if organization != "" {
		deps.clientFact.EXPECT().Build(gomock.Any(), organization).Return(deps.buildCli, nil).AnyTimes()
	}

	return deps
}

func (d *dependencies) setupDefaultOrg(org string) {
	d.config = mocks.NewMockConfig(d.ctrl)
	d.auth = mocks.NewMockAuthConfig(d.ctrl)
	d.cmd.EXPECT().Config().Return(d.config, nil).AnyTimes()
	d.config.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func TestNewCmd_update(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "update [ORG:]PROJECT/PATH", cmd.Use)
	assert.ElementsMatch(t, []string{"u"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.Args(cmd, []string{"Fabrikam/External"}))
	assert.Error(t, cmd.Args(cmd, []string{"Fabrikam/External", "Extra"}))

	f := cmd.Flags()
	assert.NotNil(t, f.Lookup("new-path"))
	assert.NotNil(t, f.Lookup("new-description"))
	assert.NotNil(t, f.Lookup("json"))
	assert.NotNil(t, f.Lookup("jq"))
	assert.NotNil(t, f.Lookup("template"))
}

func TestNewCmd_missingPath(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestRunUpdate_success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		targetArg       string
		newPath         string
		newDescription  string
		expectedOrg     string
		expectedProject string
		expectedPath    string
		fetchedFolder   build.Folder
		expectedFolder  build.Folder
		expectedOutput  string
	}{
		{
			name:            "rename only",
			targetArg:       "MyProject/P/OldName",
			newPath:         "P/NewName",
			expectedOrg:     "myorg",
			expectedProject: "MyProject",
			expectedPath:    "P/OldName",
			fetchedFolder:   build.Folder{Path: types.ToPtr("P/OldName"), Description: types.ToPtr("d")},
			expectedFolder:  build.Folder{Path: types.ToPtr("P/NewName"), Description: types.ToPtr("d")},
			expectedOutput:  "Updated folder P/NewName\n",
		},
		{
			name:            "description only",
			targetArg:       "MyProject/P/Foo",
			newDescription:  "newDesc",
			expectedOrg:     "myorg",
			expectedProject: "MyProject",
			expectedPath:    "P/Foo",
			fetchedFolder:   build.Folder{Path: types.ToPtr("P/Foo"), Description: types.ToPtr("oldDesc")},
			expectedFolder:  build.Folder{Path: types.ToPtr("P/Foo"), Description: types.ToPtr("newDesc")},
			expectedOutput:  "Updated folder P/Foo\n",
		},
		{
			name:            "both",
			targetArg:       "myorg:MyProject/P/Foo",
			newPath:         "P/NewName",
			newDescription:  "newDesc",
			expectedOrg:     "myorg",
			expectedProject: "MyProject",
			expectedPath:    "P/Foo",
			fetchedFolder:   build.Folder{Path: types.ToPtr("P/Foo"), Description: types.ToPtr("oldDesc")},
			expectedFolder:  build.Folder{Path: types.ToPtr("P/NewName"), Description: types.ToPtr("newDesc")},
			expectedOutput:  "Updated folder P/NewName\n",
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, tc.expectedOrg)
			deps.setupDefaultOrg("myorg")

			deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args build.GetFoldersArgs) (*[]build.Folder, error) {
					require.NotNil(t, args.Project)
					assert.Equal(t, tc.expectedProject, *args.Project)
					require.NotNil(t, args.Path)
					assert.Equal(t, tc.expectedPath, *args.Path)
					return &[]build.Folder{tc.fetchedFolder}, nil
				},
			)

			var captured *build.Folder
			updated := build.Folder{Path: types.ToPtr("P/NewName")}
			if tc.newPath == "" {
				updated.Path = types.ToPtr(tc.expectedPath)
			}
			if tc.newDescription != "" {
				updated.Description = types.ToPtr(tc.newDescription)
			}
			deps.buildCli.EXPECT().UpdateFolder(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args build.UpdateFolderArgs) (*build.Folder, error) {
					require.NotNil(t, args.Folder)
					require.NotNil(t, args.Project)
					assert.Equal(t, tc.expectedProject, *args.Project)
					require.NotNil(t, args.Path)
					assert.Equal(t, tc.expectedPath, *args.Path)
					captured = args.Folder
					return &updated, nil
				},
			)

			err := runUpdate(deps.cmd, &opts{targetArg: tc.targetArg, newPath: tc.newPath, newDescription: tc.newDescription})
			require.NoError(t, err)
			require.NotNil(t, captured)
			assert.Equal(t, types.GetValue(tc.expectedFolder.Path, ""), types.GetValue(captured.Path, ""))
			assert.Equal(t, types.GetValue(tc.expectedFolder.Description, ""), types.GetValue(captured.Description, ""))
			assert.Equal(t, tc.expectedOutput, deps.stdout.String())
		})
	}
}

func TestRunUpdate_mutexViolation(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")

	err := runUpdate(deps.cmd, &opts{targetArg: "MyProject/P/Foo"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specify at least one of --new-path or --new-description")
}

func TestRunUpdate_folderNotFound(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).Return(&[]build.Folder{}, nil)

	err := runUpdate(deps.cmd, &opts{targetArg: "MyProject/P/Foo", newDescription: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunUpdate_pathAmbiguous(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).Return(
		&[]build.Folder{
			{Path: types.ToPtr("P/Foo")},
			{Path: types.ToPtr("P/Foo")},
		}, nil,
	)

	err := runUpdate(deps.cmd, &opts{targetArg: "MyProject/P/Foo", newDescription: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "matched 2 folders; expected exactly 1")
}

func TestRunUpdate_getFoldersError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("boom"))

	err := runUpdate(deps.cmd, &opts{targetArg: "MyProject/P/Foo", newDescription: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch folder P/Foo: boom")
}

func TestRunUpdate_updateError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).Return(
		&[]build.Folder{{Path: types.ToPtr("P/Foo")}}, nil,
	)
	deps.buildCli.EXPECT().UpdateFolder(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("boom"))

	err := runUpdate(deps.cmd, &opts{targetArg: "MyProject/P/Foo", newDescription: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update folder P/Foo: boom")
}

func TestRunUpdate_JSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.buildCli.EXPECT().GetFolders(gomock.Any(), gomock.Any()).Return(
		&[]build.Folder{{Path: types.ToPtr("P/OldName"), Description: types.ToPtr("d")}}, nil,
	)
	updated := build.Folder{Path: types.ToPtr("P/NewName"), Description: types.ToPtr("newDesc")}
	deps.buildCli.EXPECT().UpdateFolder(gomock.Any(), gomock.Any()).Return(&updated, nil)

	exporter := util.NewJSONExporter()
	err := runUpdate(deps.cmd, &opts{targetArg: "MyProject/P/OldName", newPath: "P/NewName", newDescription: "newDesc", exporter: exporter})
	require.NoError(t, err)

	var parsed struct {
		Path        *string `json:"path"`
		Description *string `json:"description"`
	}
	err = json.Unmarshal(deps.stdout.Bytes(), &parsed)
	require.NoError(t, err)
	require.NotNil(t, parsed.Path)
	require.NotNil(t, parsed.Description)
	assert.Equal(t, "P/NewName", *parsed.Path)
	assert.Equal(t, "newDesc", *parsed.Description)
}
