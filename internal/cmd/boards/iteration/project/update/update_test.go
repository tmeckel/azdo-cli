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

func updateNode(goal string) *workitemtracking.WorkItemClassificationNode {
	id := 42
	hasChildren := true
	name := "Sprint 1"
	path := "Fabrikam\\Iteration\\Release 2025\\Sprint 1"
	attrs := map[string]any{
		"startDate":  "2025-01-06T00:00:00Z",
		"finishDate": "2025-01-19T00:00:00Z",
		"goal":       goal,
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

func existingUpdateNode() *workitemtracking.WorkItemClassificationNode {
	return updateNode("Old goal")
}

func updatedUpdateNode() *workitemtracking.WorkItemClassificationNode {
	return updateNode("Ship login")
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
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Times(0)
	deps.wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).Times(0)

	err := runUpdate(deps.cmd, &updateOptions{scopeArg: "org/Iteration", startDate: "2025-01-06"})

	requireFlagError(t, err, "target must reference a child of /Iteration")
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
			return updatedUpdateNode(), nil
		},
	)

	err := runUpdate(deps.cmd, opts)

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

func TestRunUpdate_ProjectScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		scopeArg   string
		org        string
		project    string
		path       string
		wantErr    string
		defaultOrg string
	}{
		{name: "project uses default organization", scopeArg: "proj/Sprint 1", org: "default-org", project: "proj", path: "Sprint%201", defaultOrg: "default-org"},
		{name: "variable targets stay in path", scopeArg: "org/target1/target2/extra", org: "org", project: "target1", path: "target2/extra"},
		{name: "empty scope", scopeArg: "", wantErr: "expected"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			io, _, out, _ := iostreams.Test()
			io.SetStdoutTTY(false)
			io.SetStderrTTY(false)

			cmd := mocks.NewMockCmdContext(ctrl)
			clientFact := mocks.NewMockClientFactory(ctrl)
			wit := mocks.NewMockWorkItemTrackingClient(ctrl)
			cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
			cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
			cmd.EXPECT().ClientFactory().Return(clientFact).AnyTimes()

			if tc.defaultOrg != "" {
				cfg := mocks.NewMockConfig(ctrl)
				auth := mocks.NewMockAuthConfig(ctrl)
				cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
				cfg.EXPECT().Authentication().Return(auth).AnyTimes()
				auth.EXPECT().GetDefaultOrganization().Return(tc.defaultOrg, nil).AnyTimes()
			}

			tp, err := printer.NewTablePrinter(out, false, 200)
			require.NoError(t, err)
			cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

			opts := &updateOptions{scopeArg: tc.scopeArg, startDate: "2025-01-06"}

			if tc.wantErr != "" {
				err := runUpdate(cmd, opts)
				requireFlagError(t, err, tc.wantErr)
				return
			}

			var gotOrg string
			clientFact.EXPECT().WorkItemTracking(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, organization string) (workitemtracking.Client, error) {
					gotOrg = organization
					return wit, nil
				},
			)
			var gotGet workitemtracking.GetClassificationNodeArgs
			var gotUpdate workitemtracking.CreateOrUpdateClassificationNodeArgs
			wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
					gotGet = args
					return existingUpdateNode(), nil
				},
			)
			wit.EXPECT().CreateOrUpdateClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args workitemtracking.CreateOrUpdateClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
					gotUpdate = args
					return updatedUpdateNode(), nil
				},
			)

			err = runUpdate(cmd, opts)
			require.NoError(t, err)
			assert.Equal(t, tc.org, gotOrg)
			assert.Equal(t, tc.project, *gotGet.Project)
			assert.Equal(t, tc.path, *gotGet.Path)
			assert.Equal(t, tc.project, *gotUpdate.Project)
			assert.Equal(t, tc.path, *gotUpdate.Path)
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
	var got struct {
		ID            int                    `json:"id"`
		Identifier    string                 `json:"identifier"`
		Name          string                 `json:"name"`
		Path          string                 `json:"path"`
		HasChildren   bool                   `json:"hasChildren"`
		StructureType string                 `json:"structureType"`
		URL           string                 `json:"url"`
		Attributes    map[string]any         `json:"attributes"`
		Links         map[string]interface{} `json:"_links"`
	}
	require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &got))
	assert.Equal(t, 42, got.ID)
	assert.Equal(t, identifier.String(), got.Identifier)
	assert.Equal(t, "Sprint 1", got.Name)
	assert.Equal(t, "Fabrikam\\Iteration\\Release 2025\\Sprint 1", got.Path)
	assert.True(t, got.HasChildren)
	assert.Equal(t, "iteration", got.StructureType)
	assert.Equal(t, url, got.URL)
	assert.Equal(t, "2025-01-06T00:00:00Z", got.Attributes["startDate"])
	assert.Equal(t, "2025-01-19T00:00:00Z", got.Attributes["finishDate"])
	assert.Equal(t, "Ship login", got.Attributes["goal"])
	assert.Equal(t, "Alpha", got.Attributes["team"])
	require.Contains(t, got.Links, "self")
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
