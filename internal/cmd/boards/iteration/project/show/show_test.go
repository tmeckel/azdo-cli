package show

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

type dependencies struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	wit        *mocks.MockWorkItemTrackingClient
	stdout     *bytes.Buffer
	stderr     *bytes.Buffer
	org        string
}

func newDependencies(t *testing.T, organization string) *dependencies {
	t.Helper()

	return newDependenciesWithClientFactoryError(t, organization, nil)
}

func newDependenciesWithClientFactoryError(t *testing.T, organization string, factoryErr error) *dependencies {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &dependencies{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		stdout:     out,
		stderr:     errOut,
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
	if factoryErr != nil {
		deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(nil, factoryErr).AnyTimes()
	} else {
		deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(deps.wit, nil).AnyTimes()
	}

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

func showNode() *workitemtracking.WorkItemClassificationNode {
	id := 42
	identifier := uuid.New()
	hasChildren := true
	name := "Sprint 1"
	path := "Fabrikam\\Iteration\\Sprint 1"
	structureType := workitemtracking.TreeNodeStructureTypeValues.Iteration
	url := "https://dev.azure.com/org/Fabrikam/_apis/wit/classificationNodes/iterations/42"
	return &workitemtracking.WorkItemClassificationNode{
		Id:            &id,
		Identifier:    &identifier,
		Name:          &name,
		Path:          &path,
		StructureType: &structureType,
		HasChildren:   &hasChildren,
		Url:           &url,
		Links: map[string]any{
			"self": map[string]any{"href": url},
		},
	}
}

func TestNewCmd_RegistersAsShowLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)

	assert.Equal(t, "show", cmd.Name())
	assert.Equal(t, []string{"view", "status"}, cmd.Aliases)
	assert.True(t, strings.HasPrefix(cmd.Use, "show [ORGANIZATION/]PROJECT"))
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

func TestRunShow_InvalidTarget(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	err := runShow(deps.cmd, &showOptions{scopeArg: "org"})
	requireFlagError(t, err, "expected 2-66 segments")
}

func TestRunShow_DepthBounds(t *testing.T) {
	t.Parallel()

	for _, depth := range []int{-1, 11} {
		deps := newDependencies(t, "org")

		err := runShow(deps.cmd, &showOptions{scopeArg: "org/Fabrikam/Sprint 1", depth: depth})

		requireFlagError(t, err, "--depth must be between 0 and 10")
	}
}

func TestRunShow_RootNodeRejected(t *testing.T) {
	t.Parallel()

	deps := newDependenciesWithDefaultOrg(t, "default-org")
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Times(0)

	err := runShow(deps.cmd, &showOptions{scopeArg: "org/Iteration"})

	requireFlagError(t, err, "target must reference a child of /Iteration")
}

func TestRunShow_RequestArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		opts      *showOptions
		wantPath  string
		wantProj  string
		wantDepth int
	}{
		{name: "root level", opts: &showOptions{scopeArg: "org/Fabrikam/Sprint 1"}, wantPath: "Fabrikam/Sprint%201", wantProj: "org", wantDepth: 0},
		{name: "normalizes project path", opts: &showOptions{scopeArg: "org/Fabrikam/Fabrikam/Iteration/Sprint 1"}, wantPath: "Sprint%201", wantProj: "Fabrikam", wantDepth: 0},
		{name: "escapes nested path", opts: &showOptions{scopeArg: "org/Fabrikam/My Sprint/Sub Sprint"}, wantPath: "My%20Sprint/Sub%20Sprint", wantProj: "Fabrikam", wantDepth: 0},
		{name: "uses explicit depth", opts: &showOptions{scopeArg: "org/Fabrikam/Sprint 1", depth: 2}, wantPath: "Fabrikam/Sprint%201", wantProj: "org", wantDepth: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "org")
			args, err := captureShowArgs(t, deps, tc.opts, showNode())

			require.NoError(t, err)
			assert.Equal(t, tc.wantPath, *args.Path)
			assert.Equal(t, tc.wantProj, *args.Project)
			assert.Equal(t, tc.wantDepth, *args.Depth)
			assert.Equal(t, workitemtracking.TreeStructureGroupValues.Iterations, *args.StructureGroup)
			assert.Equal(t, "iterations", string(*args.StructureGroup))
		})
	}
}

func TestRunShow_TemplateOutput(t *testing.T) {
	t.Parallel()
	childID := uuid.New()
	tests := []struct {
		name        string
		node        *workitemtracking.WorkItemClassificationNode
		opts        *showOptions
		contains    []string
		notContains []string
	}{
		{
			name: "basic fields without attributes",
			node: showNode(),
			opts: &showOptions{scopeArg: "org/Fabrikam/Sprint 1"},
			contains: []string{
				"url:",
				"https://dev.azure.com/org/Fabrikam/_apis/wit/classificationNodes/iterations/42",
				"id:",
				"42",
				"identifier:",
				"name:",
				"Sprint 1",
				"path:",
				"Fabrikam\\Iteration\\Sprint 1",
				"structure:",
				"iteration",
				"has children:",
				"true",
			},
			notContains: []string{"attributes:"},
		},
		{
			name: "attributes included",
			node: func() *workitemtracking.WorkItemClassificationNode {
				node := showNode()
				attrs := map[string]any{
					"startDate":  "2024-01-01T00:00:00Z",
					"finishDate": "2024-01-15T00:00:00Z",
				}
				node.Attributes = &attrs
				return node
			}(),
			opts:     &showOptions{scopeArg: "org/Fabrikam/Sprint 1"},
			contains: []string{"attributes:", "startDate:     2024-01-01", "finishDate:    2024-01-15"},
		},
		{
			name: "children included when requested",
			node: func() *workitemtracking.WorkItemClassificationNode {
				node := showNode()
				children := []workitemtracking.WorkItemClassificationNode{{
					Name:        types.ToPtr("Sub Sprint"),
					Identifier:  &childID,
					HasChildren: types.ToPtr(true),
				}}
				node.Children = &children
				return node
			}(),
			opts:     &showOptions{scopeArg: "org/Fabrikam/Sprint 1", includeChildren: true},
			contains: []string{"children:", "- Sub Sprint", childID.String(), "hasChildren: true"},
		},
		{
			name: "children omitted by default",
			node: func() *workitemtracking.WorkItemClassificationNode {
				node := showNode()
				children := []workitemtracking.WorkItemClassificationNode{{Name: types.ToPtr("Sub Sprint")}}
				node.Children = &children
				return node
			}(),
			opts:        &showOptions{scopeArg: "org/Fabrikam/Sprint 1"},
			notContains: []string{"  - Sub Sprint"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newDependencies(t, "org")
			deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(tc.node, nil)

			err := runShow(deps.cmd, tc.opts)

			require.NoError(t, err)
			out := deps.stdout.String()
			for _, want := range tc.contains {
				assert.Contains(t, out, want)
			}
			for _, unwanted := range tc.notContains {
				assert.NotContains(t, out, unwanted)
			}
		})
	}
}

func TestRunShow_JSONOutput(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	node := showNode()
	attrs := map[string]any{
		"startDate":  "2024-01-01T00:00:00Z",
		"finishDate": "2024-01-15T00:00:00Z",
	}
	node.Attributes = &attrs
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(node, nil)

	err := runShow(deps.cmd, &showOptions{scopeArg: "org/Fabrikam/Sprint 1", exporter: util.NewJSONExporter()})

	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &got))
	assert.Equal(t, float64(42), got["id"])
	assert.Equal(t, node.Identifier.String(), got["identifier"])
	assert.Equal(t, "Sprint 1", got["name"])
	assert.Equal(t, "Fabrikam\\Iteration\\Sprint 1", got["path"])
	assert.Equal(t, "iteration", got["structureType"])
	assert.Equal(t, true, got["hasChildren"])
	assert.Equal(t, "https://dev.azure.com/org/Fabrikam/_apis/wit/classificationNodes/iterations/42", got["url"])
	attrsJSON, ok := got["attributes"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2024-01-01T00:00:00Z", attrsJSON["startDate"])
	assert.Equal(t, "2024-01-15T00:00:00Z", attrsJSON["finishDate"])
	linksJSON, ok := got["_links"].(map[string]any)
	require.True(t, ok)
	selfJSON, ok := linksJSON["self"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://dev.azure.com/org/Fabrikam/_apis/wit/classificationNodes/iterations/42", selfJSON["href"])
}

func TestRunShow_RawFlag(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(showNode(), nil)

	err := runShow(deps.cmd, &showOptions{scopeArg: "org/Fabrikam/Sprint 1", raw: true})

	require.NoError(t, err)
	assert.Empty(t, deps.stdout.String())
	assert.Contains(t, deps.stderr.String(), "WorkItemClassificationNode")
}

func TestRunShow_ProjectScopeParsing(t *testing.T) {
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
		{name: "variable targets stay in path", scopeArg: "org/proj/release/Sprint 1", org: "org", project: "proj", path: "release/Sprint%201"},
		{name: "empty scope", scopeArg: "", wantErr: "expected"},
		{name: "organization from config default", scopeArg: "Fabrikam/Sprint 1", org: "default-org", project: "Fabrikam", path: "Sprint%201", defaultOrg: "default-org"},
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
			opts := &showOptions{scopeArg: tc.scopeArg}

			if tc.wantErr != "" {
				err := runShow(cmd, opts)
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

			var args workitemtracking.GetClassificationNodeArgs
			wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, got workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
					args = got
					return showNode(), nil
				},
			)

			err = runShow(cmd, opts)
			require.NoError(t, err)
			assert.Equal(t, tc.org, gotOrg)
			assert.Equal(t, tc.project, *args.Project)
			assert.Equal(t, tc.path, *args.Path)
		})
	}
}

func TestRunShow_ClientFactoryError(t *testing.T) {
	t.Parallel()

	deps := newDependenciesWithClientFactoryError(t, "org", errors.New("boom"))

	err := runShow(deps.cmd, &showOptions{scopeArg: "org/Fabrikam/Sprint 1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get classification client")
}

func TestRunShow_SDKError(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	err := runShow(deps.cmd, &showOptions{scopeArg: "org/Fabrikam/Sprint 1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get iteration")
}

func TestRunShow_NilResponse(t *testing.T) {
	t.Parallel()

	deps := newDependencies(t, "org")
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).Return(nil, nil)

	err := runShow(deps.cmd, &showOptions{scopeArg: "org/Fabrikam/Sprint 1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "iteration node is nil")
}

func captureShowArgs(t *testing.T, deps *dependencies, opts *showOptions, response *workitemtracking.WorkItemClassificationNode) (workitemtracking.GetClassificationNodeArgs, error) {
	t.Helper()

	var got workitemtracking.GetClassificationNodeArgs
	deps.wit.EXPECT().GetClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetClassificationNodeArgs) (*workitemtracking.WorkItemClassificationNode, error) {
			got = args
			return response, nil
		},
	)

	err := runShow(deps.cmd, opts)
	return got, err
}

func requireFlagError(t *testing.T, err error, substr string) {
	t.Helper()

	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), substr)
}
