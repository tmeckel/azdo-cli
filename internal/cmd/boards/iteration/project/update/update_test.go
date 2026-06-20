package update

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
)

type dependencies struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	wit        *mocks.MockWorkItemTrackingClient
	stdout     *bytes.Buffer
	org        string
}

func newDependencies(t *testing.T, organization string) *dependencies {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &dependencies{
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
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(deps.wit, nil).AnyTimes()

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	return deps
}

func newDependenciesWithDefaultOrg(t *testing.T, defaultOrg string) *dependencies {
	t.Helper()

	deps := newDependencies(t, defaultOrg)
	cfg := mocks.NewMockConfig(deps.ctrl)
	auth := mocks.NewMockAuthConfig(deps.ctrl)

	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return(defaultOrg, nil).AnyTimes()

	return deps
}

func existingUpdateNode() *workitemtracking.WorkItemClassificationNode {
	id := 42
	hasChildren := true
	name := "Sprint 1"
	path := "Fabrikam\\Iteration\\Release 2025\\Sprint 1"
	attrs := map[string]any{
		"startDate":  "2025-01-06T00:00:00Z",
		"finishDate": "2025-01-19T00:00:00Z",
		"goal":       "Old goal",
		"team":       "Alpha",
	}
	return &workitemtracking.WorkItemClassificationNode{
		Id:          &id,
		Name:        &name,
		Path:        &path,
		HasChildren: &hasChildren,
		Attributes:  &attrs,
	}
}

func updatedUpdateNode() *workitemtracking.WorkItemClassificationNode {
	id := 42
	hasChildren := true
	name := "Sprint 1"
	path := "Fabrikam\\Iteration\\Release 2025\\Sprint 1"
	attrs := map[string]any{
		"startDate":  "2025-01-06T00:00:00Z",
		"finishDate": "2025-01-19T00:00:00Z",
		"goal":       "Ship login",
		"team":       "Alpha",
	}
	return &workitemtracking.WorkItemClassificationNode{
		Id:          &id,
		Name:        &name,
		Path:        &path,
		HasChildren: &hasChildren,
		Attributes:  &attrs,
	}
}

func requireFlagError(t *testing.T, err error, substr string) {
	t.Helper()

	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), substr)
}

func TestNewCmd_RegistersAsUpdateLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)

	assert.Equal(t, "update", cmd.Name())
	assert.Equal(t, []string{"u", "up"}, cmd.Aliases)
	assert.True(t, strings.HasPrefix(cmd.Use, "update [ORGANIZATION/]PROJECT[/PATH]/NAME"))
	assert.Equal(t, "id,identifier,name,path,structureType,hasChildren,attributes,url,_links", cmd.Annotations["help:json-fields"])
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

func TestRunUpdate_NoUpdateFlags(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org/Fabrikam/Release 2025/Sprint 1"})

	requireFlagError(t, err, "at least one of --start-date, --finish-date, or --attributes is required")
}

func TestRunUpdate_InvalidTarget(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org", startDate: "2025-01-06"})

	requireFlagError(t, err, "expected 2-66 segments")
}

func TestRunUpdate_RootNodeRejected(t *testing.T) {
	t.Parallel()

	deps := newDependenciesWithDefaultOrg(t, "default-org")
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(existingUpdateNode(), nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Return(updatedUpdateNode(), nil)

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org/Fabrikam/Iteration", startDate: "2025-01-06"})

	require.NoError(t, err)
}

func TestRunUpdate_RequestArgs(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	opts := &updateOptions{
		scopeArg:   "org/Fabrikam/Release 2025/Sprint 1",
		startDate:  "2025-01-06",
		finishDate: "2025-01-19",
		attributes: []string{"goal=Ship login"},
	}

	gotGet, gotUpdate, err := captureUpdateArgs(t, deps, opts, updatedUpdateNode())

	require.NoError(t, err)
	assert.Equal(t, "Fabrikam", *gotGet.Project)
	assert.Equal(t, workitemtracking.TreeStructureGroupValues.Iterations, *gotGet.StructureGroup)
	assert.Equal(t, "iterations", string(*gotGet.StructureGroup))
	assert.Equal(t, "Release%202025/Sprint%201", *gotGet.Path)

	assert.Equal(t, "Fabrikam", *gotUpdate.Project)
	assert.Equal(t, workitemtracking.TreeStructureGroupValues.Iterations, *gotUpdate.StructureGroup)
	assert.Equal(t, "Release%202025/Sprint%201", *gotUpdate.Path)
	require.NotNil(t, gotUpdate.PostedNode)
	assert.Equal(t, "Sprint 1", *gotUpdate.PostedNode.Name)
	assert.Equal(t, existingUpdateNode().Id, gotUpdate.PostedNode.Id)
	require.NotNil(t, gotUpdate.PostedNode.Attributes)
	assert.Equal(t, "2025-01-06T00:00:00Z", (*gotUpdate.PostedNode.Attributes)["startDate"])
	assert.Equal(t, "2025-01-19T00:00:00Z", (*gotUpdate.PostedNode.Attributes)["finishDate"])
	assert.Equal(t, "Ship login", (*gotUpdate.PostedNode.Attributes)["goal"])
	assert.Equal(t, "Alpha", (*gotUpdate.PostedNode.Attributes)["team"])
}

func TestRunUpdate_PreservesExistingName(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	var gotUpdate workitemtracking.CreateOrUpdateClassificationNodeArgs

	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(existingUpdateNode(), nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.CreateOrUpdateClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			gotUpdate = args
			return updatedUpdateNode(), nil
		},
	)

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org/Fabrikam/Release 2025/Sprint 1", startDate: "2025-01-06"})

	require.NoError(t, err)
	require.NotNil(t, gotUpdate.PostedNode)
	assert.Equal(t, "Sprint 1", *gotUpdate.PostedNode.Name)
}

func TestRunUpdate_ProjectScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		scopeArg   string
		org        string
		project    string
		wantErr    string
		defaultOrg string
	}{
		{name: "organization and project", scopeArg: "org/proj/release/Sprint 1", org: "org", project: "proj"},
		{name: "project uses default organization", scopeArg: "proj/Sprint 1", org: "default-org", project: "proj", defaultOrg: "default-org"},
		{name: "variable targets stay in path", scopeArg: "org/target1/target2/extra", org: "org", project: "target1"},
		{name: "empty scope", scopeArg: "", wantErr: "expected"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var deps *dependencies
			if tc.defaultOrg != "" {
				deps = newDependenciesWithDefaultOrg(t, tc.defaultOrg)
			} else {
				deps = newDependencies(t, tc.org)
			}
			opts := &updateOptions{scopeArg: tc.scopeArg, startDate: "2025-01-06"}

			if tc.wantErr != "" {
				err := runUpdate(deps.cmd, opts)
				requireFlagError(t, err, tc.wantErr)
				return
			}

			gotGet, _, err := captureUpdateArgs(t, deps, opts, updatedUpdateNode())
			require.NoError(t, err)
			assert.Equal(t, tc.project, *gotGet.Project)
		})
	}
}

func TestRunUpdate_ClientFactoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	cmd := mocks.NewMockCmdContext(ctrl)
	clientFact := mocks.NewMockClientFactory(ctrl)
	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)

	cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmd.EXPECT().ClientFactory().Return(clientFact).AnyTimes()
	cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return("default-org", nil).AnyTimes()
	clientFact.EXPECT().WorkItemTracking(gomock.Any(), "default-org").Return(nil, errors.New("boom"))

	err := runUpdate(cmd, &updateOptions{scopeArg: "org/Fabrikam/Sprint 1", startDate: "2025-01-06"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get classification client")
}

func TestRunUpdate_GetError(t *testing.T) {
	t.Parallel()

	deps := newDependenciesWithDefaultOrg(t, "default-org")
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org/Fabrikam/Sprint 1", startDate: "2025-01-06"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch iteration")
}

func TestRunUpdate_MissingExistingID(t *testing.T) {
	t.Parallel()

	deps := newDependenciesWithDefaultOrg(t, "default-org")
	node := existingUpdateNode()
	node.Id = nil
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(node, nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Times(0)

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org/Fabrikam/Sprint 1", startDate: "2025-01-06"})

	requireFlagError(t, err, "existing iteration has no ID")
}

func TestRunUpdate_UpdateError(t *testing.T) {
	t.Parallel()

	deps := newDependenciesWithDefaultOrg(t, "default-org")
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(existingUpdateNode(), nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org/Fabrikam/Sprint 1", startDate: "2025-01-06"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update iteration")
}

func TestRunUpdate_DefaultOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(existingUpdateNode(), nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Return(updatedUpdateNode(), nil)

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org/Fabrikam/Release 2025/Sprint 1", startDate: "2025-01-06", attributes: []string{"goal=Ship login"}})

	require.NoError(t, err)
	assert.Equal(t, "42\tSprint 1\tFabrikam/Iteration/Release 2025/Sprint 1\t2025-01-06T00:00:00Z\t2025-01-19T00:00:00Z\ttrue\n", deps.stdout.String())
}

func TestRunUpdate_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	identifier := uuid.New()
	structureType := workitemtracking.TreeNodeStructureTypeValues.Iteration
	url := "https://dev.azure.com/org/Fabrikam/_apis/wit/classificationNodes/iterations/42"
	node := updatedUpdateNode()
	node.Identifier = &identifier
	node.StructureType = &structureType
	node.Url = &url
	node.Links = map[string]any{"self": map[string]any{"href": url}}

	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(existingUpdateNode(), nil)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Return(node, nil)

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org/Fabrikam/Release 2025/Sprint 1", startDate: "2025-01-06", attributes: []string{"goal=Ship login"}, exporter: util.NewJSONExporter()})

	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &got))
	assert.Equal(t, float64(42), got["id"])
	assert.Equal(t, identifier.String(), got["identifier"])
	assert.Equal(t, "Sprint 1", got["name"])
	assert.Equal(t, "Fabrikam\\Iteration\\Release 2025\\Sprint 1", got["path"])
	assert.Equal(t, true, got["hasChildren"])
	assert.Equal(t, "iteration", got["structureType"])
	assert.Equal(t, url, got["url"])
	attrs, ok := got["attributes"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2025-01-06T00:00:00Z", attrs["startDate"])
	assert.Equal(t, "Ship login", attrs["goal"])
}

func TestBuildUpdateAttributes_StartDateWins(t *testing.T) {
	t.Parallel()

	existing := map[string]any{"startDate": "2024-01-01T00:00:00Z"}

	got, err := buildUpdateAttributes(&existing, "2025-01-06", "", []string{"startDate=2024-12-01"})

	require.NoError(t, err)
	assert.Equal(t, "2025-01-06T00:00:00Z", got["startDate"])
}

func TestBuildUpdateAttributes_InvalidAttribute(t *testing.T) {
	t.Parallel()

	_, err := buildUpdateAttributes(nil, "", "", []string{"novalue"})

	requireFlagError(t, err, `invalid --attributes "novalue": expected key=value`)
}

func TestBuildUpdateAttributes_InvalidDate(t *testing.T) {
	t.Parallel()

	_, err := buildUpdateAttributes(nil, "2025-01-19", "2025-01-06", nil)

	requireFlagError(t, err, "--finish-date must be on or after --start-date")
}

func captureUpdateArgs(t *testing.T, deps *dependencies, opts *updateOptions, response *workitemtracking.WorkItemClassificationNode) (workitemtracking.GetClassificationNodeArgs, workitemtracking.CreateOrUpdateClassificationNodeArgs, error) {
	t.Helper()

	var gotGet workitemtracking.GetClassificationNodeArgs
	var gotUpdate workitemtracking.CreateOrUpdateClassificationNodeArgs
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			gotGet = args
			return existingUpdateNode(), nil
		},
	)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.CreateOrUpdateClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			gotUpdate = args
			return response, nil
		},
	)

	err := runUpdate(deps.cmd, opts)
	return gotGet, gotUpdate, err
}
