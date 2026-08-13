package create

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

func TestNewCmd_create(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "create [ORG:]PROJECT/PATH", cmd.Use)
	assert.ElementsMatch(t, []string{"c", "cr"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.Args(cmd, []string{"Fabrikam/External"}))
	assert.Error(t, cmd.Args(cmd, []string{"Fabrikam/External", "Extra"}))

	f := cmd.Flags()
	assert.NotNil(t, f.Lookup("description"))
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

func TestRunCreate_success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		targetArg       string
		description     string
		defaultOrg      string
		expectedOrg     string
		expectedProject string
		expectedPath    string
		expectedOutput  string
		returnedPath    string
	}{
		{
			name:            "implicit org without description",
			targetArg:       "MyProject/Foo",
			defaultOrg:      "myorg",
			expectedOrg:     "myorg",
			expectedProject: "MyProject",
			expectedPath:    "Foo",
			expectedOutput:  "Created folder MyProject/Foo\n",
			returnedPath:    "MyProject/Foo",
		},
		{
			name:            "implicit org with description",
			targetArg:       "MyProject/Foo",
			description:     "hello",
			defaultOrg:      "myorg",
			expectedOrg:     "myorg",
			expectedProject: "MyProject",
			expectedPath:    "Foo",
			expectedOutput:  "Created folder MyProject/Foo\n",
			returnedPath:    "MyProject/Foo",
		},
		{
			name:            "explicit org nested path",
			targetArg:       "myorg:MyProject/External/CI",
			expectedOrg:     "myorg",
			expectedProject: "MyProject",
			expectedPath:    "External/CI",
			expectedOutput:  "Created folder MyProject/External/CI\n",
			returnedPath:    "MyProject/External/CI",
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, tc.expectedOrg)
			if tc.defaultOrg != "" {
				deps.setupDefaultOrg(tc.defaultOrg)
			}

			returned := build.Folder{Path: types.ToPtr(tc.returnedPath)}
			deps.buildCli.EXPECT().CreateFolder(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args build.CreateFolderArgs) (*build.Folder, error) {
					require.NotNil(t, args.Folder)
					require.NotNil(t, args.Folder.Description)
					assert.Equal(t, tc.description, *args.Folder.Description)
					require.NotNil(t, args.Project)
					assert.Equal(t, tc.expectedProject, *args.Project)
					require.NotNil(t, args.Path)
					assert.Equal(t, tc.expectedPath, *args.Path)
					return &returned, nil
				},
			)

			err := runCreate(deps.cmd, &opts{targetArg: tc.targetArg, description: tc.description})
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, deps.stdout.String())
		})
	}
}

func TestRunCreate_APIError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.buildCli.EXPECT().CreateFolder(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("boom"))

	err := runCreate(deps.cmd, &opts{targetArg: "MyProject/Foo"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create folder: boom")
}

func TestRunCreate_JSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	returned := build.Folder{Path: types.ToPtr("MyProject/Foo"), Description: types.ToPtr("hello")}
	deps.buildCli.EXPECT().CreateFolder(gomock.Any(), gomock.Any()).Return(&returned, nil)

	exporter := util.NewJSONExporter()
	err := runCreate(deps.cmd, &opts{targetArg: "MyProject/Foo", exporter: exporter})
	require.NoError(t, err)

	var parsed struct {
		Path        *string `json:"path"`
		Description *string `json:"description"`
	}
	err = json.Unmarshal(deps.stdout.Bytes(), &parsed)
	require.NoError(t, err)
	require.NotNil(t, parsed.Path)
	require.NotNil(t, parsed.Description)
	assert.Equal(t, "MyProject/Foo", *parsed.Path)
	assert.Equal(t, "hello", *parsed.Description)
}

func TestRunCreate_defaultOrganization(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("myorg")
	deps.clientFact.EXPECT().Build(gomock.Any(), "myorg").Return(deps.buildCli, nil).AnyTimes()
	deps.buildCli.EXPECT().CreateFolder(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.CreateFolderArgs) (*build.Folder, error) {
			require.NotNil(t, args.Project)
			assert.Equal(t, "MyProject", *args.Project)
			return &build.Folder{Path: types.ToPtr("MyProject/Foo")}, nil
		},
	)

	err := runCreate(deps.cmd, &opts{targetArg: "MyProject/Foo"})
	require.NoError(t, err)
	assert.Equal(t, "Created folder MyProject/Foo\n", deps.stdout.String())
}

func TestRunCreate_missingDefaultOrganization(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("")

	err := runCreate(deps.cmd, &opts{targetArg: "MyProject/Foo"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no organization specified")
}
