package show

import (
	"bytes"
	"context"
	"errors"
	"io"
	"regexp"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	wishared "github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	wit        *mocks.MockWorkItemTrackingClient
	config     *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func newDependencies(t *testing.T, organization string) *dependencies {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	deps := &dependencies{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		auth:       mocks.NewMockAuthConfig(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
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

func (d *dependencies) stubGetWorkItem(t *testing.T, project string, wi *workitemtracking.WorkItem, targets ...map[int]*workitemtracking.WorkItem) {
	var targetMap map[int]*workitemtracking.WorkItem
	if len(targets) > 0 {
		targetMap = targets[0]
	}
	d.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			require.NotNil(t, args.Project)
			assert.Equal(t, project, *args.Project)
			if twi, ok := targetMap[*args.Id]; ok {
				require.NotNil(t, args.Fields)
				return twi, nil
			}
			require.NotNil(t, args.Expand)
			assert.Equal(t, workitemtracking.WorkItemExpandValues.All, *args.Expand)
			return wi, nil
		},
	).AnyTimes()
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

func workItem(id int, project string, relations *[]workitemtracking.WorkItemRelation) *workitemtracking.WorkItem {
	fields := map[string]interface{}{wishared.TeamProjectField: project}
	return &workitemtracking.WorkItem{Id: types.ToPtr(id), Fields: &fields, Relations: relations}
}

func targetWorkItem(id int, project, title string) *workitemtracking.WorkItem {
	fields := map[string]interface{}{
		wishared.TeamProjectField: project,
		"System.Title":            title,
	}
	return &workitemtracking.WorkItem{Id: types.ToPtr(id), Fields: &fields}
}

func TestNewCmd_show(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "show [ORG:]PROJECT/ID", cmd.Use)
	assert.Equal(t, []string{"s"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.Args(cmd, []string{"Fabrikam/1234"}))
	assert.Error(t, cmd.Args(cmd, []string{"Fabrikam/1234", "Extra"}))
	assert.Error(t, cmd.Args(cmd, []string{}))

	for _, name := range []string{"json"} {
		assert.NotNil(t, cmd.Flags().Lookup(name), "flag %q must exist", name)
	}
}

func Test_runShow_tableOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", workItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		{Rel: types.ToPtr("System.LinkTypes.Hierarchy-Reverse"), Url: types.ToPtr("https://dev.azure.com/myorg/Contoso/_apis/wit/workItems/77")},
		{Rel: types.ToPtr("System.ArtifactLink"), Url: types.ToPtr("https://example.com/1")},
	}), map[int]*workitemtracking.WorkItem{
		77: targetWorkItem(77, "Contoso", "Deploy the fix"),
	})

	err := runShow(deps.cmd, &showOptions{targetArg: "Fabrikam/1234"})
	require.NoError(t, err)

	out := deps.stdout.String()
	for _, hdr := range []string{"TYPE", "ORGANIZATION", "PROJECT", "ID", "TITLE"} {
		assert.Contains(t, out, hdr)
	}
	assert.Contains(t, out, "parent")
	assert.Contains(t, out, "artifact")
	assert.Contains(t, out, "Contoso")
	assert.Contains(t, out, "77")
	assert.Contains(t, out, "Deploy the fix")
	assert.Contains(t, out, "https://example.com/1")
	assert.NotContains(t, out, "URL:")
}

func Test_runShow_artifactRelationEmptyID(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", workItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		{Rel: types.ToPtr("System.ArtifactLink"), Url: types.ToPtr("https://example.com/1")},
	}))

	err := runShow(deps.cmd, &showOptions{targetArg: "Fabrikam/1234"})
	require.NoError(t, err)

	out := deps.stdout.String()
	// Artifact relations carry no work item ID: the ID cell renders empty,
	// never a literal "0".
	assert.Regexp(t, regexp.MustCompile(`(?m)^ID:\s*$`), out)
	assert.NotRegexp(t, regexp.MustCompile(`(?m)^ID:\s*0\s*$`), out)
	assert.Contains(t, out, "https://example.com/1")
}

func Test_runShow_remoteLinkFallback(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			if *args.Id == 1234 {
				fields := map[string]interface{}{wishared.TeamProjectField: "Fabrikam"}
				return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields, Relations: &[]workitemtracking.WorkItemRelation{
					{Rel: types.ToPtr("System.LinkTypes.Hierarchy-Reverse"), Url: types.ToPtr("https://dev.azure.com/otherorg/Proj/_apis/wit/workItems/42")},
				}}, nil
			}
			return nil, errors.New("not found")
		},
	).AnyTimes()

	err := runShow(deps.cmd, &showOptions{targetArg: "Fabrikam/1234"})
	require.NoError(t, err)

	out := deps.stdout.String()
	assert.Contains(t, out, "otherorg")
	assert.Contains(t, out, "Proj")
	assert.Contains(t, out, "42")
}

func Test_runShow_emptyRelations(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", workItem(1234, "Fabrikam", nil))

	err := runShow(deps.cmd, &showOptions{targetArg: "Fabrikam/1234"})
	require.NoError(t, err)
	assert.Empty(t, deps.stdout.String())
}

func Test_runShow_explicitOrganizationProject(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", workItem(1234, "Fabrikam", nil))

	err := runShow(deps.cmd, &showOptions{targetArg: "myorg:Fabrikam/1234"})
	require.NoError(t, err)
}

func Test_runShow_success_JSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	wi := workItem(1234, "Fabrikam", &[]workitemtracking.WorkItemRelation{
		{Rel: types.ToPtr("System.LinkTypes.Hierarchy-Reverse"), Url: types.ToPtr("https://dev.azure.com/2")},
	})
	deps.stubGetWorkItem(t, "Fabrikam", wi)

	exporter := &captureExporter{}
	err := runShow(deps.cmd, &showOptions{targetArg: "Fabrikam/1234", exporter: exporter})
	require.NoError(t, err)
	assert.Same(t, wi, exporter.data)
}

func Test_runShow_invalidIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		targetArg string
		wantError string
	}{
		{
			name:      "non-numeric ID",
			targetArg: "Fabrikam/abc",
			wantError: `work item ID must be a positive integer; got "abc"`,
		},
		{
			name:      "zero ID",
			targetArg: "Fabrikam/0",
			wantError: "work item ID must be a positive integer",
		},
		{
			name:      "negative ID",
			targetArg: "Fabrikam/-5",
			wantError: "work item ID must be a positive integer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg")
			deps.setupDefaultOrg("myorg")

			err := runShow(deps.cmd, &showOptions{targetArg: tt.targetArg})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func Test_runShow_getWorkItemError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom")).AnyTimes()

	err := runShow(deps.cmd, &showOptions{targetArg: "Fabrikam/1234"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get work item 1234")
	assert.Contains(t, err.Error(), "boom")
}

func Test_runShow_getRelationTypesError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetWorkItem(t, "Fabrikam", workItem(1234, "Fabrikam", nil))
	deps.wit.EXPECT().GetRelationTypes(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom")).AnyTimes()

	err := runShow(deps.cmd, &showOptions{targetArg: "Fabrikam/1234"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get relation types")
	assert.Contains(t, err.Error(), "boom")
}

func Test_runShow_projectMismatch(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetWorkItem(t, "Fabrikam", workItem(1234, "OtherProject", nil))

	err := runShow(deps.cmd, &showOptions{targetArg: "Fabrikam/1234"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `work item 1234 does not belong to project "Fabrikam"`)
}
