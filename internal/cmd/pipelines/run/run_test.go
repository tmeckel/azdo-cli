package run

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type runDeps struct {
	cmd         *mocks.MockCmdContext
	clientFact  *mocks.MockClientFactory
	buildClient *mocks.MockBuildClient
	cfg         *mocks.MockConfig
	authCfg     *mocks.MockAuthConfig
	stdout      *bytes.Buffer
	t           *testing.T
}

func setupRunDeps(t *testing.T, organization string) *runDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	d := &runDeps{
		cmd:         mocks.NewMockCmdContext(ctrl),
		clientFact:  mocks.NewMockClientFactory(ctrl),
		buildClient: mocks.NewMockBuildClient(ctrl),
		cfg:         mocks.NewMockConfig(ctrl),
		authCfg:     mocks.NewMockAuthConfig(ctrl),
		stdout:      out,
		t:           t,
	}

	d.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	d.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	d.cmd.EXPECT().ClientFactory().Return(d.clientFact).AnyTimes()
	d.cmd.EXPECT().Config().Return(d.cfg, nil).AnyTimes()
	d.cfg.EXPECT().Authentication().Return(d.authCfg).AnyTimes()
	d.authCfg.EXPECT().GetDefaultOrganization().Return(organization, nil).AnyTimes()

	tp, err := printer.NewListPrinter(out)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	return d
}

func setupBuildClient(d *runDeps) {
	d.clientFact.EXPECT().Build(gomock.Any(), gomock.Any()).Return(d.buildClient, nil).AnyTimes()
}

func expectQueueBuild(d *runDeps, project string, want *build.Build, resp *build.Build, err error) {
	d.buildClient.EXPECT().QueueBuild(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.QueueBuildArgs) (*build.Build, error) {
			require.NotNil(d.t, args.Project)
			require.Equal(d.t, project, *args.Project)
			if want != nil {
				require.NotNil(d.t, args.Build)
				require.NotNil(d.t, args.Build.Definition)
				require.Equal(d.t, types.GetValue(want.Definition.Id, 0), types.GetValue(args.Build.Definition.Id, 0))
				require.Equal(d.t, types.GetValue(want.SourceBranch, ""), types.GetValue(args.Build.SourceBranch, ""))
				require.Equal(d.t, types.GetValue(want.SourceVersion, ""), types.GetValue(args.Build.SourceVersion, ""))
				require.Equal(d.t, types.GetValue(want.Parameters, ""), types.GetValue(args.Build.Parameters, ""))
			}
			return resp, err
		},
	).Times(1)
}

func expectGetDefinitions(d *runDeps, project, name string, resp *build.GetDefinitionsResponseValue, err error) {
	d.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
			require.NotNil(d.t, args.Project)
			require.Equal(d.t, project, *args.Project)
			require.NotNil(d.t, args.Name)
			require.Equal(d.t, name, *args.Name)
			return resp, err
		},
	).Times(1)
}

func sampleBuild(t *testing.T) *build.Build {
	t.Helper()
	return &build.Build{
		Id:           types.ToPtr(1001),
		BuildNumber:  types.ToPtr("20250615.1"),
		Status:       types.ToPtr(build.BuildStatusValues.Completed),
		Result:       types.ToPtr(build.BuildResultValues.Succeeded),
		Reason:       types.ToPtr(build.BuildReasonValues.Manual),
		SourceBranch: types.ToPtr("refs/heads/main"),
		QueueTime:    &azuredevops.Time{Time: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)},
		Definition: &build.DefinitionReference{
			Id:   types.ToPtr(42),
			Name: types.ToPtr("MyPipeline"),
		},
	}
}

func TestNewCmd_RegistersAsRunLeaf(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(nil)
	assert.Equal(t, "run [ORGANIZATION/]PROJECT/PIPELINE", cmd.Use)
	require.NotNil(t, cmd.RunE)
	assert.NotNil(t, cmd.Flags().Lookup("json"))
	assert.NotNil(t, cmd.Flags().Lookup("branch"))
	assert.NotNil(t, cmd.Flags().Lookup("commit-id"))
	assert.NotNil(t, cmd.Flags().Lookup("variable"))
	assert.NotNil(t, cmd.Flags().Lookup("folder-path"))
}

func TestNewCmd_RequiresOneArg(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(nil)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline target is required")
}

func TestRunRun_ByPositiveID(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)
	d.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Times(0)

	want := &build.Build{Definition: &build.DefinitionReference{Id: types.ToPtr(42)}}
	expectQueueBuild(d, "Fabrikam", want, sampleBuild(t), nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42"})
	err := cmd.Execute()

	require.NoError(t, err)
	out := d.stdout.String()
	assert.Contains(t, out, "Run ID")
	assert.Contains(t, out, "1001")
}

func TestRunRun_ByName(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	expectGetDefinitions(d, "Fabrikam", "MyPipeline", &build.GetDefinitionsResponseValue{
		Value: []build.BuildDefinitionReference{
			{Id: types.ToPtr(42), Name: types.ToPtr("MyPipeline")},
			{Id: types.ToPtr(99), Name: types.ToPtr("MyPipeline")},
		},
	}, nil)

	want := &build.Build{Definition: &build.DefinitionReference{Id: types.ToPtr(42)}}
	expectQueueBuild(d, "Fabrikam", want, sampleBuild(t), nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/MyPipeline"})
	err := cmd.Execute()

	require.NoError(t, err)
}

func TestRunRun_RespectsBranchNormalization(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	want := &build.Build{
		Definition:   &build.DefinitionReference{Id: types.ToPtr(42)},
		SourceBranch: types.ToPtr("refs/heads/feature"),
	}
	expectQueueBuild(d, "Fabrikam", want, sampleBuild(t), nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42", "--branch", "feature"})
	err := cmd.Execute()

	require.NoError(t, err)
}

func TestRunRun_SetsCommitID(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	want := &build.Build{
		Definition:    &build.DefinitionReference{Id: types.ToPtr(42)},
		SourceVersion: types.ToPtr("abc123"),
	}
	expectQueueBuild(d, "Fabrikam", want, sampleBuild(t), nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42", "--commit-id", "abc123"})
	err := cmd.Execute()

	require.NoError(t, err)
}

func TestRunRun_SetsVariables(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	params := `{"env":"prod","ver":"2"}`
	want := &build.Build{
		Definition: &build.DefinitionReference{Id: types.ToPtr(42)},
		Parameters: &params,
	}
	expectQueueBuild(d, "Fabrikam", want, sampleBuild(t), nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42", "--variable", "env=prod", "--variable", "ver=2"})
	err := cmd.Execute()

	require.NoError(t, err)
}

func TestRunRun_DefaultsOrg(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "DefaultOrg")
	setupBuildClient(d)

	want := &build.Build{Definition: &build.DefinitionReference{Id: types.ToPtr(42)}}
	expectQueueBuild(d, "Fabrikam", want, sampleBuild(t), nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"Fabrikam/42"})
	err := cmd.Execute()

	require.NoError(t, err)
}

func TestRunRun_RejectsNonPositiveNumericID(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/0"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline id must be greater than zero")
	assert.Empty(t, d.stdout.String())
}

func TestRunRun_NameNotFound(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	expectGetDefinitions(d, "Fabrikam", "Ghost", &build.GetDefinitionsResponseValue{}, nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/Ghost"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunRun_RejectsInvalidVariable(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42", "--variable", "bad"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid variable")
}

func TestRunRun_PropagatesQueueError(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	want := &build.Build{Definition: &build.DefinitionReference{Id: types.ToPtr(42)}}
	expectQueueBuild(d, "Fabrikam", want, nil, errors.New("queue error"))

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42"})
	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to queue pipeline 42: queue error")
}

func TestRunRun_JSONOutput(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	want := &build.Build{Definition: &build.DefinitionReference{Id: types.ToPtr(42)}}
	blob := sampleBuild(t)
	expectQueueBuild(d, "Fabrikam", want, blob, nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/42", "--json=id", "--json=buildNumber"})
	err := cmd.Execute()

	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(d.stdout.Bytes(), &got))
	assert.Equal(t, float64(1001), got["id"])
	assert.Equal(t, "20250615.1", got["buildNumber"])
	_, hasStatus := got["status"]
	assert.False(t, hasStatus)
}

func TestRunRun_FolderPathByName(t *testing.T) {
	t.Parallel()
	d := setupRunDeps(t, "MyOrg")
	setupBuildClient(d)

	d.buildClient.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetDefinitionsArgs) (*build.GetDefinitionsResponseValue, error) {
			require.NotNil(d.t, args.Project)
			require.Equal(d.t, "Fabrikam", *args.Project)
			require.NotNil(d.t, args.Name)
			require.Equal(d.t, "MyPipeline", *args.Name)
			require.NotNil(d.t, args.Path)
			require.Equal(d.t, `\Shared`, *args.Path)
			return &build.GetDefinitionsResponseValue{
				Value: []build.BuildDefinitionReference{
					{Id: types.ToPtr(7), Name: types.ToPtr("MyPipeline")},
					{Id: types.ToPtr(8), Name: types.ToPtr("MyPipeline")},
				},
			}, nil
		},
	).Times(1)

	want := &build.Build{Definition: &build.DefinitionReference{Id: types.ToPtr(7)}}
	expectQueueBuild(d, "Fabrikam", want, sampleBuild(t), nil)

	cmd := NewCmd(d.cmd)
	cmd.SetArgs([]string{"MyOrg/Fabrikam/MyPipeline", "--folder-path", `\Shared`})
	err := cmd.Execute()

	require.NoError(t, err)
}

func TestEncodeVariables(t *testing.T) {
	t.Parallel()
	got, err := encodeVariables([]string{"a=b", "c=d=e"})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.JSONEq(t, `{"a":"b","c":"d=e"}`, *got)
}

func TestEncodeVariables_RejectsMissingNameValueSeparator(t *testing.T) {
	t.Parallel()
	_, err := encodeVariables([]string{"=val"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected name=value")
}

func TestNormalizeBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, expected string
	}{
		{"main", "refs/heads/main"},
		{"feature/x", "refs/heads/feature/x"},
		{"refs/heads/main", "refs/heads/main"},
		{"refs/pull/1/merge", "refs/pull/1/merge"},
		{"refs/tags/v1", "refs/tags/v1"},
	}
	for _, tt := range tests {
		got := normalizeBranch(tt.input)
		assert.Equal(t, tt.expected, got)
	}
}
