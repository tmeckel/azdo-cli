package list

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

type dependencies struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	workClient *mocks.MockWorkClient
	config     *mocks.MockConfig
	authCfg    *mocks.MockAuthConfig
}

func newDependencies(t *testing.T, organization string) (*dependencies, *bytes.Buffer) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)

	deps := &dependencies{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		workClient: mocks.NewMockWorkClient(ctrl),
		config:     mocks.NewMockConfig(ctrl),
		authCfg:    mocks.NewMockAuthConfig(ctrl),
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.cmd.EXPECT().Printer(gomock.Any()).Return(tp, nil).AnyTimes()
	deps.clientFact.EXPECT().Work(gomock.Any(), organization).Return(deps.workClient, nil).AnyTimes()

	return deps, out
}

func tfv(value string, includeChildren bool) work.TeamFieldValue {
	return work.TeamFieldValue{
		Value:           &value,
		IncludeChildren: &includeChildren,
	}
}

func TestList_EmptyResult(t *testing.T) {
	deps, _ := newDependencies(t, "myOrg")

	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		Return(&work.TeamFieldValues{Values: &[]work.TeamFieldValue{}}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "no team area paths found")
}

func TestList_RendersTable(t *testing.T) {
	deps, out := newDependencies(t, "myOrg")

	values := []work.TeamFieldValue{
		tfv("Fabrikam/Frontend", true),
		tfv("Fabrikam/Backend", false),
		tfv("Fabrikam/DevOps", true),
	}

	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		Return(&work.TeamFieldValues{DefaultValue: types.ToPtr("Fabrikam/Frontend"), Values: &values}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()

	assert.Contains(t, output, "Fabrikam/Backend")
	assert.Contains(t, output, "Fabrikam/DevOps")
	assert.Contains(t, output, "Fabrikam/Frontend")

	assert.Contains(t, output, "yes")
	assert.Contains(t, output, "no")

	assert.Contains(t, output, "Fabrikam/Frontend (default)")
	assert.Equal(t, 1, strings.Count(output, "(default)"), "only one row should have (default) marker")
	assert.NotContains(t, output, "Fabrikam/Backend (default)")
	assert.NotContains(t, output, "Fabrikam/DevOps (default)")
}

func TestList_SortedOutput(t *testing.T) {
	deps, out := newDependencies(t, "myOrg")

	values := []work.TeamFieldValue{
		tfv("Z/Path", false),
		tfv("A/Path", true),
		tfv("M/Path", false),
	}

	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		Return(&work.TeamFieldValues{DefaultValue: types.ToPtr("M/Path"), Values: &values}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)

	aIdx := bytes.Index(out.Bytes(), []byte("A/Path"))
	mIdx := bytes.Index(out.Bytes(), []byte("M/Path"))
	zIdx := bytes.Index(out.Bytes(), []byte("Z/Path"))
	assert.True(t, aIdx < mIdx && mIdx < zIdx, "expected sorted order")
}

func TestList_JSONOutput(t *testing.T) {
	deps, out := newDependencies(t, "myOrg")

	values := []work.TeamFieldValue{
		tfv("Z/Path", false),
		tfv("A/Path", true),
	}

	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		Return(&work.TeamFieldValues{DefaultValue: types.ToPtr("A/Path"), Values: &values}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--json=areaPath,includeChildren,isDefault"})
	err := cmd.Execute()
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "\"areaPath\"")
	assert.Contains(t, output, "\"includeChildren\":true")
	assert.Contains(t, output, "\"isDefault\":true")
	assert.NotContains(t, output, "AREA PATH")
	assert.NotContains(t, output, "INCLUDE SUB AREAS")

	aIdx := strings.Index(output, "A/Path")
	zIdx := strings.Index(output, "Z/Path")
	assert.True(t, aIdx < zIdx, "expected sorted JSON output")
}

func TestList_JSONOutput_EmptyValuesReturnsEmptyArray(t *testing.T) {
	deps, out := newDependencies(t, "myOrg")

	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		Return(&work.TeamFieldValues{Values: &[]work.TeamFieldValue{}}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam", "--json=areaPath,includeChildren,isDefault"})
	err := cmd.Execute()
	require.NoError(t, err)

	assert.Equal(t, "[]\n", out.String())
}

func TestList_TargetArg_ParsesOrgSlashProjectSlashTeam(t *testing.T) {
	deps, _ := newDependencies(t, "myOrg")

	var capturedArgs work.GetTeamFieldValuesArgs
	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args work.GetTeamFieldValuesArgs) (*work.TeamFieldValues, error) {
			capturedArgs = args
			return &work.TeamFieldValues{Values: &[]work.TeamFieldValue{tfv("Area/One", true)}}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/My Team"})
	err := cmd.Execute()
	require.NoError(t, err)

	require.NotNil(t, capturedArgs.Project)
	assert.Equal(t, "myProject", *capturedArgs.Project)
	require.NotNil(t, capturedArgs.Team)
	assert.Equal(t, "My Team", *capturedArgs.Team)
}

func TestList_TargetArg_UsesDefaultOrganization(t *testing.T) {
	deps, _ := newDependencies(t, "defaultOrg")
	deps.cmd.EXPECT().Config().Return(deps.config, nil).AnyTimes()
	deps.config.EXPECT().Authentication().Return(deps.authCfg).AnyTimes()
	deps.authCfg.EXPECT().GetDefaultOrganization().Return("defaultOrg", nil).AnyTimes()

	var capturedArgs work.GetTeamFieldValuesArgs
	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args work.GetTeamFieldValuesArgs) (*work.TeamFieldValues, error) {
			capturedArgs = args
			return &work.TeamFieldValues{Values: &[]work.TeamFieldValue{tfv("Area/One", true)}}, nil
		})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myProject/MyTeam"})
	err := cmd.Execute()
	require.NoError(t, err)

	require.NotNil(t, capturedArgs.Project)
	assert.Equal(t, "myProject", *capturedArgs.Project)
	require.NotNil(t, capturedArgs.Team)
	assert.Equal(t, "MyTeam", *capturedArgs.Team)
}

func TestList_InvalidTargetArg(t *testing.T) {
	run := func(arg string) {
		t.Helper()

		deps, _ := newDependencies(t, "myOrg")
		cmd := NewCmd(deps.cmd)
		cmd.SetArgs([]string{arg})

		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected")
	}

	run("myOrg")
	run("myOrg/myProject/MyTeam/extra")
}

func TestList_WorkClientFactoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFact := mocks.NewMockClientFactory(ctrl)
	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFact).AnyTimes()
	clientFact.EXPECT().Work(gomock.Any(), "myOrg").Return(nil, errors.New("boom"))
	cmd := NewCmd(cmdCtx)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "failed to create Work client")
	require.ErrorContains(t, err, "boom")
}

func TestList_GetTeamFieldValuesError(t *testing.T) {
	deps, _ := newDependencies(t, "myOrg")

	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("api failed"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "failed to fetch team field values")
	require.ErrorContains(t, err, "api failed")
}

func TestList_PrinterError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &dependencies{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		workClient: mocks.NewMockWorkClient(ctrl),
	}
	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().Work(gomock.Any(), "myOrg").Return(deps.workClient, nil).AnyTimes()
	_ = out

	values := []work.TeamFieldValue{tfv("A/Path", true)}
	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		Return(&work.TeamFieldValues{DefaultValue: types.ToPtr("A/Path"), Values: &values}, nil)

	deps.cmd.EXPECT().Printer("list").Return(nil, errors.New("printer failed"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "printer failed")
}

func TestList_NilResponse(t *testing.T) {
	deps, _ := newDependencies(t, "myOrg")

	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		Return(nil, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "no team area paths found")
}

func TestList_NilValues(t *testing.T) {
	deps, _ := newDependencies(t, "myOrg")

	deps.workClient.EXPECT().GetTeamFieldValues(gomock.Any(), gomock.Any()).
		Return(&work.TeamFieldValues{Values: nil}, nil)

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myOrg/myProject/MyTeam"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "no team area paths found")
}

func TestBuildView_NilResponse(t *testing.T) {
	got := buildView(nil)
	assert.Nil(t, got)
}

func TestBuildView_NilValues(t *testing.T) {
	got := buildView(&work.TeamFieldValues{Values: nil})
	assert.Nil(t, got)
}

func TestBuildView_NilRowFields(t *testing.T) {
	values := []work.TeamFieldValue{{Value: nil, IncludeChildren: nil}}
	got := buildView(&work.TeamFieldValues{
		DefaultValue: types.ToPtr("Other"),
		Values:       &values,
	})
	require.Len(t, got, 1)
	assert.Equal(t, "", got[0].AreaPath)
	assert.False(t, got[0].IncludeChildren)
	assert.False(t, got[0].IsDefault)
}

func TestBuildView_Sorting(t *testing.T) {
	values := []work.TeamFieldValue{
		tfv("z/path", false),
		tfv("A/path", false),
	}
	got := buildView(&work.TeamFieldValues{Values: &values})
	require.Len(t, got, 2)
	assert.Equal(t, "A/path", got[0].AreaPath)
	assert.Equal(t, "z/path", got[1].AreaPath)
}

func TestBuildView_DefaultMatchingIsCaseInsensitive(t *testing.T) {
	values := []work.TeamFieldValue{tfv("Fabrikam/Frontend", true)}
	got := buildView(&work.TeamFieldValues{
		DefaultValue: types.ToPtr("fabrikam/frontend"),
		Values:       &values,
	})
	require.Len(t, got, 1)
	assert.True(t, got[0].IsDefault)
}

func TestBuildView_NoMatchDefault(t *testing.T) {
	values := []work.TeamFieldValue{tfv("Fabrikam/Frontend", true)}
	got := buildView(&work.TeamFieldValues{
		DefaultValue: types.ToPtr("Other/Path"),
		Values:       &values,
	})
	require.Len(t, got, 1)
	assert.False(t, got[0].IsDefault)
}
