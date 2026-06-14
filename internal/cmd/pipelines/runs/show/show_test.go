package show

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deps struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	build      *mocks.MockBuildClient
	cfg        *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	stdout     *bytes.Buffer
}

func newDeps(t *testing.T, organization string) *deps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	d := &deps{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		build:      mocks.NewMockBuildClient(ctrl),
		cfg:        mocks.NewMockConfig(ctrl),
		auth:       mocks.NewMockAuthConfig(ctrl),
		stdout:     out,
	}

	d.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	d.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	d.cmd.EXPECT().ClientFactory().Return(d.clientFact).AnyTimes()
	d.clientFact.EXPECT().Build(gomock.Any(), organization).Return(d.build, nil).AnyTimes()

	return d
}

func newDepsWithCfg(t *testing.T, defaultOrg string) *deps {
	d := newDeps(t, defaultOrg)
	d.cmd.EXPECT().Config().Return(d.cfg, nil).AnyTimes()
	d.cfg.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(defaultOrg, nil).AnyTimes()
	return d
}

func sampleBuild(id int) build.Build {
	idPtr := id
	bnum := "20240101." + strconv.Itoa(id%10)
	sourceBranch := "refs/heads/main"
	sourceVersion := "abc12345def"
	dispName := "Alice"
	uniqName := "alice@x.com"
	pipelineName := "MyPipeline"
	queueName := "MyQueue"
	queueID := 42
	urlStr := "https://dev.azure.com/myorg/fabrikam/_build/results?buildId=" + strconv.Itoa(id)
	return build.Build{
		Id:            &idPtr,
		BuildNumber:   &bnum,
		Status:        &build.BuildStatusValues.Completed,
		Result:        &build.BuildResultValues.Succeeded,
		Reason:        &build.BuildReasonValues.Manual,
		Definition:    &build.DefinitionReference{Name: &pipelineName, Id: &idPtr},
		Queue:         &build.AgentPoolQueue{Name: &queueName, Id: &queueID},
		SourceBranch:  &sourceBranch,
		SourceVersion: &sourceVersion,
		RequestedBy:   &webapi.IdentityRef{DisplayName: &dispName, UniqueName: &uniqName},
		RequestedFor:  &webapi.IdentityRef{DisplayName: &dispName, UniqueName: &uniqName},
		Priority:      types.ToPtr(build.QueuePriority("normal")),
		QueueTime:     &azuredevops.Time{},
		StartTime:     &azuredevops.Time{},
		FinishTime:    &azuredevops.Time{},
		Url:           &urlStr,
	}
}

type spyExporter struct {
	writes int
	got    any
}

func (s *spyExporter) Fields() []string { return nil }

func (s *spyExporter) Write(_ *iostreams.IOStreams, v any) error {
	s.writes++
	s.got = v
	return nil
}

func TestNewCmd_RegistersAsShowLeaf(t *testing.T) {
	t.Parallel()
	cmd := NewCmd(nil)
	assert.Equal(t, "show", cmd.Name())
	assert.Contains(t, cmd.Aliases, "view")
	assert.Contains(t, cmd.Aliases, "status")
	assert.Contains(t, cmd.Use, "[ORGANIZATION/]PROJECT RUN_ID")
}

func TestRunShow_RunIDMustBeInteger(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "abc")
	require.Error(t, err)
	var fe *util.FlagError
	assert.ErrorAs(t, err, &fe)
}

func TestRunShow_RunIDMustBePositive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		rawID string
	}{
		{name: "zero", rawID: "0"},
		{name: "negative", rawID: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t, "MyOrg")
			err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, tt.rawID)
			require.Error(t, err)
			var fe *util.FlagError
			assert.ErrorAs(t, err, &fe)
		})
	}
}

func TestRunShow_BasicCall(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	buildObj := sampleBuild(12345)
	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args build.GetBuildArgs) (*build.Build, error) {
			assert.Equal(t, 12345, *args.BuildId)
			assert.Equal(t, "Fabrikam", *args.Project)
			return &buildObj, nil
		})

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "12345")
	require.NoError(t, err)
}

func TestRunShow_TemplateOutput_BasicFields(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	buildObj := sampleBuild(42)
	buildObj.Status = &build.BuildStatusValues.Completed
	buildObj.Result = &build.BuildResultValues.Succeeded
	buildObj.Reason = &build.BuildReasonValues.Manual
	buildObj.SourceBranch = types.ToPtr("refs/heads/main")
	buildObj.SourceVersion = types.ToPtr("abc12345def")

	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(&buildObj, nil)

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "42")
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "id:")
	assert.Contains(t, output, "build number:")
	assert.Contains(t, output, "status:")
	assert.Contains(t, output, "result:")
	assert.Contains(t, output, "reason:")
	assert.Contains(t, output, "source branch:")
	assert.Contains(t, output, "source version:")
	assert.Contains(t, output, "definition:")
}

func TestRunShow_TemplateOutput_Hyperlink(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	buildObj := sampleBuild(1)
	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(&buildObj, nil)

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "1")
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "\x1b]8;;")
	assert.Contains(t, output, "url:")
}

func TestRunShow_TemplateOutput_DurationFormatted(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	buildObj := sampleBuild(1)
	startTime, _ := time.Parse(time.RFC3339, "2024-01-01T12:00:00Z")
	finishTime, _ := time.Parse(time.RFC3339, "2024-01-01T12:02:13Z")
	buildObj.StartTime = &azuredevops.Time{Time: startTime}
	buildObj.FinishTime = &azuredevops.Time{Time: finishTime}

	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(&buildObj, nil)

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "1")
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "duration:")
	assert.Contains(t, output, "2m13s")
}

func TestRunShow_TemplateOutput_NoDurationNotStarted(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	buildObj := sampleBuild(1)
	buildObj.FinishTime = nil

	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(&buildObj, nil)

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "1")
	require.NoError(t, err)

	output := d.stdout.String()
	assert.NotContains(t, output, "duration:")
}

func TestRunShow_TemplateOutput_Tags(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	buildObj := sampleBuild(1)
	buildObj.Tags = &[]string{"release", "nightly"}

	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(&buildObj, nil)

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "1")
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "tags:")
	assert.Contains(t, output, "release")
	assert.Contains(t, output, "nightly")
}

func TestRunShow_TemplateOutput_NoTags(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	buildObj := sampleBuild(1)
	buildObj.Tags = nil

	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(&buildObj, nil)

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "1")
	require.NoError(t, err)

	output := d.stdout.String()
	assert.NotContains(t, output, "tags:")
}

func TestRunShow_TemplateOutput_DefinitionAndQueueNested(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	buildObj := sampleBuild(1)
	defID := 99
	defName := "MyDef"
	queueID := 55
	queueName := "Azure Pipelines"
	buildObj.Definition = &build.DefinitionReference{Name: &defName, Id: &defID}
	buildObj.Queue = &build.AgentPoolQueue{Name: &queueName, Id: &queueID}

	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(&buildObj, nil)

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "1")
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "MyDef")
	assert.Contains(t, output, "Azure Pipelines")
}

func TestRunShow_TemplateOutput_ResultVisibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		status        build.BuildStatus
		result        *build.BuildResult
		wantResultRow bool
	}{
		{
			name:          "hidden when not completed",
			status:        build.BuildStatusValues.InProgress,
			result:        nil,
			wantResultRow: false,
		},
		{
			name:          "shown when completed",
			status:        build.BuildStatusValues.Completed,
			result:        types.ToPtr(build.BuildResultValues.Succeeded),
			wantResultRow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newDeps(t, "MyOrg")

			buildObj := sampleBuild(1)
			buildObj.Status = &tt.status
			buildObj.Result = tt.result

			d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(&buildObj, nil)

			err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "1")
			require.NoError(t, err)

			output := d.stdout.String()
			if tt.wantResultRow {
				assert.Contains(t, output, "result:")
				assert.Contains(t, output, "succeeded")
				return
			}
			assert.NotContains(t, output, "result:")
		})
	}
}

func TestRunShow_JSONOutput(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	buildObj := sampleBuild(1)
	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(&buildObj, nil)

	spy := &spyExporter{}
	err := runShow(d.cmd, &showOptions{
		scopeArg: "MyOrg/Fabrikam",
		exporter: spy,
	}, "1")
	require.NoError(t, err)
	assert.Equal(t, 1, spy.writes)
	require.NotNil(t, spy.got)

	gotBuild, ok := spy.got.(*build.Build)
	require.True(t, ok, "exporter must receive *build.Build")
	assert.Equal(t, 1, *gotBuild.Id)
}

func TestRunShow_ProjectScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scopeArg string
		wantOrg  string
		wantProj string
		withCfg  bool
	}{
		{name: "org/project", scopeArg: "MyOrg/Fabrikam", wantOrg: "MyOrg", wantProj: "Fabrikam", withCfg: false},
		{name: "project only uses config default", scopeArg: "Fabrikam", wantOrg: "default-org", wantProj: "Fabrikam", withCfg: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d *deps
			if tt.withCfg {
				d = newDepsWithCfg(t, tt.wantOrg)
			} else {
				d = newDeps(t, tt.wantOrg)
			}

			buildObj := sampleBuild(1)
			d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, args build.GetBuildArgs) (*build.Build, error) {
					assert.Equal(t, tt.wantProj, *args.Project)
					return &buildObj, nil
				})

			err := runShow(d.cmd, &showOptions{scopeArg: tt.scopeArg}, "1")
			require.NoError(t, err)
		})
	}
}

func TestRunShow_ClientFactoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	cmd := mocks.NewMockCmdContext(ctrl)
	clientFact := mocks.NewMockClientFactory(ctrl)
	cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmd.EXPECT().ClientFactory().Return(clientFact).AnyTimes()
	clientFact.EXPECT().Build(gomock.Any(), "MyOrg").Return(nil, assert.AnError)

	err := runShow(cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "1")
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestRunShow_SDKError(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "MyOrg")

	d.build.EXPECT().GetBuild(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	err := runShow(d.cmd, &showOptions{scopeArg: "MyOrg/Fabrikam"}, "1")
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}
