package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	wit        *mocks.MockWorkItemTrackingClient
	config     *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	in         *bytes.Buffer
	stdout     *bytes.Buffer
	errOut     *bytes.Buffer
}

func newDependencies(t *testing.T, organization string) *dependencies {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, in, out, errOut := iostreams.Test()

	deps := &dependencies{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		auth:       mocks.NewMockAuthConfig(ctrl),
		in:         in,
		stdout:     out,
		errOut:     errOut,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Config().Return(deps.config, nil).AnyTimes()
	deps.config.EXPECT().Authentication().Return(deps.auth).AnyTimes()
	if organization != "" {
		deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(deps.wit, nil).AnyTimes()
	}

	return deps
}

func (d *dependencies) setupDefaultOrg(org string) {
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func (d *dependencies) setupEditor(editor string) {
	d.config.EXPECT().Get([]string{"", "editor"}).Return(editor, nil).AnyTimes()
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

func (d *dependencies) stubUpdateWorkItem(t *testing.T, project string, extraFields ...map[string]any) *workitemtracking.UpdateWorkItemArgs {
	var captured workitemtracking.UpdateWorkItemArgs
	d.wit.EXPECT().UpdateWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Project)
			assert.Equal(t, project, *args.Project)
			captured = args
			fields := map[string]interface{}{"System.TeamProject": project}
			for _, extra := range extraFields {
				for k, v := range extra {
					fields[k] = v
				}
			}
			return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
		},
	)
	return &captured
}

func updatedWorkItem(id int, fields map[string]any) *workitemtracking.WorkItem {
	return &workitemtracking.WorkItem{Id: types.ToPtr(id), Fields: &fields}
}

func patchPaths(doc *[]webapi.JsonPatchOperation) []string {
	paths := make([]string, 0, len(*doc))
	for _, op := range *doc {
		paths = append(paths, types.GetValue(op.Path, ""))
	}
	return paths
}

func TestNewCmd_update(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "update [ORG:]PROJECT/ID", cmd.Use)
	assert.Equal(t, []string{"u"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.Args(cmd, []string{"Fabrikam/1234"}))
	assert.Error(t, cmd.Args(cmd, []string{"Fabrikam/1234", "Extra"}))
	assert.Error(t, cmd.Args(cmd, []string{}))

	f := cmd.Flags()
	for _, name := range []string{
		"title", "description", "description-file", "description-editor", "assigned-to",
		"state", "area", "iteration", "reason", "fields", "discussion", "bypass-rules",
		"suppress-notifications", "validate-only", "expand", "open", "json",
	} {
		assert.NotNil(t, f.Lookup(name), "flag %q must exist", name)
	}
}

func TestNewCmd_missingTarget(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project/work item target required")
}

func Test_runUpdate_minimalTitle(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam", map[string]any{"System.Title": "New title"})

	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", title: "New title"})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "add", string(*(*args.Document)[0].Op))
	assert.Equal(t, "/fields/System.Title", *(*args.Document)[0].Path)
	assert.Equal(t, "New title", (*args.Document)[0].Value)
	require.NotNil(t, args.Id)
	assert.Equal(t, 1234, *args.Id)
	assert.Contains(t, deps.stdout.String(), "New title")
}

func Test_runUpdate_allOptionalFields_canonicalOrder(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	opts := &updateOptions{
		targetArg:   "Fabrikam/1234",
		title:       "T",
		description: "D",
		assignedTo:  "A",
		state:       "S",
		area:        "AR",
		iteration:   "I",
		reason:      "R",
		customFields: []string{
			"Foo.Bar=value",
		},
	}
	err := runUpdate(deps.cmd, opts)
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	assert.Equal(t, []string{
		"/fields/System.Title",
		"/fields/System.Description",
		"/fields/System.AssignedTo",
		"/fields/System.State",
		"/fields/System.AreaPath",
		"/fields/System.IterationPath",
		"/fields/System.Reason",
		"/fields/Foo.Bar",
	}, patchPaths(args.Document))
}

func Test_runUpdate_customFields(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{
		targetArg:    "Fabrikam/1234",
		title:        "T",
		customFields: []string{"Foo.Bar=value", "Baz.Qux=other"},
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	assert.Equal(t, []string{"/fields/System.Title", "/fields/Foo.Bar", "/fields/Baz.Qux"}, patchPaths(args.Document))
	require.Len(t, *args.Document, 3)
	assert.Equal(t, "value", (*args.Document)[1].Value)
	assert.Equal(t, "other", (*args.Document)[2].Value)
}

func Test_runUpdate_fieldsParseSplitOnFirstEquals(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{
		targetArg:    "Fabrikam/1234",
		customFields: []string{"Foo.Bar=key=value"},
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "/fields/Foo.Bar", *(*args.Document)[0].Path)
	assert.Equal(t, "key=value", (*args.Document)[0].Value)
}

func Test_runUpdate_customFieldsMissingEquals(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")

	err := runUpdate(deps.cmd, &updateOptions{
		targetArg:    "Fabrikam/1234",
		customFields: []string{"Foo.Bar"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--fields value \"Foo.Bar\" must be in the form Ref.Name=value")
}

func Test_runUpdate_discussionTriggersAddComment(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	var captured workitemtracking.AddCommentArgs
	deps.wit.EXPECT().AddComment(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.AddCommentArgs) (*workitemtracking.Comment, error) {
			captured = args
			return &workitemtracking.Comment{}, nil
		},
	)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", discussion: "nice work"})
	require.NoError(t, err)

	require.NotNil(t, captured.Project)
	assert.Equal(t, "Fabrikam", *captured.Project)
	require.NotNil(t, captured.WorkItemId)
	assert.Equal(t, 1234, *captured.WorkItemId)
	require.NotNil(t, captured.Request)
	require.NotNil(t, captured.Request.Text)
	assert.Equal(t, "nice work", *captured.Request.Text)
}

func Test_runUpdate_noDiscussion(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)
	deps.wit.EXPECT().AddComment(gomock.Any(), gomock.Any()).Times(0)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234"})
	require.NoError(t, err)
}

func Test_runUpdate_bypassRulesAndSuppressNotifications(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{
		targetArg:             "Fabrikam/1234",
		bypassRules:           true,
		suppressNotifications: true,
	})
	require.NoError(t, err)

	require.NotNil(t, args.BypassRules)
	assert.True(t, *args.BypassRules)
	require.NotNil(t, args.SuppressNotifications)
	assert.True(t, *args.SuppressNotifications)
	assert.Contains(t, deps.errOut.String(), "warning: --bypass-rules/--suppress-notifications")
}

func Test_runUpdate_validateOnly(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", validateOnly: true})
	require.NoError(t, err)

	require.NotNil(t, args.ValidateOnly)
	assert.True(t, *args.ValidateOnly)
}

func Test_runUpdate_expand(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", expand: "All"})
	require.NoError(t, err)

	require.NotNil(t, args.Expand)
	assert.Equal(t, workitemtracking.WorkItemExpand("All"), *args.Expand)
}

func Test_runUpdate_invalidID(t *testing.T) {
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

			deps := newDependencies(t, "myorg")
			deps.setupDefaultOrg("myorg")

			err := runUpdate(deps.cmd, &updateOptions{targetArg: tc.target})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expected)
		})
	}
}

func Test_runUpdate_projectScopeDefaultOrg(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234"})
	require.NoError(t, err)

	require.NotNil(t, args.Id)
	assert.Equal(t, 1234, *args.Id)
}

func Test_runUpdate_explicitOrganizationProject(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "myorg:Fabrikam/1234"})
	require.NoError(t, err)

	require.NotNil(t, args.Id)
	assert.Equal(t, 1234, *args.Id)
}

func Test_runUpdate_missingDefaultOrganization(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("")

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no organization specified")
}

func Test_runUpdate_ProjectMismatch(t *testing.T) {
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

			deps := newDependencies(t, "myorg")
			deps.setupDefaultOrg("myorg")
			deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).Return(
				&workitemtracking.WorkItem{Fields: tc.fields}, nil,
			)

			err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", title: "T"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expected)
		})
	}
}

func Test_runUpdate_PreflightError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch work item 1234: boom")
}

func Test_runUpdate_APIError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.wit.EXPECT().UpdateWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", title: "T"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update work item 1234: boom")
}

func Test_runUpdate_clientError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("myorg")
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), "myorg").Return(nil, fmt.Errorf("no client"))

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create work item tracking client: no client")
}

func Test_runUpdate_emptyPatchDoc(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", bypassRules: true})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	assert.Empty(t, *args.Document)
}

func Test_runUpdate_success_JSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.wit.EXPECT().UpdateWorkItem(gomock.Any(), gomock.Any()).Return(
		updatedWorkItem(1234, map[string]any{}), nil,
	)

	exporter := &captureExporter{}
	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", title: "T", exporter: exporter})
	require.NoError(t, err)

	got, ok := exporter.data.(*workitemtracking.WorkItem)
	require.True(t, ok, "exporter must receive the raw WorkItem")
	require.NotNil(t, got.Id)
	assert.Equal(t, 1234, *got.Id)
}

func Test_runUpdate_tableOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.wit.EXPECT().UpdateWorkItem(gomock.Any(), gomock.Any()).Return(
		updatedWorkItem(1234, map[string]any{
			"System.WorkItemType":  "User Story",
			"System.State":         "Active",
			"System.Title":         "Fix the bug",
			"System.AssignedTo":    "Alice <alice@x.com>",
			"System.AreaPath":      "Fabrikam\\Web",
			"System.IterationPath": "Fabrikam\\Release 1\\Sprint 1",
		}), nil,
	)
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", title: "T"})
	require.NoError(t, err)

	out := deps.stdout.String()
	assert.Contains(t, out, "1234")
	assert.Contains(t, out, "User Story")
	assert.Contains(t, out, "Active")
	assert.Contains(t, out, "Fix the bug")
	assert.Contains(t, out, "Alice <alice@x.com>")
	assert.Contains(t, out, "Fabrikam\\Web")
	assert.Contains(t, out, "Fabrikam\\Release 1\\Sprint 1")
}

func Test_runUpdate_openBrowserFlag(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", openInBrowser: true})
	require.NoError(t, err)
}

func Test_runUpdate_DescriptionFromInline(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", description: "text"})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "/fields/System.Description", *(*args.Document)[0].Path)
	assert.Equal(t, "text", (*args.Document)[0].Value)
}

func Test_runUpdate_DescriptionFromSingleFile(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "desc.md")
	require.NoError(t, os.WriteFile(file, []byte("file content"), 0o600))

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionFiles: []string{file}})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "file content", (*args.Document)[0].Value)
}

func Test_runUpdate_DescriptionFromStdin(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)
	deps.in.WriteString("stdin content")

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionFiles: []string{"-"}})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "stdin content", (*args.Document)[0].Value)
}

func Test_runUpdate_DescriptionFromMultipleFiles_Concatenated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.md")
	fileB := filepath.Join(dir, "b.md")
	require.NoError(t, os.WriteFile(fileA, []byte("alpha"), 0o600))
	require.NoError(t, os.WriteFile(fileB, []byte("beta"), 0o600))

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionFiles: []string{fileA, fileB}})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "alpha\nbeta", (*args.Document)[0].Value)
}

func Test_runUpdate_DescriptionFileNotFound(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionFiles: []string{"/nonexistent"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/nonexistent")
}

func Test_runUpdate_DescriptionFileTooLarge(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "big.md")
	require.NoError(t, os.WriteFile(file, bytes.Repeat([]byte("a"), 1024*1024+1), 0o600))

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionFiles: []string{file}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 1 MB")
}

func Test_runUpdate_DescriptionFileBinary(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "bin.md")
	require.NoError(t, os.WriteFile(file, []byte("abc\x00def"), 0o600))

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionFiles: []string{file}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "appears to be binary")
}

func Test_runUpdate_DescriptionFileNotUTF8(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "bad.md")
	require.NoError(t, os.WriteFile(file, []byte{0xff, 0xfe, 0xfd}, 0o600))

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionFiles: []string{file}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid UTF-8")
}

func Test_runUpdate_DescriptionEditor(t *testing.T) {
	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.setupEditor("")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	original := shared.ExecEditorCommand
	t.Cleanup(func() { shared.ExecEditorCommand = original })
	shared.ExecEditorCommand = fakeEditor("written by editor")

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionEditor: true})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "written by editor", (*args.Document)[0].Value)
}

func Test_runUpdate_DescriptionEditorUsesConfigEditor(t *testing.T) {
	t.Setenv("AZDO_EDITOR", "")

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.setupEditor("myeditor --wait")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	var command []string
	original := shared.ExecEditorCommand
	t.Cleanup(func() { shared.ExecEditorCommand = original })
	shared.ExecEditorCommand = func(c []string, file string) error {
		command = c
		return os.WriteFile(file, []byte("content"), 0o600)
	}

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionEditor: true})
	require.NoError(t, err)

	assert.Equal(t, []string{"myeditor", "--wait"}, command)
	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "content", (*args.Document)[0].Value)
}

func Test_runUpdate_DescriptionEditorStripsCommentLines(t *testing.T) {
	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.setupEditor("")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	original := shared.ExecEditorCommand
	t.Cleanup(func() { shared.ExecEditorCommand = original })
	shared.ExecEditorCommand = fakeEditor("# comment\n# also comment\n# my notes\nactual content\n")

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionEditor: true})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "actual content", (*args.Document)[0].Value)
}

func Test_runUpdate_DescriptionEditorEmptyAborts(t *testing.T) {
	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.setupEditor("")
	deps.stubPreflight(t, "Fabrikam")

	original := shared.ExecEditorCommand
	t.Cleanup(func() { shared.ExecEditorCommand = original })
	shared.ExecEditorCommand = fakeEditor("")

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionEditor: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "editor produced empty description")
}

func Test_runUpdate_DescriptionEditorNonZeroExit(t *testing.T) {
	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.setupEditor("")
	deps.stubPreflight(t, "Fabrikam")

	original := shared.ExecEditorCommand
	t.Cleanup(func() { shared.ExecEditorCommand = original })
	shared.ExecEditorCommand = func(_ []string, _ string) error {
		return errors.New("exit status 1")
	}

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", descriptionEditor: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 1")
}

func Test_runUpdate_DescriptionPrecedenceEditorOverFile(t *testing.T) {
	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.setupEditor("")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	original := shared.ExecEditorCommand
	t.Cleanup(func() { shared.ExecEditorCommand = original })
	shared.ExecEditorCommand = fakeEditor("editor content")

	file := filepath.Join(t.TempDir(), "desc.md")
	require.NoError(t, os.WriteFile(file, []byte("file content"), 0o600))

	err := runUpdate(deps.cmd, &updateOptions{
		targetArg:         "Fabrikam/1234",
		descriptionEditor: true,
		descriptionFiles:  []string{file},
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "editor content", (*args.Document)[0].Value)
	assert.Contains(t, deps.errOut.String(), "takes precedence over --description-file")
}

func Test_runUpdate_DescriptionPrecedenceEditorOverInline(t *testing.T) {
	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.setupEditor("")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	original := shared.ExecEditorCommand
	t.Cleanup(func() { shared.ExecEditorCommand = original })
	shared.ExecEditorCommand = fakeEditor("editor content")

	err := runUpdate(deps.cmd, &updateOptions{
		targetArg:         "Fabrikam/1234",
		descriptionEditor: true,
		description:       "inline content",
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "editor content", (*args.Document)[0].Value)
	assert.Contains(t, deps.errOut.String(), "takes precedence over --description")
}

func Test_runUpdate_DescriptionPrecedenceFileOverInline(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "desc.md")
	require.NoError(t, os.WriteFile(file, []byte("file content"), 0o600))

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{
		targetArg:        "Fabrikam/1234",
		descriptionFiles: []string{file},
		description:      "inline content",
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "file content", (*args.Document)[0].Value)
	assert.Contains(t, deps.errOut.String(), "takes precedence over --description")
}

func Test_runUpdate_DescriptionAbsent_OmitsPatchOp(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubPreflight(t, "Fabrikam")
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, deps.stdout), nil)

	err := runUpdate(deps.cmd, &updateOptions{targetArg: "Fabrikam/1234", title: "T"})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.NotEqual(t, "/fields/System.Description", *(*args.Document)[0].Path)
}

func mustListPrinter(t *testing.T, w io.Writer) printer.Printer {
	t.Helper()
	tp, err := printer.NewListPrinter(w)
	require.NoError(t, err)
	return tp
}

func fakeEditor(content string) func([]string, string) error {
	return func(_ []string, file string) error {
		return os.WriteFile(file, []byte(content), 0o600)
	}
}

type captureExporter struct {
	data any
}

func (c *captureExporter) Fields() []string { return nil }
func (c *captureExporter) Write(_ *iostreams.IOStreams, data any) error {
	c.data = data
	return nil
}
