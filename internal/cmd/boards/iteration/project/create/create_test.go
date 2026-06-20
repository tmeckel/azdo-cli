package create

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type fakeCreateDeps struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	wit        *mocks.MockWorkItemTrackingClient
	stdout     *bytes.Buffer
	org        string
}

func setupFakeDeps(t *testing.T, defaultOrganization string) *fakeCreateDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &fakeCreateDeps{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		stdout:     out,
		org:        defaultOrganization,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)
	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return(defaultOrganization, nil).AnyTimes()

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	return deps
}

var minimalCreatedNode = &workitemtracking.WorkItemClassificationNode{
	Id:          types.ToPtr(42),
	Name:        types.ToPtr("Sprint 1"),
	Path:        types.ToPtr("Fabrikam\\Iteration\\Sprint 1"),
	HasChildren: types.ToPtr(true),
	Attributes: &map[string]any{
		"startDate":  "2025-01-06T00:00:00Z",
		"finishDate": "2025-01-19T00:00:00Z",
	},
}

func TestNewCmd_RegistersAsCreateLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)

	assert.Equal(t, "create", cmd.Name())
	assert.Equal(t, []string{"c", "cr"}, cmd.Aliases)
	assert.True(t, strings.HasPrefix(cmd.Use, "create [ORGANIZATION/]PROJECT"))
}

func TestNewCmd_TargetArgRequired(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs(nil)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "target argument required")
}

func TestRunCreate_InvalidTarget(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org"}

	err := runCreate(deps.cmd, opts)

	requireFlagError(t, err, "expected 2-66 segments")
}

func TestRunCreate_RootLevelCreate(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/Sprint 1"}

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode)

	require.NoError(t, err)
	require.NotNil(t, args.PostedNode)
	require.NotNil(t, args.Path)
	assert.Equal(t, "Fabrikam", *args.Path)
	assert.Equal(t, workitemtracking.TreeStructureGroupValues.Iterations, *args.StructureGroup)
	assert.Equal(t, "org", *args.Project)
	assert.Equal(t, "Sprint 1", *args.PostedNode.Name)
	assert.Nil(t, args.PostedNode.Id)
	assert.Nil(t, args.PostedNode.Attributes)
}

func TestRunCreate_PathParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scopeArg string
		wantPath string
		wantName string
	}{
		{name: "nested path", scopeArg: "org/Fabrikam/Release 2025/Sprint 2", wantPath: "Release%202025", wantName: "Sprint 2"},
		{name: "normalizes repeated root segments", scopeArg: "org/Fabrikam/Fabrikam/Iteration/Release 2025/Sprint 1/Sprint 2", wantPath: "Release%202025/Sprint%201", wantName: "Sprint 2"},
		{name: "url escapes path segments", scopeArg: "org/Fabrikam/My Sprint/Sub Sprint/Sprint 2", wantPath: "My%20Sprint/Sub%20Sprint", wantName: "Sprint 2"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := setupFakeDeps(t, "org")
			args, err := captureCreateArgs(t, deps, &createOptions{scopeArg: tc.scopeArg}, minimalCreatedNode)

			require.NoError(t, err)
			require.NotNil(t, args.Path)
			assert.Equal(t, tc.wantPath, *args.Path)
			assert.Equal(t, tc.wantName, *args.PostedNode.Name)
		})
	}
}

func TestRunCreate_DateAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		startDate  string
		finishDate string
		want       map[string]any
	}{
		{name: "start only", startDate: "2025-01-06", want: map[string]any{"startDate": "2025-01-06T00:00:00Z"}},
		{name: "finish only", finishDate: "2025-01-19T00:00:00Z", want: map[string]any{"finishDate": "2025-01-19T00:00:00Z"}},
		{name: "both dates", startDate: "2025-01-06T00:00:00Z", finishDate: "2025-01-19T00:00:00Z", want: map[string]any{"startDate": "2025-01-06T00:00:00Z", "finishDate": "2025-01-19T00:00:00Z"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := setupFakeDeps(t, "org")
			args, err := captureCreateArgs(t, deps, &createOptions{
				scopeArg:   "org/Fabrikam/Sprint 1",
				startDate:  tc.startDate,
				finishDate: tc.finishDate,
			}, minimalCreatedNode)

			require.NoError(t, err)
			require.NotNil(t, args.PostedNode.Attributes)
			assert.Equal(t, tc.want, *args.PostedNode.Attributes)
		})
	}
}

func TestRunCreate_DateFlags_InvalidFormat(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/Sprint 1", startDate: "yesterday"}

	err := runCreate(deps.cmd, opts)

	requireFlagError(t, err, "invalid --start-date")
}

func TestRunCreate_DateFlags_FinishBeforeStart(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{
		scopeArg:   "org/Fabrikam/Sprint 1",
		startDate:  "2025-01-19",
		finishDate: "2025-01-06",
	}

	err := runCreate(deps.cmd, opts)

	requireFlagError(t, err, "--finish-date must be on or after --start-date")
}

func TestRunCreate_AttributesFlagMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		startDate  string
		attributes []string
		want       map[string]any
	}{
		{name: "merges custom attributes", attributes: []string{"goal=Ship", "team=Alpha"}, want: map[string]any{"goal": "Ship", "team": "Alpha"}},
		{name: "start date flag wins", startDate: "2025-01-06", attributes: []string{"startDate=2024-12-01"}, want: map[string]any{"startDate": "2025-01-06T00:00:00Z"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := setupFakeDeps(t, "org")
			args, err := captureCreateArgs(t, deps, &createOptions{
				scopeArg:   "org/Fabrikam/Sprint 1",
				startDate:  tc.startDate,
				attributes: tc.attributes,
			}, minimalCreatedNode)

			require.NoError(t, err)
			require.NotNil(t, args.PostedNode.Attributes)
			assert.Equal(t, tc.want, *args.PostedNode.Attributes)
		})
	}
}

func TestRunCreate_AttributesFlag_InvalidFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		attributes []string
		want       string
	}{
		{name: "empty key", attributes: []string{"=value"}, want: `invalid --attributes "=value": expected key=value`},
		{name: "missing equals", attributes: []string{"novalue"}, want: `invalid --attributes "novalue": expected key=value`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := setupFakeDeps(t, "org")
			opts := &createOptions{scopeArg: "org/Fabrikam/Sprint 1", attributes: tc.attributes}

			err := runCreate(deps.cmd, opts)

			requireFlagError(t, err, tc.want)
		})
	}
}

func TestRunCreate_ClientFactoryError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "default-org")
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), "default-org").Return(nil, errors.New("boom"))

	err := runCreate(deps.cmd, &createOptions{scopeArg: "Fabrikam/Sprint 1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get classification client")
}

func TestRunCreate_SDKError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/Sprint 1"}
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), "org").Return(deps.wit, nil).AnyTimes()
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runCreate(deps.cmd, opts)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create iteration")
}

func TestRunCreate_TableOutput_AllColumns(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/Sprint 1"}
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), "org").Return(deps.wit, nil).AnyTimes()
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Return(minimalCreatedNode, nil)

	err := runCreate(deps.cmd, opts)

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(deps.stdout.String()), "\n")
	require.Len(t, lines, 1)
	fields := strings.Split(lines[0], "\t")
	assert.Equal(t, []string{
		"42",
		"Sprint 1",
		"Fabrikam/Iteration/Sprint 1",
		"2025-01-06T00:00:00Z",
		"2025-01-19T00:00:00Z",
		"true",
	}, fields)
}

func TestRunCreate_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/Sprint 1", exporter: util.NewJSONExporter()}
	identifier := uuid.New()
	path := "Fabrikam\\Iteration\\Sprint 1"
	jsonNode := &workitemtracking.WorkItemClassificationNode{
		Id:            types.ToPtr(42),
		Identifier:    &identifier,
		Name:          types.ToPtr("Sprint 1"),
		Path:          &path,
		HasChildren:   types.ToPtr(true),
		Attributes:    &map[string]any{"startDate": "2025-01-06T00:00:00Z", "finishDate": "2025-01-19T00:00:00Z"},
		StructureType: types.ToPtr(workitemtracking.TreeNodeStructureTypeValues.Iteration),
		Url:           types.ToPtr("https://dev.azure.com/org/Fabrikam/_apis/wit/classificationNodes/iterations/42"),
		Links: map[string]any{
			"self": map[string]any{"href": "https://dev.azure.com/org/Fabrikam/_apis/wit/classificationNodes/iterations/42"},
		},
	}
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), "org").Return(deps.wit, nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Return(jsonNode, nil)

	err := runCreate(deps.cmd, opts)

	require.NoError(t, err)
	var got struct {
		ID            int                    `json:"id"`
		Identifier    string                 `json:"identifier"`
		Name          string                 `json:"name"`
		Path          string                 `json:"path"`
		HasChildren   bool                   `json:"hasChildren"`
		URL           string                 `json:"url"`
		StructureType string                 `json:"structureType"`
		Attributes    map[string]any         `json:"attributes"`
		Links         map[string]any         `json:"_links"`
	}
	require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &got))
	assert.Equal(t, 42, got.ID)
	assert.Equal(t, identifier.String(), got.Identifier)
	assert.Equal(t, "Sprint 1", got.Name)
	assert.Equal(t, "Fabrikam\\Iteration\\Sprint 1", got.Path)
	assert.True(t, got.HasChildren)
	assert.Equal(t, "https://dev.azure.com/org/Fabrikam/_apis/wit/classificationNodes/iterations/42", got.URL)
	assert.Equal(t, "iteration", got.StructureType)
	assert.Equal(t, map[string]any{"startDate": "2025-01-06T00:00:00Z", "finishDate": "2025-01-19T00:00:00Z"}, got.Attributes)
	assert.Contains(t, got.Links, "self")
}

func TestRunCreate_OrganizationFromConfigDefault(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "default-org")
	opts := &createOptions{scopeArg: "Fabrikam/Sprint 1"}
	var got workitemtracking.CreateOrUpdateClassificationNodeArgs
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), "default-org").Return(deps.wit, nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.CreateOrUpdateClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			got = args
			return minimalCreatedNode, nil
		},
	)

	err := runCreate(deps.cmd, opts)

	require.NoError(t, err)
	assert.Equal(t, "Fabrikam", *got.Project)
}

func captureCreateArgs(t *testing.T, deps *fakeCreateDeps, opts *createOptions, response *workitemtracking.WorkItemClassificationNode) (workitemtracking.CreateOrUpdateClassificationNodeArgs, error) {
	t.Helper()

	var got workitemtracking.CreateOrUpdateClassificationNodeArgs
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), deps.org).Return(deps.wit, nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.CreateOrUpdateClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			got = args
			return response, nil
		},
	)

	err := runCreate(deps.cmd, opts)
	return got, err
}

func requireFlagError(t *testing.T, err error, substr string) {
	t.Helper()

	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), substr)
}
