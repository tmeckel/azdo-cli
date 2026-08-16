package add

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

func (d *dependencies) stubGetWorkItem(t *testing.T, project string, targetIDs map[int]string, populated *workitemtracking.WorkItem) {
	d.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			require.NotNil(t, args.Project)
			assert.Equal(t, project, *args.Project)
			fields := map[string]interface{}{wishared.TeamProjectField: project}
			if url, ok := targetIDs[*args.Id]; ok {
				return &workitemtracking.WorkItem{Id: args.Id, Url: types.ToPtr(url), Fields: &fields}, nil
			}
			if populated.Fields == nil {
				populated.Fields = &fields
			}
			return populated, nil
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

func docValues(doc *[]webapi.JsonPatchOperation) []map[string]any {
	values := make([]map[string]any, 0, len(*doc))
	for _, op := range *doc {
		values = append(values, op.Value.(map[string]any))
	}
	return values
}

func TestNewCmd_add(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "add [ORG:]PROJECT/ID", cmd.Use)
	assert.Equal(t, []string{"a"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.Args(cmd, []string{"Fabrikam/1234"}))
	assert.Error(t, cmd.Args(cmd, []string{"Fabrikam/1234", "Extra"}))
	assert.Error(t, cmd.Args(cmd, []string{}))

	f := cmd.Flags()
	for _, name := range []string{"relation-type", "target-id", "target-url", "json"} {
		assert.NotNil(t, f.Lookup(name), "flag %q must exist", name)
	}
	assert.Equal(t, "T", f.Lookup("target-id").Shorthand, "target-id shorthand collides with --template (-t)")
	assert.Equal(t, "u", f.Lookup("target-url").Shorthand)
}

func Test_runAdd_minimal(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, &workitemtracking.WorkItem{
		Id:        types.ToPtr(1234),
		Relations: &[]workitemtracking.WorkItemRelation{},
	})
	args := deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	op := (*args.Document)[0]
	assert.Equal(t, "add", string(types.GetValue(op.Op, webapi.Operation(""))))
	assert.Equal(t, "/relations/-", types.GetValue(op.Path, ""))
	value, ok := op.Value.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "System.LinkTypes.Hierarchy-Reverse", value["rel"])
	assert.Equal(t, targetURL(2), value["url"])
	require.NotNil(t, args.Id)
	assert.Equal(t, 1234, *args.Id)
}

func Test_runAdd_patchTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		relationType string
		targetIDs    []string
		wantURLs     []string
	}{
		{
			name:         "multiple target-id flags",
			relationType: "parent",
			targetIDs:    []string{"2", "3"},
			wantURLs:     []string{targetURL(2), targetURL(3)},
		},
		{
			name:         "comma-separated single flag",
			relationType: "parent",
			targetIDs:    []string{"2,3,4"},
			wantURLs:     []string{targetURL(2), targetURL(3), targetURL(4)},
		},
		{
			name:         "patches in input order",
			relationType: "parent",
			targetIDs:    []string{"4,2,3"},
			wantURLs:     []string{targetURL(4), targetURL(2), targetURL(3)},
		},
		{
			name:         "case-insensitive relation type",
			relationType: "PARENT",
			targetIDs:    []string{"2"},
			wantURLs:     []string{targetURL(2)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg")
			deps.setupDefaultOrg("myorg")
			deps.stubGetRelationTypes(relationTypes)
			deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2), 3: targetURL(3), 4: targetURL(4)}, &workitemtracking.WorkItem{
				Id:        types.ToPtr(1234),
				Relations: &[]workitemtracking.WorkItemRelation{},
			})
			args := deps.stubUpdateWorkItem(t, "Fabrikam")

			err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: tt.relationType, targetIDs: tt.targetIDs})
			require.NoError(t, err)

			require.NotNil(t, args.Document)
			require.Len(t, *args.Document, len(tt.wantURLs))
			values := docValues(args.Document)
			for i, want := range tt.wantURLs {
				assert.Equal(t, want, values[i]["url"])
			}
			value := (*args.Document)[0].Value.(map[string]any)
			assert.Equal(t, "System.LinkTypes.Hierarchy-Reverse", value["rel"])
		})
	}
}

func Test_runAdd_targetURLs(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", nil, &workitemtracking.WorkItem{
		Id:        types.ToPtr(1234),
		Relations: &[]workitemtracking.WorkItemRelation{},
	})
	args := deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runAdd(deps.cmd, &addOptions{
		targetArg:    "Fabrikam/1234",
		relationType: "artifact",
		targetURLs:   []string{"https://example.com/1", "https://example.com/2"},
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 2)
	values := docValues(args.Document)
	assert.Equal(t, "https://example.com/1", values[0]["url"])
	assert.Equal(t, "https://example.com/2", values[1]["url"])
}

func Test_runAdd_invalidRelationType(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "bogus", targetIDs: []string{"2"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--relation-type is not valid. Use \"azdo boards work-item relation list-type\"")
}

func Test_runAdd_invalidIDs(t *testing.T) {
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
			name:      "negative target ID",
			targetArg: "Fabrikam/1234",
			targetIDs: []string{"-3"},
			wantError: "target work item ID must be a positive integer; got \"-3\"",
		},
		{
			name:      "non-numeric target ID",
			targetArg: "Fabrikam/1234",
			targetIDs: []string{"abc"},
			wantError: "target work item ID must be a positive integer; got \"abc\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg")
			deps.setupDefaultOrg("myorg")

			err := runAdd(deps.cmd, &addOptions{targetArg: tt.targetArg, relationType: "parent", targetIDs: tt.targetIDs})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func Test_runAdd_noTargets(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--target-id or --target-url must be provided")
}

func Test_runAdd_bothTargets(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")

	err := runAdd(deps.cmd, &addOptions{
		targetArg:    "Fabrikam/1234",
		relationType: "parent",
		targetIDs:    []string{"2"},
		targetURLs:   []string{"https://example.com/1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--target-id and --target-url are mutually exclusive")
}

func Test_runAdd_targetIDNotFound(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			if *args.Id == 1234 {
				fields := map[string]interface{}{wishared.TeamProjectField: "Fabrikam"}
				return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
			}
			return nil, errors.New("not found")
		},
	).AnyTimes()

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve target work item 2")
	assert.Contains(t, err.Error(), "not found")
}

func Test_runAdd_scope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		targetArg     string
		useDefaultOrg bool
	}{
		{
			name:          "project with default organization",
			targetArg:     "Fabrikam/1234",
			useDefaultOrg: true,
		},
		{
			name:      "explicit organization prefix",
			targetArg: "myorg:Fabrikam/1234",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg")
			if tt.useDefaultOrg {
				deps.setupDefaultOrg("myorg")
			}
			deps.stubGetRelationTypes(relationTypes)
			deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, &workitemtracking.WorkItem{
				Id:        types.ToPtr(1234),
				Relations: &[]workitemtracking.WorkItemRelation{},
			})
			args := deps.stubUpdateWorkItem(t, "Fabrikam")

			err := runAdd(deps.cmd, &addOptions{targetArg: tt.targetArg, relationType: "parent", targetIDs: []string{"2"}})
			require.NoError(t, err)
			require.NotNil(t, args.Id)
			assert.Equal(t, 1234, *args.Id)
		})
	}
}

func Test_runAdd_projectMismatchSource(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			require.NotNil(t, args.Project)
			if *args.Id == 2 {
				fields := map[string]interface{}{wishared.TeamProjectField: "Fabrikam"}
				return &workitemtracking.WorkItem{Id: args.Id, Url: types.ToPtr(targetURL(2)), Fields: &fields}, nil
			}
			fields := map[string]interface{}{wishared.TeamProjectField: "OtherProject"}
			return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
		},
	).AnyTimes()

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `work item 1234 does not belong to project "Fabrikam"`)
}

func Test_run_add_projectMismatchTarget(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			require.NotNil(t, args.Project)
			if *args.Id == 1234 {
				fields := map[string]interface{}{wishared.TeamProjectField: "Fabrikam"}
				return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
			}
			fields := map[string]interface{}{wishared.TeamProjectField: "OtherProject"}
			return &workitemtracking.WorkItem{Id: args.Id, Url: types.ToPtr(targetURL(2)), Fields: &fields}, nil
		},
	).AnyTimes()

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `target work item 2 does not belong to project "Fabrikam"`)
}

func Test_runAdd_targetIDForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		targetIDs   []string
		wantProject map[int]string // id -> expected fetch project
		wantError   string
	}{
		{
			name:      "bare ID resolves in scope project",
			targetIDs: []string{"42"},
			wantProject: map[int]string{
				42: "Fabrikam",
			},
		},
		{
			name:      "non-numeric bare ID",
			targetIDs: []string{"abc"},
			wantError: `target work item ID must be a positive integer; got "abc"`,
		},
		{
			name:      "non-numeric project prefixed ID",
			targetIDs: []string{"Contoso/abc"},
			wantError: `target work item ID must be a positive integer; got "abc"`,
		},
		{
			name:      "negative project prefixed ID",
			targetIDs: []string{"Contoso/-3"},
			wantError: "target work item ID must be a positive integer",
		},
		{
			name:      "legacy org slash form",
			targetIDs: []string{"myorg/Fabrikam/42"},
			wantError: "organization is not allowed",
		},
		{
			name:      "org prefixed colon form",
			targetIDs: []string{"myorg:Fabrikam/42"},
			wantError: "organization is not allowed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg")
			deps.setupDefaultOrg("myorg")
			deps.stubGetRelationTypes(relationTypes)
			if tt.wantError == "" {
				deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
						require.NotNil(t, args.Id)
						require.NotNil(t, args.Project)
						if *args.Id == 1234 {
							assert.Equal(t, "Fabrikam", *args.Project)
							fields := map[string]interface{}{wishared.TeamProjectField: "Fabrikam"}
							return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
						}
						assert.Equal(t, tt.wantProject[*args.Id], *args.Project)
						fields := map[string]interface{}{wishared.TeamProjectField: tt.wantProject[*args.Id]}
						return &workitemtracking.WorkItem{Id: args.Id, Url: types.ToPtr(targetURL(*args.Id)), Fields: &fields}, nil
					},
				).AnyTimes()
				deps.stubUpdateWorkItem(t, "Fabrikam")
			}

			err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: tt.targetIDs})
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_runAdd_crossProjectTarget(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			require.NotNil(t, args.Project)
			switch *args.Id {
			case 1234:
				assert.Equal(t, "Fabrikam", *args.Project)
				fields := map[string]interface{}{wishared.TeamProjectField: "Fabrikam"}
				return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
			case 77:
				assert.Equal(t, "Contoso", *args.Project)
				fields := map[string]interface{}{wishared.TeamProjectField: "Contoso"}
				return &workitemtracking.WorkItem{Id: args.Id, Url: types.ToPtr(targetURL(77)), Fields: &fields}, nil
			default:
				t.Fatalf("unexpected work item ID %d", *args.Id)
				return nil, nil
			}
		},
	).AnyTimes()
	args := deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"Contoso/77"}})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	values := docValues(args.Document)
	assert.Equal(t, targetURL(77), values[0]["url"])
}

func Test_runAdd_crossProjectTargetMismatch(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			if *args.Id == 1234 {
				fields := map[string]interface{}{wishared.TeamProjectField: "Fabrikam"}
				return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
			}
			fields := map[string]interface{}{wishared.TeamProjectField: "YetAnother"}
			return &workitemtracking.WorkItem{Id: args.Id, Url: types.ToPtr(targetURL(77)), Fields: &fields}, nil
		},
	).AnyTimes()

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"Contoso/77"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `target work item 77 does not belong to project "Contoso"`)
}

func Test_runAdd_APIError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, &workitemtracking.WorkItem{
		Id:        types.ToPtr(1234),
		Relations: &[]workitemtracking.WorkItemRelation{},
	})
	deps.wit.EXPECT().UpdateWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom")).AnyTimes()

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update work item 1234")
	assert.Contains(t, err.Error(), "boom")
}

func Test_runAdd_success_JSON(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	populated := &workitemtracking.WorkItem{
		Id:  types.ToPtr(1234),
		Rev: types.ToPtr(2),
		Relations: &[]workitemtracking.WorkItemRelation{
			{Rel: types.ToPtr("System.LinkTypes.Hierarchy-Reverse"), Url: types.ToPtr(targetURL(2))},
		},
	}
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, populated)
	deps.stubUpdateWorkItem(t, "Fabrikam")

	exporter := &captureExporter{}
	err := runAdd(deps.cmd, &addOptions{
		targetArg:    "Fabrikam/1234",
		relationType: "parent",
		targetIDs:    []string{"2"},
		exporter:     exporter,
	})
	require.NoError(t, err)
	assert.Same(t, populated, exporter.data)
}

func Test_runAdd_tableOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, &workitemtracking.WorkItem{
		Id: types.ToPtr(1234),
		Relations: &[]workitemtracking.WorkItemRelation{
			{Rel: types.ToPtr("System.LinkTypes.Hierarchy-Reverse"), Url: types.ToPtr(targetURL(2))},
			{Rel: types.ToPtr("System.ArtifactLink"), Url: types.ToPtr("https://example.com/1")},
		},
	})
	deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}})
	require.NoError(t, err)

	out := deps.stdout.String()
	assert.Contains(t, out, "TYPE")
	assert.Contains(t, out, "URL")
	assert.Contains(t, out, "parent")
	assert.Contains(t, out, targetURL(2))
	assert.Contains(t, out, "artifact")
	assert.Contains(t, out, "https://example.com/1")
}

func Test_runAdd_emptyRelations(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubGetRelationTypes(relationTypes)
	deps.stubGetWorkItem(t, "Fabrikam", map[int]string{2: targetURL(2)}, &workitemtracking.WorkItem{
		Id:        types.ToPtr(1234),
		Relations: nil,
	})
	deps.stubUpdateWorkItem(t, "Fabrikam")

	err := runAdd(deps.cmd, &addOptions{targetArg: "Fabrikam/1234", relationType: "parent", targetIDs: []string{"2"}})
	require.NoError(t, err)
	assert.Empty(t, deps.stdout.String())
}
