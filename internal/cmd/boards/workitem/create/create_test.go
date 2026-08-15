package create

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type dependencies struct {
	ctrl       *gomock.Controller
	io         *iostreams.IOStreams
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
		io:         io,
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

func (d *dependencies) setupOrgURL(org, url string) {
	d.auth.EXPECT().GetURL(org).Return(url, nil).AnyTimes()
}

func (d *dependencies) setupDefaultOrg(org string) {
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func (d *dependencies) stubCreateWorkItem(t *testing.T, project string, extraFields ...map[string]any) *workitemtracking.CreateWorkItemArgs {
	t.Helper()

	var captured workitemtracking.CreateWorkItemArgs
	d.wit.EXPECT().CreateWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.CreateWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Project)
			assert.Equal(t, project, *args.Project)
			captured = args
			fields := map[string]any{}
			for _, extra := range extraFields {
				for k, v := range extra {
					fields[k] = v
				}
			}
			return &workitemtracking.WorkItem{
				Id:     types.ToPtr(1),
				Url:    types.ToPtr("https://dev.azure.com/" + project + "/_apis/wit/workItems/1"),
				Fields: &fields,
			}, nil
		},
	)
	return &captured
}

func (d *dependencies) stubTableOutput() {
	d.cmd.EXPECT().Printer("list").Return(mustListPrinter(d.stdout), nil)
}

func mustListPrinter(w io.Writer) printer.Printer {
	tp, err := printer.NewListPrinter(w)
	if err != nil {
		panic(err)
	}
	return tp
}

func patchPaths(doc *[]webapi.JsonPatchOperation) []string {
	paths := make([]string, 0, len(*doc))
	for _, op := range *doc {
		paths = append(paths, types.GetValue(op.Path, ""))
	}
	return paths
}

func createdWorkItemFields() map[string]any {
	return map[string]any{
		"System.WorkItemType":  "Bug",
		"System.State":         "New",
		"System.Title":         "Login broken",
		"System.AssignedTo":    "ada@fabrikam.com",
		"System.AreaPath":      "Fabrikam",
		"System.IterationPath": "Fabrikam\\Sprint 1",
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

func TestNewCmd_RegistersAsCreateLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "create", cmd.Name())
	assert.Equal(t, []string{"c", "cr"}, cmd.Aliases)
	assert.True(t, strings.HasPrefix(cmd.Use, "create [ORG:]PROJECT"))
	assert.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.Args(cmd, []string{"Fabrikam"}))
	assert.Error(t, cmd.Args(cmd, []string{"Fabrikam", "Extra"}))
	assert.Error(t, cmd.Args(cmd, []string{}))

	f := cmd.Flags()
	for _, name := range []string{
		"type", "title", "description", "description-file", "description-editor", "description-format",
		"assigned-to", "area", "iteration", "tag", "priority", "severity", "parent",
		"reason", "state", "fields", "link", "bypass-rules", "suppress-notifications",
		"validate-only", "expand", "open", "discussion", "json",
	} {
		assert.NotNil(t, f.Lookup(name), "flag %q must exist", name)
	}
}

func TestRunCreate_RequiredFlagsMissing(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{"Fabrikam"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
	assert.Contains(t, err.Error(), "type")
	assert.Contains(t, err.Error(), "title")
}

func TestRunCreate_MinimalTitleAndType(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{scopeArg: "Fabrikam", workItemType: "Bug", title: "Login broken"})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	op := (*args.Document)[0]
	assert.Equal(t, webapi.OperationValues.Add, types.GetValue(op.Op, ""))
	assert.Equal(t, "/fields/System.Title", types.GetValue(op.Path, ""))
	assert.Equal(t, "Login broken", op.Value)

	assert.Contains(t, deps.stdout.String(), "1")
	assert.Contains(t, deps.stdout.String(), "Bug")
	assert.Contains(t, deps.stdout.String(), "New")
	assert.Contains(t, deps.stdout.String(), "Login broken")
}

func TestRunCreate_AllOptionalFields_CanonicalOrder(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.setupOrgURL("myorg", "https://dev.azure.com/myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		description:  "desc",
		assignedTo:   "ada@fabrikam.com",
		area:         "Web",
		iteration:    "Sprint 1",
		tags:         []string{"t1", "t2"},
		priority:     2,
		prioritySet:  true,
		severity:     "2 - High",
		parent:       42,
		parentSet:    true,
		reason:       "Investigating",
		state:        "Active",
		customFields: []string{"Foo.Bar=fb"},
		links:        []string{"related,https://example.com/1"},
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	doc := *args.Document
	require.Len(t, doc, 14)
	assert.Equal(t, []string{
		"/fields/System.Title",
		"/fields/System.Description",
		"/multilineFieldsFormat/System.Description",
		"/fields/System.AssignedTo",
		"/fields/System.AreaPath",
		"/fields/System.IterationPath",
		"/fields/System.Tags",
		"/fields/Microsoft.VSTS.Common.Priority",
		"/fields/Microsoft.VSTS.Common.Severity",
		"/fields/System.Reason",
		"/fields/System.State",
		"/fields/Foo.Bar",
		"/relations/-",
		"/relations/-",
	}, patchPaths(args.Document))

	assert.Equal(t, "desc", doc[1].Value)
	assert.Equal(t, "Markdown", doc[2].Value)
	assert.Equal(t, "t1; t2", doc[6].Value)
	assert.Equal(t, 2, doc[7].Value)
	assert.Equal(t, "fb", doc[11].Value)
	parentLink, ok := doc[12].Value.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "System.LinkTypes.Hierarchy-Reverse", parentLink["rel"])
	assert.Equal(t, "https://dev.azure.com/myorg/_apis/wit/workItems/42", parentLink["url"])
	assert.Equal(t, map[string]any{"rel": "related", "url": "https://example.com/1"}, doc[13].Value)
}

func TestRunCreate_AreaIterationRelativeSlashNormalized(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		area:         "Web/Payments",
		iteration:    "Release 2/Sprint 36",
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	doc := *args.Document
	require.Len(t, doc, 3)
	assert.Equal(t, "/fields/System.AreaPath", types.GetValue(doc[1].Path, ""))
	assert.Equal(t, `Fabrikam\Web\Payments`, doc[1].Value)
	assert.Equal(t, "/fields/System.IterationPath", types.GetValue(doc[2].Path, ""))
	assert.Equal(t, `Fabrikam\Release 2\Sprint 36`, doc[2].Value)
}

func TestRunCreate_AreaIterationRootedPathsPreserved(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		area:         `Fabrikam\Area\Voice`,
		iteration:    `fabrikam\Release 2\Sprint 36`,
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	doc := *args.Document
	require.Len(t, doc, 3)
	assert.Equal(t, `Fabrikam\Area\Voice`, doc[1].Value)
	// Already-rooted input is preserved unchanged, including casing.
	assert.Equal(t, `fabrikam\Release 2\Sprint 36`, doc[2].Value)
}

func TestRunCreate_CustomFields(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		customFields: []string{"Foo.Bar=one", "Baz.Qux=two"},
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	doc := *args.Document
	require.Len(t, doc, 3)
	assert.Equal(t, "/fields/Foo.Bar", types.GetValue(doc[1].Path, ""))
	assert.Equal(t, "one", doc[1].Value)
	assert.Equal(t, "/fields/Baz.Qux", types.GetValue(doc[2].Path, ""))
	assert.Equal(t, "two", doc[2].Value)
}

func TestRunCreate_Links(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		links:        []string{"related,https://example.com/1"},
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	doc := *args.Document
	require.Len(t, doc, 2)
	op := doc[1]
	assert.Equal(t, webapi.OperationValues.Add, types.GetValue(op.Op, ""))
	assert.Equal(t, "/relations/-", types.GetValue(op.Path, ""))
	value, ok := op.Value.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "related", value["rel"])
	assert.Equal(t, "https://example.com/1", value["url"])
}

func TestRunCreate_ParentEmitsHierarchyReverseRelation(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.setupOrgURL("myorg", "https://dev.azure.com/myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		parent:       7,
		parentSet:    true,
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	doc := *args.Document
	require.Len(t, doc, 2)
	op := doc[1]
	assert.Equal(t, "/relations/-", types.GetValue(op.Path, ""))
	value, ok := op.Value.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "System.LinkTypes.Hierarchy-Reverse", value["rel"])
	assert.Equal(t, "https://dev.azure.com/myorg/_apis/wit/workItems/7", value["url"])
}

func TestRunCreate_ParentRequiresOrgURL(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.auth.EXPECT().GetURL("myorg").Return("", errors.New("no url")).AnyTimes()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		parent:       7,
		parentSet:    true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve organization URL")
}

func TestRunCreate_DiscussionTriggersAddComment(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.wit.EXPECT().AddComment(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.AddCommentArgs) (*workitemtracking.Comment, error) {
			assert.Equal(t, 1, types.GetValue(args.WorkItemId, 0))
			require.NotNil(t, args.Request)
			assert.Equal(t, "nice work", types.GetValue(args.Request.Text, ""))
			return &workitemtracking.Comment{}, nil
		},
	)
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		discussion:   "nice work",
	})
	require.NoError(t, err)
}

func TestRunCreate_NoDiscussionSkipsAddComment(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.wit.EXPECT().AddComment(gomock.Any(), gomock.Any()).Times(0)
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
	})
	require.NoError(t, err)
}

func TestRunCreate_ProjectScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		scope   string
		project string
		wantErr bool
	}{
		{name: "project with default org", scope: "Fabrikam", project: "Fabrikam"},
		{name: "explicit org", scope: "myorg:Fabrikam", project: "Fabrikam"},
		{name: "empty scope", scope: "", wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg")
			deps.setupDefaultOrg("myorg")
			if !tt.wantErr {
				deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
				deps.stubTableOutput()
			}

			err := runCreate(deps.cmd, &createOptions{scopeArg: tt.scope, workItemType: "Bug", title: "T"})
			if tt.wantErr {
				var flagErr *util.FlagError
				require.ErrorAs(t, err, &flagErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRunCreate_InvalidProjectScope(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")

	err := runCreate(deps.cmd, &createOptions{scopeArg: "myorg/Fabrikam", workItemType: "Bug", title: "T"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")
}

func TestRunCreate_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubCreateWorkItem(t, "Fabrikam", map[string]any{"System.Title": "T"})

	exporter := &captureExporter{}
	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		exporter:     exporter,
	})
	require.NoError(t, err)

	data, err := json.Marshal(exporter.data)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":1`)
	assert.Contains(t, string(data), `"fields":{"System.Title":"T"}`)
	assert.Contains(t, string(data), `"url":"https://dev.azure.com/Fabrikam/_apis/wit/workItems/1"`)
}

func TestRunCreate_BypassRulesAndSuppressNotifications(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:              "Fabrikam",
		workItemType:          "Bug",
		title:                 "T",
		bypassRules:           true,
		suppressNotifications: true,
	})
	require.NoError(t, err)

	assert.True(t, types.GetValue(args.BypassRules, false))
	assert.True(t, types.GetValue(args.SuppressNotifications, false))
	assert.Contains(t, deps.errOut.String(), "warning: --bypass-rules/--suppress-notifications bypass work item type rules and notifications")
}

func TestRunCreate_ValidateOnly(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		validateOnly: true,
	})
	require.NoError(t, err)

	assert.True(t, types.GetValue(args.ValidateOnly, false))
	assert.Contains(t, deps.stdout.String(), "Bug")
}

func TestRunCreate_ExpandPropagation(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		expand:       "All",
	})
	require.NoError(t, err)

	require.NotNil(t, args.Expand)
	assert.Equal(t, workitemtracking.WorkItemExpand("All"), *args.Expand)
}

func TestRunCreate_MissingDefaultOrganization(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("")

	err := runCreate(deps.cmd, &createOptions{scopeArg: "Fabrikam", workItemType: "Bug", title: "T"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no organization specified")
}

func TestRunCreate_APIError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.wit.EXPECT().CreateWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runCreate(deps.cmd, &createOptions{scopeArg: "Fabrikam", workItemType: "Bug", title: "T"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestRunCreate_ClientError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "")
	deps.setupDefaultOrg("myorg")
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), "myorg").Return(nil, fmt.Errorf("no client"))

	err := runCreate(deps.cmd, &createOptions{scopeArg: "Fabrikam", workItemType: "Bug", title: "T"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create work item tracking client: no client")
}

func TestRunCreate_TagsValidation(t *testing.T) {
	t.Parallel()

	assert.NoError(t, shared.ValidateTags("--tag", []string{"a", "b"}))
	err := shared.ValidateTags("--tag", []string{"a", ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--tag value cannot be empty")
	err = shared.ValidateTags("--tag", []string{"a", "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--tag value cannot be empty")
}

func TestRunCreate_FieldsParseSplitOnFirstEquals(t *testing.T) {
	t.Parallel()

	kv, err := parseCustomFields([]string{"Foo.Bar=key=value"})
	require.NoError(t, err)
	require.Len(t, kv, 1)
	assert.Equal(t, fieldKV{ref: "Foo.Bar", value: "key=value"}, kv[0])
}

func TestRunCreate_FieldsParseMissingEquals(t *testing.T) {
	t.Parallel()

	_, err := parseCustomFields([]string{"Foo.Bar"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be in the form Ref.Name=value")
}

func TestRunCreate_LinkParseSplitOnFirstComma(t *testing.T) {
	t.Parallel()

	links, err := parseLinks([]string{"rel,url,extra"})
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, linkKV{rel: "rel", url: "url,extra"}, links[0])
}

func TestRunCreate_LinkParseMissingComma(t *testing.T) {
	t.Parallel()

	_, err := parseLinks([]string{"nocomma"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be in the form rel,url")
}

func TestRunCreate_OpenBrowserFlag(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.wit.EXPECT().CreateWorkItem(gomock.Any(), gomock.Any()).Return(
		&workitemtracking.WorkItem{Id: types.ToPtr(1)}, nil,
	)
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:      "Fabrikam",
		workItemType:  "Bug",
		title:         "T",
		openInBrowser: true,
	})
	require.NoError(t, err)
}

func TestCreateCmd_LinkAndFieldsFlags_NonCSV(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	err := cmd.ParseFlags([]string{
		"--link", "related,https://example.com/a,b",
		"--link", "parent,https://example.com/p",
		"--fields", "Foo.Bar=one,two",
		"--fields", "Baz.Qux=three",
	})
	require.NoError(t, err)

	links, err := cmd.Flags().GetStringArray("link")
	require.NoError(t, err)
	assert.Equal(t, []string{"related,https://example.com/a,b", "parent,https://example.com/p"}, links)

	fields, err := cmd.Flags().GetStringArray("fields")
	require.NoError(t, err)
	assert.Equal(t, []string{"Foo.Bar=one,two", "Baz.Qux=three"}, fields)
}

func TestCreateCmd_TagFlag_NonCSV(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	err := cmd.ParseFlags([]string{
		"--tag", "web,security",
		"--tag", "release;v1",
	})
	require.NoError(t, err)

	tags, err := cmd.Flags().GetStringArray("tag")
	require.NoError(t, err)
	// Comma/semicolon values must survive flag parsing untouched so
	// ValidateTags can reject them; CSV splitting would silently accept them.
	assert.Equal(t, []string{"web,security", "release;v1"}, tags)
}

func TestCreateCmd_DescriptionFileFlag_NonCSV(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	err := cmd.ParseFlags([]string{
		"--description-file", "notes,v1.md",
		"--description-file", "repro.md",
	})
	require.NoError(t, err)

	files, err := cmd.Flags().GetStringArray("description-file")
	require.NoError(t, err)
	assert.Equal(t, []string{"notes,v1.md", "repro.md"}, files)
}

func TestCreateCmd_PriorityZeroExplicit_Sent(t *testing.T) {
	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"Fabrikam", "--type", "Bug", "--title", "T", "--priority", "0"})
	require.NoError(t, cmd.Execute())

	require.NotNil(t, args.Document)
	doc := *args.Document
	require.Len(t, doc, 2)
	assert.Equal(t, "/fields/Microsoft.VSTS.Common.Priority", types.GetValue(doc[1].Path, ""))
	assert.Equal(t, 0, doc[1].Value)
}

func TestRunCreate_PriorityZeroUnset_Omitted(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		priority:     0,
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.Equal(t, "/fields/System.Title", types.GetValue((*args.Document)[0].Path, ""))
}

func TestCreateCmd_ParentNonPositive_Rejected(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		parent string
	}{
		{name: "zero", parent: "0"},
		{name: "negative", parent: "-5"},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg")
			deps.setupDefaultOrg("myorg")

			cmd := NewCmd(deps.cmd)
			cmd.SetArgs([]string{"Fabrikam", "--type", "Bug", "--title", "T", "--parent", tt.parent})
			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--parent must be a positive work item ID")
		})
	}
}

func TestCreateCmd_ValidateOnlyRejectsDiscussionAndOpen(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{name: "discussion", args: []string{"--validate-only", "--discussion", "hi"}, want: "--discussion cannot be combined with --validate-only"},
		{name: "open", args: []string{"--validate-only", "--open"}, want: "--open cannot be combined with --validate-only"},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "myorg")
			deps.setupDefaultOrg("myorg")

			cmd := NewCmd(deps.cmd)
			cmd.SetArgs(append([]string{"Fabrikam", "--type", "Bug", "--title", "T"}, tt.args...))
			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRunCreate_EmptyAPIResponse_Error(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.wit.EXPECT().CreateWorkItem(gomock.Any(), gomock.Any()).Return(nil, nil)

	err := runCreate(deps.cmd, &createOptions{scopeArg: "Fabrikam", workItemType: "Bug", title: "T"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

func TestCreateCmd_OpenWithJSON_OpensAfterOutput(t *testing.T) {
	t.Setenv("BROWSER", "true")

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	deps.stubCreateWorkItem(t, "Fabrikam", map[string]any{"System.Title": "T"})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"Fabrikam", "--type", "Bug", "--title", "T", "--json", "--open"})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, deps.stdout.String(), `"id":1`)
}

func TestRunCreate_DescriptionFromInline(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
		description:  "text",
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	doc := *args.Document
	require.Len(t, doc, 3)
	assert.Equal(t, "/fields/System.Description", types.GetValue(doc[1].Path, ""))
	assert.Equal(t, "text", doc[1].Value)
	assert.Equal(t, "/multilineFieldsFormat/System.Description", types.GetValue(doc[2].Path, ""))
	assert.Equal(t, "Markdown", doc[2].Value)
}

func TestRunCreate_DescriptionAbsent_OmitsPatchOp(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:     "Fabrikam",
		workItemType: "Bug",
		title:        "T",
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	require.Len(t, *args.Document, 1)
	assert.NotEqual(t, "/fields/System.Description", types.GetValue((*args.Document)[0].Path, ""))
}

func TestRunCreate_DescriptionFormatHtml(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")
	args := deps.stubCreateWorkItem(t, "Fabrikam", createdWorkItemFields())
	deps.stubTableOutput()

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:          "Fabrikam",
		workItemType:      "Bug",
		title:             "T",
		description:       "text",
		descriptionFormat: "html",
	})
	require.NoError(t, err)

	require.NotNil(t, args.Document)
	doc := *args.Document
	require.Len(t, doc, 3)
	assert.Equal(t, "/multilineFieldsFormat/System.Description", types.GetValue(doc[2].Path, ""))
	assert.Equal(t, "Html", doc[2].Value)
}

func TestRunCreate_DescriptionFormatInvalid(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "myorg")
	deps.setupDefaultOrg("myorg")

	err := runCreate(deps.cmd, &createOptions{
		scopeArg:          "Fabrikam",
		workItemType:      "Bug",
		title:             "T",
		description:       "text",
		descriptionFormat: "plaintext",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `--description-format must be "markdown" or "html"`)
}
