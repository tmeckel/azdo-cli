package delete

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type fakeDeleteDeps struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	wit        *mocks.MockWorkItemTrackingClient
	prompter   *mocks.MockPrompter
	stdout     *bytes.Buffer
}

func setupFakeDeps(t *testing.T, organization string, canPrompt bool) *fakeDeleteDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdinTTY(canPrompt)
	io.SetStdoutTTY(canPrompt)
	io.SetStderrTTY(canPrompt)

	deps := &fakeDeleteDeps{
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		wit:        mocks.NewMockWorkItemTrackingClient(ctrl),
		prompter:   mocks.NewMockPrompter(ctrl),
		stdout:     out,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(deps.wit, nil).AnyTimes()
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).AnyTimes()

	return deps
}

func setupFakeDepsWithDefaultOrg(t *testing.T, defaultOrg string, canPrompt bool) *fakeDeleteDeps {
	t.Helper()

	deps := setupFakeDeps(t, defaultOrg, canPrompt)
	cfg := mocks.NewMockConfig(deps.ctrl)
	auth := mocks.NewMockAuthConfig(deps.ctrl)

	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return(defaultOrg, nil).AnyTimes()

	return deps
}

func requireFlagError(t *testing.T, err error, substr string) {
	t.Helper()

	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), substr)
}

func TestNewCmd_RegistersAsDeleteLeaf(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)

	assert.Equal(t, "delete", cmd.Name())
	assert.Equal(t, []string{"d", "del", "rm"}, cmd.Aliases)
	assert.True(t, strings.HasPrefix(cmd.Use, "delete [ORGANIZATION/]PROJECT"))
}

func TestNewCmd_PathFlagRequired(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	cmd.SetArgs([]string{"Fabrikam"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path")
}

func TestRunDelete_EmptyPathFlag(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "   "}

	err := runDelete(deps.cmd, opts)

	requireFlagError(t, err, "--path must not be empty")
}

func TestRunDelete_RootNode_Rejected(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Fabrikam/Iteration"}

	err := runDelete(deps.cmd, opts)

	requireFlagError(t, err, "--path must reference a child")
}

func TestRunDelete_PathNormalizationStripsProjectAndIteration(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Fabrikam/Iteration/Release 2025/Sprint 1", yes: true}
	var got workitemtracking.DeleteClassificationNodeArgs

	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.DeleteClassificationNodeArgs) error {
			got = args
			return nil
		},
	)
	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
	assert.Equal(t, "Release%202025/Sprint%201", *got.Path)
}

func TestRunDelete_PathURLEscaping(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "My Sprint/Sub Sprint", yes: true}
	var got workitemtracking.DeleteClassificationNodeArgs

	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.DeleteClassificationNodeArgs) error {
			got = args
			return nil
		},
	)
	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
	assert.Equal(t, "My%20Sprint/Sub%20Sprint", *got.Path)
}

func TestRunDelete_ReclassifyId_Set(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1", reclassifyID: types.ToPtr(42), yes: true}
	var got workitemtracking.DeleteClassificationNodeArgs

	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.DeleteClassificationNodeArgs) error {
			got = args
			return nil
		},
	)
	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
	require.NotNil(t, got.ReclassifyId)
	assert.Equal(t, 42, *got.ReclassifyId)
}

func TestRunDelete_ProjectScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		scopeArg   string
		org        string
		project    string
		wantErr    string
		defaultOrg string
	}{
		{name: "organization and project", scopeArg: "org/proj", org: "org", project: "proj"},
		{name: "project uses default organization", scopeArg: "proj", org: "default-org", project: "proj", defaultOrg: "default-org"},
		{name: "too many segments", scopeArg: "org/proj/extra", wantErr: "expected"},
		{name: "empty scope", scopeArg: "", wantErr: "expected"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var deps *fakeDeleteDeps
			if tc.defaultOrg != "" {
				deps = setupFakeDepsWithDefaultOrg(t, tc.defaultOrg, false)
			} else {
				deps = setupFakeDeps(t, tc.org, false)
			}
			opts := &deleteOptions{scopeArg: tc.scopeArg, path: "Sprint 1", yes: true}

			if tc.wantErr != "" {
				err := runDelete(deps.cmd, opts)
				requireFlagError(t, err, tc.wantErr)
				return
			}

			var got workitemtracking.DeleteClassificationNodeArgs
			deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args workitemtracking.DeleteClassificationNodeArgs) error {
					got = args
					return nil
				},
			)
			err := runDelete(deps.cmd, opts)
			require.NoError(t, err)
			assert.Equal(t, tc.project, *got.Project)
		})
	}
}

func TestRunDelete_ClientFactoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	cmd := mocks.NewMockCmdContext(ctrl)
	clientFact := mocks.NewMockClientFactory(ctrl)

	cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmd.EXPECT().ClientFactory().Return(clientFact).AnyTimes()
	clientFact.EXPECT().WorkItemTracking(gomock.Any(), "org").Return(nil, errors.New("boom"))

	err := runDelete(cmd, &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1", yes: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get classification client")
}

func TestRunDelete_SDKError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1", yes: true}
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(errors.New("boom"))

	err := runDelete(deps.cmd, opts)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete iteration")
}

func TestRunDelete_YesFlag_SkipsPrompt(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", true)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1", yes: true}

	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Times(0)
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
}

func TestRunDelete_ConfirmationPrompt_Yes(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", true)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1"}

	deps.prompter.EXPECT().Confirm("Delete iteration \"Sprint%201\" from project org/Fabrikam?", false).Return(true, nil)
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
}

func TestRunDelete_ConfirmationPrompt_No(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", true)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1"}

	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Times(0)

	err := runDelete(deps.cmd, opts)

	require.ErrorIs(t, err, util.ErrCancel)
}

func TestRunDelete_NonTTY_NoYes_ReturnsError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1"}

	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Times(0)

	err := runDelete(deps.cmd, opts)

	requireFlagError(t, err, "--yes required when not running interactively")
}

func TestRunDelete_DefaultOutput(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1", yes: true}

	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
	assert.Equal(t, "Deleted iteration: Sprint%201\n", deps.stdout.String())
}

func TestRunDelete_DefaultOutput_WithReclassify(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1", reclassifyID: types.ToPtr(42), yes: true}

	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
	assert.Equal(t, "Deleted iteration: Sprint%201\nReclassified work items to: 42\n", deps.stdout.String())
}

func TestRunDelete_JSONOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		reclassifyID *int
	}{
		{name: "without reclassify"},
		{name: "with reclassify", reclassifyID: types.ToPtr(42)},
		{name: "with explicit zero", reclassifyID: types.ToPtr(0)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := setupFakeDeps(t, "org", false)
			opts := &deleteOptions{scopeArg: "org/Fabrikam", path: "Sprint 1", reclassifyID: tc.reclassifyID, yes: true, exporter: util.NewJSONExporter()}

			deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

			err := runDelete(deps.cmd, opts)

			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &got))
			assert.Equal(t, true, got["deleted"])
			assert.Equal(t, "Sprint%201", got["path"])
			if tc.reclassifyID == nil {
				assert.NotContains(t, got, "reclassifyId")
				return
			}
			assert.Equal(t, float64(*tc.reclassifyID), got["reclassifyId"])
		})
	}
}

func TestRunDelete_OrganizationFromConfigDefault(t *testing.T) {
	t.Parallel()

	deps := setupFakeDepsWithDefaultOrg(t, "default-org", false)
	opts := &deleteOptions{scopeArg: "Fabrikam", path: "Sprint 1", yes: true}
	var got workitemtracking.DeleteClassificationNodeArgs
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.DeleteClassificationNodeArgs) error {
			got = args
			return nil
		},
	)

	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
	assert.Equal(t, "Fabrikam", *got.Project)
	assert.Equal(t, "Sprint%201", *got.Path)
	assert.Equal(t, workitemtracking.TreeStructureGroupValues.Iterations, *got.StructureGroup)
	assert.Equal(t, "iterations", string(*got.StructureGroup))
	assert.Nil(t, got.ReclassifyId)
}
