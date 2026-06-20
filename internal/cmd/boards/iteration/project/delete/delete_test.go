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
	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)
	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return(organization, nil).AnyTimes()
	deps.clientFact.EXPECT().WorkItemTracking(gomock.Any(), organization).Return(deps.wit, nil).AnyTimes()
	deps.cmd.EXPECT().Prompter().Return(deps.prompter, nil).AnyTimes()

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

func TestRunDelete_InvalidTarget(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org", yes: true}

	err := runDelete(deps.cmd, opts)

	requireFlagError(t, err, "expected 2-66 segments")
}

func TestRunDelete_RootNode_Rejected(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam/Iteration", yes: true}
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
}

func TestRunDelete_PathParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scopeArg string
		wantPath string
	}{
		{name: "normalizes repeated root segments", scopeArg: "org/Fabrikam/Fabrikam/Iteration/Release 2025/Sprint 1", wantPath: "Release%202025/Sprint%201"},
		{name: "url escapes path segments", scopeArg: "org/Fabrikam/My Sprint/Sub Sprint", wantPath: "My%20Sprint/Sub%20Sprint"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := setupFakeDeps(t, "org", false)
			var got workitemtracking.DeleteClassificationNodeArgs
			deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args workitemtracking.DeleteClassificationNodeArgs) error {
					got = args
					return nil
				},
			)

			err := runDelete(deps.cmd, &deleteOptions{scopeArg: tc.scopeArg, yes: true})

			require.NoError(t, err)
			assert.Equal(t, tc.wantPath, *got.Path)
		})
	}
}

func TestRunDelete_ReclassifyId_Set(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam/Sprint 1", reclassifyID: types.ToPtr(42), yes: true}
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

	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)
	cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return("default-org", nil).AnyTimes()
	clientFact.EXPECT().WorkItemTracking(gomock.Any(), "default-org").Return(nil, assert.AnError)

	err := runDelete(cmd, &deleteOptions{scopeArg: "Fabrikam/Sprint 1", yes: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get classification client")
}

func TestRunDelete_SDKError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam/Sprint 1", yes: true}
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(errors.New("boom"))

	err := runDelete(deps.cmd, opts)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete iteration")
}

func TestRunDelete_YesFlag_SkipsPrompt(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", true)
	opts := &deleteOptions{scopeArg: "org/Fabrikam/Sprint 1", yes: true}

	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Times(0)
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
}

func TestRunDelete_ConfirmationPrompt_Yes(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", true)
	opts := &deleteOptions{scopeArg: "org/Fabrikam/Sprint 1"}

	deps.prompter.EXPECT().Confirm("Delete iteration \"Fabrikam/Sprint%201\" from project org/org?", false).Return(true, nil)
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

	err := runDelete(deps.cmd, opts)

	require.NoError(t, err)
}

func TestRunDelete_ConfirmationPrompt_No(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", true)
	opts := &deleteOptions{scopeArg: "org/Fabrikam/Sprint 1"}

	deps.prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)
	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Times(0)

	err := runDelete(deps.cmd, opts)

	require.ErrorIs(t, err, util.ErrCancel)
}

func TestRunDelete_NonTTY_NoYes_ReturnsError(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "org", false)
	opts := &deleteOptions{scopeArg: "org/Fabrikam/Sprint 1"}

	deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Times(0)

	err := runDelete(deps.cmd, opts)

	requireFlagError(t, err, "--yes required when not running interactively")
}

func TestRunDelete_DefaultOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		reclassifyID *int
		wantOutput   string
	}{
		{name: "without reclassify", wantOutput: "Deleted iteration: Fabrikam/Sprint%201\n"},
		{name: "with reclassify", reclassifyID: types.ToPtr(42), wantOutput: "Deleted iteration: Fabrikam/Sprint%201\nReclassified work items to: 42\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := setupFakeDeps(t, "org", false)
			deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

			err := runDelete(deps.cmd, &deleteOptions{scopeArg: "org/Fabrikam/Sprint 1", reclassifyID: tc.reclassifyID, yes: true})

			require.NoError(t, err)
			assert.Equal(t, tc.wantOutput, deps.stdout.String())
		})
	}
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
			opts := &deleteOptions{scopeArg: "org/Fabrikam/Sprint 1", reclassifyID: tc.reclassifyID, yes: true, exporter: util.NewJSONExporter()}

			deps.wit.EXPECT().DeleteClassificationNode(gomock.Any(), gomock.Any()).Return(nil)

			err := runDelete(deps.cmd, opts)

			require.NoError(t, err)
			var got struct {
				Deleted      bool   `json:"deleted"`
				Path         string `json:"path"`
				ReclassifyID *int   `json:"reclassifyId,omitempty"`
			}
			require.NoError(t, json.Unmarshal(deps.stdout.Bytes(), &got))
			assert.True(t, got.Deleted)
			assert.Equal(t, "Fabrikam/Sprint%201", got.Path)
			if tc.reclassifyID == nil {
				assert.Nil(t, got.ReclassifyID)
				return
			}
			require.NotNil(t, got.ReclassifyID)
			assert.Equal(t, *tc.reclassifyID, *got.ReclassifyID)
		})
	}
}

func TestRunDelete_OrganizationFromConfigDefault(t *testing.T) {
	t.Parallel()

	deps := setupFakeDeps(t, "default-org", false)
	opts := &deleteOptions{scopeArg: "Fabrikam/Sprint 1", yes: true}
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
	assert.Nil(t, got.ReclassifyId)
}
