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

func setupFakeDeps(t *testing.T, organization string) *fakeCreateDeps {
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
		org:        organization,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)
	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return(organization, nil).AnyTimes()

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	return deps
}

func setupFakeDepsWithDefaultOrg(t *testing.T, defaultOrg string) *fakeCreateDeps {
	t.Helper()

	deps := setupFakeDeps(t, defaultOrg)
	cfg := mocks.NewMockConfig(deps.ctrl)
	auth := mocks.NewMockAuthConfig(deps.ctrl)

	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return(defaultOrg, nil).AnyTimes()

	return deps
}

func minimalCreatedNode() *workitemtracking.WorkItemClassificationNode {
	attrs := map[string]any{
		"startDate":  "2025-01-06T00:00:00Z",
		"finishDate": "2025-01-19T00:00:00Z",
	}
	id := 42
	hasChildren := true
	name := "Sprint 1"
	path := "Fabrikam\\Iteration\\Sprint 1"
	return &workitemtracking.WorkItemClassificationNode{
		Id:          &id,
		Name:        &name,
		Path:        &path,
		HasChildren: &hasChildren,
		Attributes:  &attrs,
	}
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

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())

	require.NoError(t, err)
	require.NotNil(t, args.PostedNode)
	require.NotNil(t, args.Path)
	assert.Equal(t, "Fabrikam", *args.Path)
	assert.Equal(t, workitemtracking.TreeStructureGroupValues.Iterations, *args.StructureGroup)
	assert.Equal(t, "iterations", string(*args.StructureGroup))
	assert.Equal(t, "org", *args.Project)
	assert.Equal(t, "Sprint 1", *args.PostedNode.Name)
	assert.Nil(t, args.PostedNode.Id)
	assert.Nil(t, args.PostedNode.Attributes)
}

func TestRunCreate_NestedPathCreate(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/Release 2025/Sprint 2"}

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())

	require.NoError(t, err)
	require.NotNil(t, args.Path)
	assert.Equal(t, "Release%202025", *args.Path)
	assert.Equal(t, "Sprint 2", *args.PostedNode.Name)
}

func TestRunCreate_PathNormalizationStripsProjectAndIteration(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/Fabrikam/Iteration/Release 2025/Sprint 1/Sprint 2"}

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())

	require.NoError(t, err)
	require.NotNil(t, args.Path)
	assert.Equal(t, "Release%202025/Sprint%201", *args.Path)
	assert.Equal(t, "Sprint 2", *args.PostedNode.Name)
}

func TestRunCreate_PathURLEscaping(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/My Sprint/Sub Sprint/Sprint 2"}

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())

	require.NoError(t, err)
	require.NotNil(t, args.Path)
	assert.Equal(t, "My%20Sprint/Sub%20Sprint", *args.Path)
	assert.Equal(t, "Sprint 2", *args.PostedNode.Name)
}

func TestRunCreate_StartDateOnly(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/Sprint 1", startDate: "2025-01-06"}

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())

	require.NoError(t, err)
	require.NotNil(t, args.PostedNode.Attributes)
	assert.Equal(t, "2025-01-06T00:00:00Z", (*args.PostedNode.Attributes)["startDate"])
	assert.NotContains(t, *args.PostedNode.Attributes, "finishDate")
}

func TestRunCreate_FinishDateOnly(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{scopeArg: "org/Fabrikam/Sprint 1", finishDate: "2025-01-19T00:00:00Z"}

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())

	require.NoError(t, err)
	require.NotNil(t, args.PostedNode.Attributes)
	assert.Equal(t, "2025-01-19T00:00:00Z", (*args.PostedNode.Attributes)["finishDate"])
	assert.NotContains(t, *args.PostedNode.Attributes, "startDate")
}

func TestRunCreate_BothDates_RFC3339(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{
		scopeArg:   "org/Fabrikam/Sprint 1",
		startDate:  "2025-01-06T00:00:00Z",
		finishDate: "2025-01-19T00:00:00Z",
	}

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())

	require.NoError(t, err)
	require.NotNil(t, args.PostedNode.Attributes)
	assert.Equal(t, "2025-01-06T00:00:00Z", (*args.PostedNode.Attributes)["startDate"])
	assert.Equal(t, "2025-01-19T00:00:00Z", (*args.PostedNode.Attributes)["finishDate"])
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

func TestRunCreate_AttributesFlag_Merged(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{
		scopeArg:   "org/Fabrikam/Sprint 1",
		attributes: []string{"goal=Ship", "team=Alpha"},
	}

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())

	require.NoError(t, err)
	require.NotNil(t, args.PostedNode.Attributes)
	assert.Equal(t, "Ship", (*args.PostedNode.Attributes)["goal"])
	assert.Equal(t, "Alpha", (*args.PostedNode.Attributes)["team"])
}

func TestRunCreate_AttributesFlag_StartDateWins(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org")
	opts := &createOptions{
		scopeArg:   "org/Fabrikam/Sprint 1",
		startDate:  "2025-01-06",
		attributes: []string{"startDate=2024-12-01"},
	}

	args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())

	require.NoError(t, err)
	require.NotNil(t, args.PostedNode.Attributes)
	assert.Equal(t, "2025-01-06T00:00:00Z", (*args.PostedNode.Attributes)["startDate"])
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

func TestRunCreate_ProjectScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		scopeArg   string
		org        string
		project    string
		wantErr    string
		defaultOrg string
	}{
		{name: "organization and project", scopeArg: "org/proj/Sprint 1", org: "org", project: "org"},
		{name: "project uses default organization", scopeArg: "proj/Sprint 1", org: "default-org", project: "proj", defaultOrg: "default-org"},
		{name: "variable targets stay in parent path", scopeArg: "org/proj/release/Sprint 1", org: "org", project: "proj"},
		{name: "empty scope", scopeArg: "", wantErr: "expected"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var deps *fakeCreateDeps
			if tc.defaultOrg != "" {
				deps = setupFakeDepsWithDefaultOrg(t, tc.defaultOrg)
			} else {
				deps = setupFakeDeps(t, tc.org)
			}
			opts := &createOptions{scopeArg: tc.scopeArg}

			if tc.wantErr != "" {
				err := runCreate(deps.cmd, opts)
				requireFlagError(t, err, tc.wantErr)
				return
			}

			args, err := captureCreateArgs(t, deps, opts, minimalCreatedNode())
			require.NoError(t, err)
			assert.Equal(t, tc.project, *args.Project)
		})
	}
}

func TestRunCreate_ClientFactoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	cmd := mocks.NewMockCmdContext(ctrl)
	clientFact := mocks.NewMockClientFactory(ctrl)

	cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmd.EXPECT().ClientFactory().Return(clientFact).AnyTimes()
	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)
	cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return("default-org", nil).AnyTimes()
	clientFact.EXPECT().WorkItemTracking(gomock.Any(), "default-org").Return(nil, errors.New("boom"))

	err := runCreate(cmd, &createOptions{scopeArg: "org/Fabrikam/Sprint 1"})

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
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Return(minimalCreatedNode(), nil)

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
	var got map[string]any
	require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &got))
	assert.Equal(t, float64(42), got["id"])
	assert.Equal(t, identifier.String(), got["identifier"])
	assert.Equal(t, "Sprint 1", got["name"])
	assert.Equal(t, "Fabrikam\\Iteration\\Sprint 1", got["path"])
	assert.Equal(t, true, got["hasChildren"])
	assert.Equal(t, "https://dev.azure.com/org/Fabrikam/_apis/wit/classificationNodes/iterations/42", got["url"])
	assert.Equal(t, "iteration", got["structureType"])
	attrs, ok := got["attributes"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2025-01-06T00:00:00Z", attrs["startDate"])
	assert.Equal(t, "2025-01-19T00:00:00Z", attrs["finishDate"])
	links, ok := got["_links"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, links, "self")
}

func TestRunCreate_OrganizationFromConfigDefault(t *testing.T) {
	t.Parallel()

	deps := setupFakeDepsWithDefaultOrg(t, "default-org")
	opts := &createOptions{scopeArg: "Fabrikam/Sprint 1"}
	var got workitemtracking.CreateOrUpdateClassificationNodeArgs
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), "default-org").Return(deps.wit, nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.CreateOrUpdateClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			got = args
			return minimalCreatedNode(), nil
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
