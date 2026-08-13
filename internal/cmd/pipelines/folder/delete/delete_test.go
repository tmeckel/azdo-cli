package delete

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
)

type dependencies struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	buildCli   *mocks.MockBuildClient
	prompter   *mocks.MockPrompter
	config     *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func newDependencies(t *testing.T, organization string, canPrompt bool) *dependencies {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdinTTY(canPrompt)
	io.SetStdoutTTY(canPrompt)
	io.SetStderrTTY(canPrompt)

	deps := &dependencies{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		buildCli:   mocks.NewMockBuildClient(ctrl),
		prompter:   mocks.NewMockPrompter(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).AnyTimes()
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

func TestNewCmd_delete(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "delete [ORG:]PROJECT/PATH", cmd.Use)
	assert.ElementsMatch(t, []string{"d", "del", "rm"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.Args(cmd, []string{"Fabrikam/External"}))
	assert.Error(t, cmd.Args(cmd, []string{"Fabrikam/External", "Extra"}))

	f := cmd.Flags()
	assert.NotNil(t, f.Lookup("yes"))
	assert.Nil(t, f.Lookup("json"))
}

func TestNewCmd_missingPath(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestRunDelete_success_withYes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		targetArg       string
		defaultOrg      string
		expectedOrg     string
		expectedProject string
		expectedPath    string
		expectedOutput  string
	}{
		{
			name:            "implicit org",
			targetArg:       "MyProject/Foo",
			defaultOrg:      "myorg",
			expectedOrg:     "myorg",
			expectedProject: "MyProject",
			expectedPath:    "Foo",
			expectedOutput:  "Deleted folder MyProject/Foo\n",
		},
		{
			name:            "explicit org nested path",
			targetArg:       "myorg:MyProject/External/CI",
			expectedOrg:     "myorg",
			expectedProject: "MyProject",
			expectedPath:    "External/CI",
			expectedOutput:  "Deleted folder MyProject/External/CI\n",
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, tc.expectedOrg, false)
			if tc.defaultOrg != "" {
				deps.setupDefaultOrg(tc.defaultOrg)
			}

			deps.buildCli.EXPECT().DeleteFolder(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args build.DeleteFolderArgs) error {
					require.NotNil(t, args.Project)
					assert.Equal(t, tc.expectedProject, *args.Project)
					require.NotNil(t, args.Path)
					assert.Equal(t, tc.expectedPath, *args.Path)
					return nil
				},
			)

			err := runDelete(deps.cmd, &opts{targetArg: tc.targetArg, yes: true})
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, deps.stdout.String())
		})
	}
}

func TestRunDelete_success_confirmed(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", true)
	deps.setupDefaultOrg("myorg")
	deps.buildCli.EXPECT().DeleteFolder(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.DeleteFolderArgs) error {
			require.NotNil(t, args.Project)
			assert.Equal(t, "MyProject", *args.Project)
			require.NotNil(t, args.Path)
			assert.Equal(t, "Foo", *args.Path)
			return nil
		},
	)
	deps.prompter.EXPECT().Confirm(
		"This will delete all pipelines in this folder. Are you sure you want to delete this folder?",
		false,
	).Return(true, nil)

	err := runDelete(deps.cmd, &opts{targetArg: "MyProject/Foo"})
	require.NoError(t, err)
	assert.Equal(t, "Deleted folder MyProject/Foo\n", deps.stdout.String())
}

func TestRunDelete_cancelled(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", true)
	deps.setupDefaultOrg("myorg")
	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)

	err := runDelete(deps.cmd, &opts{targetArg: "MyProject/Foo"})
	require.ErrorIs(t, err, util.ErrCancel)
	assert.Empty(t, deps.stdout.String())
}

func TestRunDelete_nonInteractive(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")

	err := runDelete(deps.cmd, &opts{targetArg: "MyProject/Foo"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes required when not running interactively")
}

func TestRunDelete_APIError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.buildCli.EXPECT().DeleteFolder(gomock.Any(), gomock.Any()).Return(errors.New("boom"))

	err := runDelete(deps.cmd, &opts{targetArg: "MyProject/Foo", yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete folder Foo: boom")
}

func TestRunDelete_missingDefaultOrganization(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "", false)
	deps.setupDefaultOrg("")

	err := runDelete(deps.cmd, &opts{targetArg: "MyProject/Foo", yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no organization specified")
}

func TestRunDelete_clientError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "", false)
	deps.setupDefaultOrg("myorg")
	deps.clientFact.EXPECT().Build(gomock.Any(), "myorg").Return(nil, fmt.Errorf("no client"))

	err := runDelete(deps.cmd, &opts{targetArg: "MyProject/Foo", yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create build client: no client")
}
