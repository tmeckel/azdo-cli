package remove

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	wishared "github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
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
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		prompter:   mocks.NewMockPrompter(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		auth:       mocks.NewMockAuthConfig(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).AnyTimes()
	deps.cmd.EXPECT().Config().Return(deps.config, nil).AnyTimes()
	deps.config.EXPECT().Authentication().Return(deps.auth).AnyTimes()
	deps.cmd.EXPECT().Printer("list").Return(mustListPrinter(t, out), nil).AnyTimes()
	if organization != "" {
		deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(deps.wit, nil).AnyTimes()
	}

	return deps
}

func (d *dependencies) setupDefaultOrg(org string) {
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func (d *dependencies) stubGetRelationTypes(types []workitemtracking.WorkItemRelationType) {
	d.wit.EXPECT().GetRelationTypes(gomock.Any(), gomock.Any()).Return(&types, nil).AnyTimes()
}

func (d *dependencies) stubGetWorkItem(t *testing.T, project string, targets map[int]string, source, populated *workitemtracking.WorkItem) {
	sourceFetches := 0
	d.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			require.NotNil(t, args.Project)
			assert.Equal(t, project, *args.Project)
			if url, ok := targets[*args.Id]; ok {
				fields := map[string]interface{}{wishared.TeamProjectField: project}
				return &workitemtracking.WorkItem{Id: args.Id, Url: types.ToPtr(url), Fields: &fields}, nil
			}
			sourceFetches++
			if sourceFetches > 1 && populated != nil {
				return populated, nil
			}
			return source, nil
		},
	).AnyTimes()
}

func (d *dependencies) stubUpdateWorkItem(t *testing.T, project string) *workitemtracking.UpdateWorkItemArgs {
	var captured workitemtracking.UpdateWorkItemArgs
	d.wit.EXPECT().UpdateWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.UpdateWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Project)
			assert.Equal(t, project, *args.Project)
			captured = args
			return &workitemtracking.WorkItem{Id: args.Id}, nil
		},
	).AnyTimes()
	return &captured
}

func mustListPrinter(t *testing.T, w io.Writer) printer.Printer {
	t.Helper()
	tp, err := printer.NewListPrinter(w)
	require.NoError(t, err)
	return tp
}

type captureExporter struct {
	data any
}

func (c *captureExporter) Fields() []string { return nil }
func (c *captureExporter) Write(_ *iostreams.IOStreams, data any) error {
	c.data = data
	return nil
}

var relationTypes = []workitemtracking.WorkItemRelationType{
	{Name: types.ToPtr("parent"), ReferenceName: types.ToPtr("System.LinkTypes.Hierarchy-Reverse")},
	{Name: types.ToPtr("artifact"), ReferenceName: types.ToPtr("System.ArtifactLink")},
}

func targetURL(id int) string {
	return "https://dev.azure.com/myorg/_apis/wit/workItems/" + strconv.Itoa(id)
}

func rel(relRef, url string) workitemtracking.WorkItemRelation {
	return workitemtracking.WorkItemRelation{Rel: types.ToPtr(relRef), Url: types.ToPtr(url)}
}

func sourceWorkItem(id int, project string, relations *[]workitemtracking.WorkItemRelation) *workitemtracking.WorkItem {
	fields := map[string]interface{}{wishared.TeamProjectField: project}
	return &workitemtracking.WorkItem{Id: types.ToPtr(id), Fields: &fields, Relations: relations}
}

func patchPaths(doc *[]webapi.JsonPatchOperation) []string {
	paths := make([]string, 0, len(*doc))
	for _, op := range *doc {
		paths = append(paths, types.GetValue(op.Path, ""))
	}
	return paths
}

func TestNewCmd_remove(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "remove [ORG:]PROJECT/ID", cmd.Use)
	assert.Equal(t, []string{"r", "rm"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.Args(cmd, []string{"Fabrikam/1234"}))
	assert.Error(t, cmd.Args(cmd, []string{"Fabrikam/1234", "Extra"}))
	assert.Error(t, cmd.Args(cmd, []string{}))

	f := cmd.Flags()
	for _, name := range []string{"relation-type", "target-id", "yes", "json"} {
		assert.NotNil(t, f.Lookup(name), "flag %q must exist", name)
	}
}

func Test_runRemove_minimal(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
	}), nil)
	args := deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}, yes: true})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	op := (*args.Document)[0]
	assert.Equal(t, "remove", string(types.GetValue(op.Op, webapi.Operation(""))))
	assert.Equal(t, "/relations/0", types.GetValue(op.Path, ""))
	require.NotNil(t, args.Id)
	assert.Equal(t, 1234, *args.Id)
}

func Test_runRemove_multipleTargets(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2), 3: targetURL(3)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
		rel("System.ArtifactLink", "https://example.com/1"),
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(3)),
	}), nil)
	args := deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2", "3"}, yes: true})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 2)
	assert.Equal(t, []string{"/relations/2", "/relations/0"}, patchPaths(args.Document))
}

func Test_runRemove_commaSeparated(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2), 3: targetURL(3)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
		rel("System.ArtifactLink", "https://example.com/1"),
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(3)),
	}), nil)
	args := deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2,3"}, yes: true})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 2)
	assert.Equal(t, []string{"/relations/2", "/relations/0"}, patchPaths(args.Document))
}

func Test_runRemove_noMatch(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, sourceWorkItem(1234, "Fabrikam", nil), nil)

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}, yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Id(s) supplied in --target-id is not valid")
}

func Test_runRemove_partialMatch(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2), 3: targetURL(3)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
	}), nil)

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2", "3"}, yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Id(s) supplied in --target-id is not valid")
}

func Test_runRemove_invalidRelationType(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "bogus", targetIDs: []string{"2"}, yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--relation-type is not valid. Use \"azdo boards work-item relation list-type\"")
}

func Test_runRemove_invalidIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		targetArg string
		targetIDs []string
		wantError string
	}{
		{
			name:      "non-numeric source ID",
			targetArg: "Fabrikam/abc",
			targetIDs: []string{"2"},
			wantError: "work item ID must be a positive integer; got \"abc\"",
		},
		{
			name:      "zero source ID",
			targetArg: "Fabrikam/0",
			targetIDs: []string{"2"},
			wantError: "work item ID must be a positive integer",
		},
		{
			name:      "negative source ID",
			targetArg: "Fabrikam/-5",
			targetIDs: []string{"2"},
			wantError: "work item ID must be a positive integer",
		},
		{
			name:      "non-numeric target ID",
			targetArg: "Fabrikam/1234",
			targetIDs: []string{"abc"},
			wantError: "target work item ID must be a positive integer; got \"abc\"",
		},
		{
			name:      "negative target ID",
			targetArg: "Fabrikam/1234",
			targetIDs: []string{"-3"},
			wantError: "target work item ID must be a positive integer; got \"-3\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg", false)
			deps.setupDefaultOrg("myorg")

			err := runRemove(deps.cmd, &removeOptions{targetArg: tt.targetArg, relationType: "parent", targetIDs: tt.targetIDs, yes: true})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func Test_runRemove_noTargets(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--target-id must be provided")
}

func Test_runRemove_targetIDNotFound(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found")).AnyTimes()

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}, yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve target work item 2")
	assert.Contains(t, err.Error(), "not found")
}

func Test_runRemove_confirmationPrompt_userConfirms(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", true)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
	}), nil)
	args := deps.stubUpdateWorkItem(t, "Fabrikam")
	deps.prompter.EXPECT().Confirm(
		"Are you sure you want to remove this relation(s)?",
		false,
	).Return(true, nil)

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
}

func Test_runRemove_confirmationPrompt_userDeclines(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", true)
	deps.setupDefaultOrg("myorg")
	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}})
	require.ErrorIs(t, err, util.ErrCancel)
	assert.Empty(t, deps.stdout.String())
}

func Test_runRemove_explicitOrganizationProject(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
	}), nil)
	args := deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runRemove(deps.cmd, &removeOptions{targetArg: "myorg:Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}, yes: true})
	require.NoError(t, err)
	require.NotNil(t, args.Id)
	assert.Equal(t, 1234, *args.Id)
}

func Test_runRemove_APIError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
	}), nil)
	deps.wit.EXPECT().UpdateWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom")).AnyTimes()

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}, yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update work item 1234")
	assert.Contains(t, err.Error(), "boom")
}

func Test_runRemove_success_JSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	populated := sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
	})
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, populated, populated)
	deps.stubUpdateWorkItem(t, "Fabrikam")

	exporter := &captureExporter{}
	err := runRemove(deps.cmd, &removeOptions{
		targetArg:    "Fabrikam/1234",
		relationType: "parent",
		targetIDs:    []string{"2"},
		yes:          true,
		exporter:     exporter,
	})
	require.NoError(t, err)
	assert.Same(t, populated, exporter.data)
}

func Test_runRemove_tableOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
	}), nil)
	deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}, yes: true})
	require.NoError(t, err)

	out := deps.stdout.String()
	assert.Contains(t, out, "TYPE")
	assert.Contains(t, out, "URL")
	assert.Contains(t, out, "parent")
	assert.Contains(t, out, targetURL(2))
}

func Test_runRemove_emptyRelationsAfterRemove(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
	}), sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{}))
	deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}, yes: true})
	require.NoError(t, err)
	assert.Empty(t, deps.stdout.String())
}

func Test_runRemove_relationTypeMatchIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, sourceWorkItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		rel("System.LinkTypes.Hierarchy-Reverse", targetURL(2)),
	}), nil)
	args := deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "PARENT", targetIDs: []string{"2"}, yes: true})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "/relations/0", types.GetValue((*args.Document)[0].Path, ""))
}

func Test_runRemove_projectMismatchSource(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			if *args.Id == 2 {
				fields := map[string]interface{}{wishared.TeamProjectField: "Fabrikam"}
				return &workitemtracking.WorkItem{Id: args.Id, Url: types.ToPtr(targetURL(2)), Fields: &fields}, nil
			}
			fields := map[string]interface{}{wishared.TeamProjectField: "OtherProject"}
			return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
		},
	).AnyTimes()

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}, yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `work item 1234 does not belong to project "Fabrikam"`)
}

func Test_runRemove_projectMismatchTarget(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg", false)
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			fields := map[string]interface{}{wishared.TeamProjectField: "OtherProject"}
			return &workitemtracking.WorkItem{Id: args.Id, Url: types.ToPtr(targetURL(2)), Fields: &fields}, nil
		},
	).AnyTimes()

	err := runRemove(deps.cmd, &removeOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}, yes: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `target work item 2 does not belong to project "Fabrikam"`)
}
