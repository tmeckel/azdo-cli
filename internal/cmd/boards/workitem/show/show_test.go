package show

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// ----- NewCmd structure tests -----

func TestNewCmd_RegistersAsShowLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "show", cmd.Name())
	assert.ElementsMatch(t, []string{"view", "status"}, cmd.Aliases)
	assert.True(t, strings.HasPrefix(cmd.Use, "show [ORG:]PROJECT/ID"))
	assert.NotNil(t, cmd.Flags().Lookup("comments"))
	assert.NotNil(t, cmd.Flags().Lookup("relations"))
	assert.Nil(t, cmd.Flags().Lookup("raw"))
	assert.NotNil(t, cmd.Flags().Lookup("json"))
}

func TestNewCmd_RequiresProjectTarget(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project/work item target required")
}

func TestNewCmd_TooManyArgs(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{"Fabrikam/12345", "extra"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many arguments")
}

// ----- runShow tests via gomock -----

type fakeShowDeps struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	connFact   *mocks.MockConnectionFactory
	conn       *mocks.MockConnection
	client     *mocks.MockClient
	wit        *mocks.MockWorkItemTrackingClient
	authCfg    *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func setupShowDeps(t *testing.T, organization string) *fakeShowDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeShowDeps{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		connFact:   mocks.NewMockConnectionFactory(ctrl),
		conn:       mocks.NewMockConnection(ctrl),
		client:     mocks.NewMockClient(ctrl),
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		authCfg:    mocks.NewMockAuthConfig(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().ConnectionFactory().Return(deps.connFact).AnyTimes()

	cfg := mocks.NewMockConfig(ctrl)
	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(deps.authCfg).AnyTimes()
	deps.authCfg.EXPECT().GetURL(gomock.Any()).DoAndReturn(func(org string) (string, error) {
		return "https://dev.azure.com/" + org, nil
	}).AnyTimes()
	deps.authCfg.EXPECT().GetDefaultOrganization().Return(organization, nil).AnyTimes()

	deps.connFact.EXPECT().Connection(gomock.Any()).Return(deps.conn, nil).AnyTimes()
	deps.conn.EXPECT().GetClientByUrl(gomock.Any()).Return(deps.client).AnyTimes()

	return deps
}

// stubWit wires the WorkItemTracking client factory call for tests that reach
// the SDK. Tests for factory errors omit it.
func stubWit(deps *fakeShowDeps) {
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), gomock.Any()).Return(deps.wit, nil).AnyTimes()
}

// stubWorkItem stubs GetWorkItem, asserting the required SDK arguments and
// returning the given work item.
func stubWorkItem(t *testing.T, deps *fakeShowDeps, wi *workitemtracking.WorkItem) {
	t.Helper()

	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.NotNil(t, args.Id)
			require.NotNil(t, args.Project)
			require.NotNil(t, args.Expand)
			assert.Equal(t, 12345, *args.Id)
			assert.Equal(t, "Fabrikam", *args.Project)
			assert.Equal(t, workitemtracking.WorkItemExpandValues.All, *args.Expand)
			return wi, nil
		},
	).AnyTimes()
}

// stubEnvelope stubs the low-level raw payload fetch used for format-aware
// description rendering. payload is the raw work item JSON.
func stubEnvelope(t *testing.T, deps *fakeShowDeps, payload string) {
	t.Helper()

	deps.client.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ any, _ string, _ any, _ any, _ io.Reader, _ string, _ string, _ any) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload))}, nil
		}).AnyTimes()
	deps.client.EXPECT().UnmarshalBody(gomock.Any(), gomock.Any()).
		DoAndReturn(func(resp *http.Response, v any) error {
			raw, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			return json.Unmarshal(raw, v)
		}).AnyTimes()
}

// envelopeJSON marshals the work item plus the optional multilineFieldsFormat
// map into the raw payload shape the low-level endpoint returns.
func envelopeJSON(t *testing.T, wi *workitemtracking.WorkItem, format *map[string]string) string {
	t.Helper()

	env := struct {
		*workitemtracking.WorkItem
		MultilineFieldsFormat *map[string]string `json:"multilineFieldsFormat,omitempty"`
	}{WorkItem: wi, MultilineFieldsFormat: format}
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return string(raw)
}

func sampleShowWorkItem(extraFields map[string]any) *workitemtracking.WorkItem {
	fields := map[string]any{
		"System.WorkItemType":  "Bug",
		"System.State":         "Active",
		"System.Reason":        "Investigation",
		"System.Title":         "Login broken",
		"System.TeamProject":   "Fabrikam",
		"System.AssignedTo":    map[string]any{"displayName": "Alice", "uniqueName": "alice@contoso.com"},
		"System.CreatedBy":     map[string]any{"displayName": "Bob", "uniqueName": "bob@contoso.com"},
		"System.CreatedDate":   "2024-01-15T10:30:00Z",
		"System.ChangedDate":   "2024-01-16T08:00:00Z",
		"System.AreaPath":      "Fabrikam\\Web",
		"System.IterationPath": "Fabrikam\\Release 1\\Sprint 1",
		"System.Tags":          "web; login",
	}
	for k, v := range extraFields {
		fields[k] = v
	}
	url := "https://dev.azure.com/myorg/Fabrikam/_apis/wit/workItems/12345"
	return &workitemtracking.WorkItem{
		Id:     types.ToPtr(12345),
		Rev:    types.ToPtr(3),
		Url:    &url,
		Links:  map[string]any{"self": map[string]any{"href": url}},
		Fields: &fields,
		Relations: &[]workitemtracking.WorkItemRelation{
			{Rel: types.ToPtr("System.LinkTypes.Hierarchy-Forward"), Url: types.ToPtr("https://dev.azure.com/myorg/Fabrikam/_apis/wit/workItems/12346")},
		},
		CommentVersionRef: &workitemtracking.WorkItemCommentVersionRef{
			CommentId: types.ToPtr(1),
			Version:   types.ToPtr(1),
		},
	}
}

func showOpts(scopeArg string) *showOptions {
	return &showOptions{scopeArg: scopeArg}
}

// stubTemplatePath wires everything the template rendering path needs:
// SDK work item, raw envelope payload and comments.
func stubTemplatePath(t *testing.T, deps *fakeShowDeps, wi *workitemtracking.WorkItem, format *map[string]string) {
	t.Helper()

	stubWit(deps)
	stubWorkItem(t, deps, wi)
	stubEnvelope(t, deps, envelopeJSON(t, wi, format))
}

func TestRunShow_IDMustBeInteger(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")

	err := runShow(deps.cmd, showOpts("Fabrikam/abc"))
	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), "positive integer")
}

func TestRunShow_IDMustBePositive(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"0", "-1"} {
		t.Run("id "+id, func(t *testing.T) {
			t.Parallel()

			deps := setupShowDeps(t, "org")
			err := runShow(deps.cmd, showOpts("Fabrikam/"+id))
			require.Error(t, err)
			var flagErr *util.FlagError
			require.ErrorAs(t, err, &flagErr)
			assert.Contains(t, err.Error(), "positive integer")
		})
	}
}

func TestRunShow_BasicCall(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	stubTemplatePath(t, deps, wi, nil)

	err := runShow(deps.cmd, showOpts("org:Fabrikam/12345"))
	require.NoError(t, err)
}

func TestRunShow_CommentsFlag_TriggersGetComments(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	stubTemplatePath(t, deps, wi, nil)

	deps.wit.EXPECT().GetComments(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetCommentsArgs) (*workitemtracking.CommentList, error) {
			require.NotNil(t, args.Project)
			require.NotNil(t, args.WorkItemId)
			assert.Equal(t, "Fabrikam", *args.Project)
			assert.Equal(t, 12345, *args.WorkItemId)
			return &workitemtracking.CommentList{}, nil
		},
	)

	opts := showOpts("org:Fabrikam/12345")
	opts.showComments = true
	require.NoError(t, runShow(deps.cmd, opts))
}

func TestRunShow_CommentsFlag_DefaultDoesNotCallGetComments(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	stubTemplatePath(t, deps, wi, nil)
	deps.wit.EXPECT().GetComments(gomock.Any(), gomock.Any()).Times(0)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))
	assert.NotContains(t, deps.stdout.String(), "comments:")
}

func TestRunShow_RelationsFlag_ControlsTemplateOnly(t *testing.T) {
	t.Parallel()

	// The SDK call must always use Expand=All; --relations only toggles the
	// template block.
	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	wi.Relations = &[]workitemtracking.WorkItemRelation{
		{Rel: types.ToPtr("System.LinkTypes.Hierarchy-Forward"), Url: types.ToPtr("https://dev.azure.com/myorg/Fabrikam/_apis/wit/workItems/12346")},
	}
	stubTemplatePath(t, deps, wi, nil)

	opts := showOpts("org:Fabrikam/12345")
	opts.showRelations = true
	require.NoError(t, runShow(deps.cmd, opts))
	assert.Contains(t, deps.stdout.String(), "relations:")
}

func TestRunShow_TemplateOutput_BasicFields(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(map[string]any{
		"System.Description": "<p>Investigate the login flow</p>",
	})
	stubTemplatePath(t, deps, wi, nil)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))

	out := deps.stdout.String()
	assert.Contains(t, out, "url:")
	assert.Contains(t, out, "id:")
	assert.Contains(t, out, "rev:")
	assert.Contains(t, out, "type:")
	assert.Contains(t, out, "Bug")
	assert.Contains(t, out, "Active")
	assert.Contains(t, out, "Investigation")
	assert.Contains(t, out, "Login broken")
	assert.Contains(t, out, "Alice (alice@contoso.com)")
	assert.Contains(t, out, "Bob (bob@contoso.com)")
	assert.Contains(t, out, "created on:")
	assert.Contains(t, out, "changed on:")
	assert.Contains(t, out, "Fabrikam\\Web")
	assert.Contains(t, out, "iteration:")
	assert.Contains(t, out, "Investigate the login flow")
}

func TestRunShow_TemplateOutput_Hyperlink(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	stubTemplatePath(t, deps, wi, nil)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))

	out := deps.stdout.String()
	assert.Contains(t, out, "\x1b]8;;https://dev.azure.com/myorg/Fabrikam/_apis/wit/workItems/12345\x1b\\")
}

func TestRunShow_TemplateOutput_AssignedTo_DisplayAndUnique(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(map[string]any{
		"System.AssignedTo": map[string]any{"displayName": "Alice", "uniqueName": "alice@contoso.com"},
	})
	stubTemplatePath(t, deps, wi, nil)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))
	assert.Contains(t, deps.stdout.String(), "Alice (alice@contoso.com)")
}

func TestRunShow_TemplateOutput_DescriptionMarkdownFormat(t *testing.T) {
	t.Parallel()

	// Markdown-format content passes through to the markdown template func
	// unconverted: the literal markdown markers survive glamour's notty style.
	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(map[string]any{
		"System.Description": "**Bold** intro",
	})
	format := map[string]string{"System.Description": "Markdown"}
	stubTemplatePath(t, deps, wi, &format)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))
	assert.Contains(t, deps.stdout.String(), "**Bold** intro")
}

func TestRunShow_TemplateOutput_DescriptionHtmlFormat(t *testing.T) {
	t.Parallel()

	// Html-format content is converted HTML->Markdown first, so the converted
	// markdown markers appear instead of literal HTML tags.
	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(map[string]any{
		"System.Description": "<strong>Bold</strong> intro",
	})
	format := map[string]string{"System.Description": "Html"}
	stubTemplatePath(t, deps, wi, &format)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))

	out := deps.stdout.String()
	assert.Contains(t, out, "**Bold** intro")
	assert.NotContains(t, out, "<strong>")
}

func TestRunShow_DescriptionFormatFallback(t *testing.T) {
	t.Parallel()

	// A legacy work item without the multilineFieldsFormat map is treated as
	// Html, so the description is converted.
	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(map[string]any{
		"System.Description": "<strong>Bold</strong> intro",
	})
	stubTemplatePath(t, deps, wi, nil)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))
	assert.Contains(t, deps.stdout.String(), "**Bold** intro")
}

func TestRunShow_DescriptionFormatFromPayload(t *testing.T) {
	t.Parallel()

	// The format is read from /multilineFieldsFormat/System.Description, not
	// sniffed from content: HTML-looking content marked Markdown must not be
	// converted (no markdown markers appear).
	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(map[string]any{
		"System.Description": "<strong>Bold</strong> intro",
	})
	format := map[string]string{"System.Description": "Markdown"}
	stubTemplatePath(t, deps, wi, &format)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))

	out := deps.stdout.String()
	assert.Contains(t, out, "Bold")
	assert.NotContains(t, out, "**Bold**")
}

func TestRunShow_TemplateOutput_NoDescription(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	stubTemplatePath(t, deps, wi, nil)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))
	assert.Contains(t, deps.stdout.String(), "None given")
}

func TestRunShow_TemplateOutput_Tags(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(map[string]any{"System.Tags": "tag1; tag2"})
	stubTemplatePath(t, deps, wi, nil)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))

	out := deps.stdout.String()
	assert.Contains(t, out, "tags:")
	assert.Contains(t, out, "tag1; tag2")
}

func TestRunShow_TemplateOutput_NoTags(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(map[string]any{"System.Tags": ""})
	stubTemplatePath(t, deps, wi, nil)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))
	assert.NotContains(t, deps.stdout.String(), "tags:")
}

func TestRunShow_TemplateOutput_RelationsIncluded(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	wi.Relations = &[]workitemtracking.WorkItemRelation{
		{Rel: types.ToPtr("System.LinkTypes.Hierarchy-Forward"), Url: types.ToPtr("https://dev.azure.com/myorg/Fabrikam/_apis/wit/workItems/12346")},
	}
	stubTemplatePath(t, deps, wi, nil)

	opts := showOpts("org:Fabrikam/12345")
	opts.showRelations = true
	require.NoError(t, runShow(deps.cmd, opts))

	out := deps.stdout.String()
	assert.Contains(t, out, "relations:")
	assert.Contains(t, out, "System.LinkTypes.Hierarchy-Forward")
	assert.Contains(t, out, "workItems/12346")
}

func TestRunShow_TemplateOutput_RelationsOmitted_Default(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	wi.Relations = &[]workitemtracking.WorkItemRelation{
		{Rel: types.ToPtr("System.LinkTypes.Hierarchy-Forward"), Url: types.ToPtr("https://dev.azure.com/myorg/Fabrikam/_apis/wit/workItems/12346")},
	}
	stubTemplatePath(t, deps, wi, nil)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))
	assert.NotContains(t, deps.stdout.String(), "relations:")
}

func TestRunShow_TemplateOutput_CommentsIncluded(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	stubTemplatePath(t, deps, wi, nil)

	createdDate := "2024-01-15T11:00:00Z"
	comments := []workitemtracking.Comment{
		{
			CreatedBy:   &webapi.IdentityRef{DisplayName: types.ToPtr("Alice")},
			CreatedDate: types.ToPtr(mustParseTime(t, createdDate)),
			Text:        types.ToPtr("**note** about the login"),
		},
	}

	deps.wit.EXPECT().GetComments(gomock.Any(), gomock.Any()).Return(
		&workitemtracking.CommentList{Comments: &comments}, nil,
	)

	opts := showOpts("org:Fabrikam/12345")
	opts.showComments = true
	require.NoError(t, runShow(deps.cmd, opts))

	out := deps.stdout.String()
	assert.Contains(t, out, "comments:")
	assert.Contains(t, out, "Alice")
	assert.Contains(t, out, "commented")
	assert.Contains(t, out, "**note** about the login")
}

func TestRunShow_TemplateOutput_CommentsOmitted_Default(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	stubTemplatePath(t, deps, wi, nil)
	deps.wit.EXPECT().GetComments(gomock.Any(), gomock.Any()).Times(0)

	require.NoError(t, runShow(deps.cmd, showOpts("org:Fabrikam/12345")))
	assert.NotContains(t, deps.stdout.String(), "comments:")
}

func TestRunShow_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	stubWit(deps)
	stubWorkItem(t, deps, wi)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"org:Fabrikam/12345", "--json"})
	require.NoError(t, cmd.Execute())

	out := deps.stdout.String()
	for _, key := range []string{"id", "rev", "fields", "relations", "url", "_links", "commentVersionRef"} {
		assert.Contains(t, out, `"`+key+`"`)
	}
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &parsed))
	assert.Equal(t, float64(12345), parsed["id"])
	assert.Equal(t, float64(3), parsed["rev"])
}

func TestRunShow_ProjectScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		scopeArg  string
		wantError string
	}{
		{name: "explicit org", scopeArg: "myorg:Fabrikam/12345"},
		{name: "implicit org", scopeArg: "Fabrikam/12345"},
		{name: "legacy org slash rejected", scopeArg: "org/proj/extra", wantError: "legacy ORGANIZATION"},
		{name: "empty input", scopeArg: "", wantError: "project is required"},
		{name: "missing id", scopeArg: "Fabrikam", wantError: "targets"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := setupShowDeps(t, "default-org")
			if tc.wantError == "" {
				wi := sampleShowWorkItem(nil)
				stubTemplatePath(t, deps, wi, nil)
				require.NoError(t, runShow(deps.cmd, showOpts(tc.scopeArg)))
			} else {
				err := runShow(deps.cmd, showOpts(tc.scopeArg))
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantError)
			}
		})
	}
}

func TestRunShow_InvalidProjectScope(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")

	err := runShow(deps.cmd, showOpts("org/proj/extra"))
	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported")
}

func TestRunShow_ClientFactoryError(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("factory boom"))

	err := runShow(deps.cmd, showOpts("org:Fabrikam/12345"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create work item tracking client")
}

func TestRunShow_SDKError(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	stubWit(deps)
	deps.wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("sdk boom"))

	err := runShow(deps.cmd, showOpts("org:Fabrikam/12345"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get work item")
}

func TestRunShow_GetCommentsError(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(nil)
	stubTemplatePath(t, deps, wi, nil)
	deps.wit.EXPECT().GetComments(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("comments boom"))

	opts := showOpts("org:Fabrikam/12345")
	opts.showComments = true
	err := runShow(deps.cmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get work item comments")
}

func TestRunShow_OrganizationFromConfigDefault(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "default-org")
	wi := sampleShowWorkItem(nil)

	var capturedOrg string
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, org string) (workitemtracking.Client, error) {
			capturedOrg = org
			return deps.wit, nil
		}).AnyTimes()
	stubWorkItem(t, deps, wi)
	stubEnvelope(t, deps, envelopeJSON(t, wi, nil))

	require.NoError(t, runShow(deps.cmd, showOpts("Fabrikam/12345")))
	assert.Equal(t, "default-org", capturedOrg)
}

func TestRunShow_WorkItemProjectMismatch(t *testing.T) {
	t.Parallel()

	deps := setupShowDeps(t, "org")
	wi := sampleShowWorkItem(map[string]any{"System.TeamProject": "OtherProject"})
	stubWit(deps)
	stubWorkItem(t, deps, wi)

	err := runShow(deps.cmd, showOpts("org:Fabrikam/12345"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to project")
	assert.Empty(t, deps.stdout.String())
}

func mustParseTime(t *testing.T, raw string) azuredevops.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err)
	return azuredevops.Time{Time: parsed}
}
