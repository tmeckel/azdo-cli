package delete

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
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
	wit        *mocks.MockWorkItemTrackingClient
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
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		prompter:   mocks.NewMockPrompter(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).AnyTimes()
	if organization != "" {
		deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(deps.wit, nil).AnyTimes()
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

func (d *dependencies) stubPreflight(t *testing.T, project string) {
	d.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			require.NotNil(t, args.Project)
			assert.Equal(t, project, *args.Project)
			fields := map[string]interface{}{"System.TeamProject": project}
			return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
		},
	)
}

func TestNewCmd_delete(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "delete [ORG:]PROJECT/ID", cmd.Use)
	assert.ElementsMatch(t, []string{"d", "del", "rm"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.Args(cmd, []string{"Fabrikam/1234"}))
	assert.Error(t, cmd.Args(cmd, []string{"Fabrikam/1234", "Extra"}))
	assert.Error(t, cmd.Args(cmd, []string{}))

	f := cmd.Flags()
	assert.NotNil(t, f.Lookup("yes"))
	assert.NotNil(t, f.Lookup("destroy"))
	assert.NotNil(t, f.Lookup("json"))
}

func TestNewCmd_missingTarget(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project/work item target required")
}

func TestRunDelete_success_withYes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		targetArg   string
		defaultOrg  string
		expectedOrg string
	}{
		{
			name:        "implicit org",
			targetArg:   "Fabrikam/1234",
			defaultOrg:  "myorg",
			expectedOrg: "myorg",
		},
		{
			name:        "explicit org",
			targetArg:   "myorg:Fabrikam/1234",
			expectedOrg: "myorg",
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
			deps.stubPreflight(t, "Fabrikam")

			deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args workitemtracking.DeleteWorkItemArgs) (*workitemtracking.WorkItemDelete, error) {
					require.NotNil(t, args.Project)
					assert.Equal(t, "Fabrikam", *args.Project)
					require.NotNil(t, args.Id)
					assert.Equal(t, 1234, *args.Id)
					require.NotNil(t, args.Destroy)
					assert.False(t, *args.Destroy)
					return &workitemtracking.WorkItemDelete{Id: args.Id}, nil
				},
			)

			err := runDelete(deps.cmd, &opts{targetArg: tc.targetArg, yes: true})
			require.NoError(t, err)
			assert.Equal(t, "Deleted work item 1234\n", deps.stdout.String())
		})
	}
}

func TestRunDelete_success_destroy(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")

	deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.DeleteWorkItemArgs) (*workitemtracking.WorkItemDelete, error) {
			require.NotNil(t, args.Destroy)
			assert.True(t, *args.Destroy)
			return &workitemtracking.WorkItemDelete{Id: args.Id}, nil
		},
	)

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true, destroy: true})
	require.NoError(t, err)
	assert.Equal(t, "Permanently deleted work item 1234\n", deps.stdout.String())
}

func TestRunDelete_success_confirmed(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", true)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).Return(&workitemtracking.WorkItemDelete{}, nil)
	deps.prompter.EXPECT().Confirm(
		"Are you sure you want to delete this work item?",
		false,
	).Return(true, nil)

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234"})
	require.NoError(t, err)
	assert.Equal(t, "Deleted work item 1234\n", deps.stdout.String())
}

func TestRunDelete_cancelled(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", true)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234"})
	require.ErrorIs(t, err, util.ErrCancel)
	assert.Empty(t, deps.stdout.String())
}

func TestRunDelete_notInteractive(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes required when not running interactively")
}

func TestRunDelete_invalidID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		target   string
		expected string
	}{
		{name: "non numeric", target: "Fabrikam/abc", expected: `work item ID must be a positive integer; got "abc"`},
		{name: "zero", target: "Fabrikam/0", expected: `work item ID must be a positive integer; got "0"`},
		{name: "negative", target: "Fabrikam/-5", expected: `work item ID must be a positive integer; got "-5"`},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg", false)
			deps.setupDefaultOrg("myorg")

			err := runDelete(deps.cmd, &opts{targetArg: tc.target, yes: true})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expected)
		})
	}
}

func TestRunDelete_ProjectMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fields   *map[string]interface{}
		expected string
	}{
		{
			name:     "different project",
			fields:   &map[string]interface{}{"System.TeamProject": "OtherProject"},
			expected: `work item 1234 does not belong to project "Fabrikam"`,
		},
		{
			name:     "missing team project field",
			fields:   &map[string]interface{}{},
			expected: `work item 1234 does not belong to project "Fabrikam"`,
		},
		{
			name:     "nil fields",
			expected: `work item 1234 does not belong to project "Fabrikam"`,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg", false)
			deps.setupDefaultOrg("myorg")
			deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).Return(
				&workitemtracking.WorkItem{Fields: tc.fields}, nil,
			)

			err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expected)
			assert.Empty(t, deps.stdout.String())
		})
	}
}

func TestRunDelete_preflightError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch work item 1234: boom")
}

func TestRunDelete_success_empty204Body(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	// Official 204 No Content: the API commits the deletion and returns an
	// empty body, which the SDK reports as a JSON decode error. The recycle
	// bin delete must still be treated as successful.
	deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.DeleteWorkItemArgs) (*workitemtracking.WorkItemDelete, error) {
			require.NotNil(t, args.Destroy)
			assert.False(t, *args.Destroy)
			return nil, &json.SyntaxError{}
		},
	)

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true})
	require.NoError(t, err)
	assert.Equal(t, "Deleted work item 1234\n", deps.stdout.String())
}

func TestRunDelete_empty204Body_JSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).Return(nil, &json.SyntaxError{})

	exporter := &captureExporter{}
	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true, exporter: exporter})
	require.NoError(t, err)

	got, ok := exporter.data.(*workitemtracking.WorkItemDelete)
	require.True(t, ok, "exporter must receive the raw WorkItemDelete")
	require.NotNil(t, got.Id, "--json output must not be {} for a 204 response")
	assert.Equal(t, 1234, *got.Id)
}

func TestRunDelete_APIError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete work item 1234: boom")
}

func TestRunDelete_nilResponse(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).Return(nil, nil)

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "work item tracking API returned an empty response")
}

func TestRunDelete_clientError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "", false)
	deps.setupDefaultOrg("myorg")
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), "myorg").Return(nil, fmt.Errorf("no client"))

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create work item tracking client: no client")
}

func TestRunDelete_missingDefaultOrganization(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "", false)
	deps.setupDefaultOrg("")

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no organization specified")
}

func TestRunDelete_success_JSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).Return(
		&workitemtracking.WorkItemDelete{Id: types.ToPtr(1234), Name: types.ToPtr("Fix bug")}, nil,
	)

	exporter := &captureExporter{}
	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true, exporter: exporter})
	require.NoError(t, err)

	got, ok := exporter.data.(*workitemtracking.WorkItemDelete)
	require.True(t, ok, "exporter must receive the raw WorkItemDelete")
	assert.Equal(t, 1234, *got.Id)
	assert.Equal(t, "Fix bug", *got.Name)
}

func TestRunDelete_EmptyResponse_PopulatesIDForJSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	// Official 204 No Content: the SDK returns an empty WorkItemDelete.
	deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).Return(&workitemtracking.WorkItemDelete{}, nil)

	exporter := &captureExporter{}
	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", yes: true, exporter: exporter})
	require.NoError(t, err)

	got, ok := exporter.data.(*workitemtracking.WorkItemDelete)
	require.True(t, ok, "exporter must receive the raw WorkItemDelete")
	require.NotNil(t, got.Id, "--json output must not be {} for a 204 response")
	assert.Equal(t, 1234, *got.Id)
}

func TestRunDelete_destroyPromptText(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", true)
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.wit.EXPECT().DeleteWorkItem(gomock.Any(), gomock.Any()).Return(&workitemtracking.WorkItemDelete{}, nil)

	var promptMessage string
	deps.prompter.EXPECT().Confirm(gomock.Any(), false).DoAndReturn(
		func(message string, defaultValue bool) (bool, error) {
			promptMessage = message
			return true, nil
		},
	)

	err := runDelete(deps.cmd, &opts{targetArg: "Fabrikam/1234", destroy: true})
	require.NoError(t, err)
	assert.Contains(t, promptMessage, "permanently destroy")
	assert.Contains(t, promptMessage, "cannot be undone")
}

type captureExporter struct {
	data any
}

func (c *captureExporter) Fields() []string { return nil }
func (c *captureExporter) Write(_ *iostreams.IOStreams, data any) error {
	c.data = data
	return nil
}
