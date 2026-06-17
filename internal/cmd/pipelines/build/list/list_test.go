package list

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deps struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	build      *mocks.MockBuildClient
	ext        *mocks.MockAzDOExtension
	ident      *mocks.MockIdentityClient
	config     *mocks.MockConfig
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
		ctrl:       ctrl,
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		build:      mocks.NewMockBuildClient(ctrl),
		ext:        mocks.NewMockAzDOExtension(ctrl),
		ident:      mocks.NewMockIdentityClient(ctrl),
		stdout:     out,
	}

	d.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	d.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	d.cmd.EXPECT().ClientFactory().Return(d.clientFact).AnyTimes()
	if organization != "" {
		d.clientFact.EXPECT().Build(gomock.Any(), organization).Return(d.build, nil).AnyTimes()
	}

	return d
}

func (d *deps) setupDefaultOrg(org string) {
	d.config = mocks.NewMockConfig(d.ctrl)
	d.auth = mocks.NewMockAuthConfig(d.ctrl)
	d.cmd.EXPECT().Config().Return(d.config, nil).AnyTimes()
	d.config.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func sampleBuild(id int, number, status, result, reason, defName, branch, requestedFor string) build.Build {
	b := build.Build{
		Id:           types.ToPtr(id),
		BuildNumber:  types.ToPtr(number),
		Status:       buildStatusPtr(status),
		Result:       buildResultPtr(result),
		Reason:       buildReasonPtr(reason),
		SourceBranch: types.ToPtr(branch),
		StartTime:    &azuredevops.Time{Time: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)},
		FinishTime:   &azuredevops.Time{Time: time.Date(2025, 1, 15, 11, 30, 0, 0, time.UTC)},
	}
	if defName != "" {
		b.Definition = &build.DefinitionReference{Name: types.ToPtr(defName)}
	}
	if requestedFor != "" {
		b.RequestedFor = &webapi.IdentityRef{DisplayName: types.ToPtr(requestedFor)}
	}
	return b
}

func buildStatusPtr(s string) *build.BuildStatus {
	if s == "" {
		return nil
	}
	v := build.BuildStatus(s)
	return &v
}

func buildResultPtr(s string) *build.BuildResult {
	if s == "" {
		return nil
	}
	v := build.BuildResult(s)
	return &v
}

func buildReasonPtr(s string) *build.BuildReason {
	if s == "" {
		return nil
	}
	v := build.BuildReason(s)
	return &v
}

func TestNewCmd(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	assert.Equal(t, "list [ORGANIZATION/]PROJECT", cmd.Use)
	assert.ElementsMatch(t, []string{"ls", "l"}, cmd.Aliases)
	assert.NotNil(t, cmd.RunE)
}

func TestList_EmptyResult(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "org")
	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).Return(&build.GetBuildsResponseValue{}, nil)

	tp, err := printer.NewTablePrinter(d.stdout, false, 200)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(d.cmd, &listOptions{scopeArg: "org/Fabrikam"})
	require.NoError(t, err)
	assert.Empty(t, d.stdout.String())
}

func TestList_NoFilters(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "org")

	builds := []build.Build{
		sampleBuild(1, "1", "completed", "succeeded", "manual", "web-app", "refs/heads/main", "Alice"),
		sampleBuild(2, "2", "completed", "failed", "pullRequest", "api", "refs/heads/feature/x", "Bob"),
	}
	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).Return(&build.GetBuildsResponseValue{Value: builds}, nil)

	tp, err := printer.NewTablePrinter(d.stdout, false, 200)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(d.cmd, &listOptions{scopeArg: "org/Fabrikam"})
	require.NoError(t, err)

	output := d.stdout.String()
	assert.Contains(t, output, "1")
	assert.Contains(t, output, "web-app")
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "Bob")
}

func TestList_ContinuationToken_Loops(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "org")
	callCount := 0

	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			callCount++
			switch callCount {
			case 1:
				assert.Nil(t, args.ContinuationToken, "first call has no token")
				return &build.GetBuildsResponseValue{
					Value:             []build.Build{sampleBuild(1, "1", "completed", "succeeded", "manual", "web", "main", "Alice")},
					ContinuationToken: "tok-1",
				}, nil
			case 2:
				assert.NotNil(t, args.ContinuationToken)
				assert.Equal(t, "tok-1", *args.ContinuationToken)
				return &build.GetBuildsResponseValue{
					Value:             []build.Build{sampleBuild(2, "2", "completed", "failed", "pullRequest", "api", "feature/x", "Bob")},
					ContinuationToken: "",
				}, nil
			default:
				return nil, fmt.Errorf("unexpected call %d", callCount)
			}
		},
	).Times(2)

	tp, err := printer.NewTablePrinter(d.stdout, false, 200)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(d.cmd, &listOptions{scopeArg: "org/Fabrikam"})
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
	assert.Contains(t, d.stdout.String(), "1")
	assert.Contains(t, d.stdout.String(), "2")
}

func TestList_FiltersPassedToSDK(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "org")

	var capturedArgs build.GetBuildsArgs
	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			capturedArgs = args
			return &build.GetBuildsResponseValue{Value: []build.Build{sampleBuild(1, "1", "completed", "succeeded", "manual", "web", "main", "Alice")}}, nil
		},
	)

	d.clientFact.EXPECT().Extensions(gomock.Any(), "org").Return(d.ext, nil)
	d.ext.EXPECT().ResolveCurrentIdentity(gomock.Any()).Return(&identity.Identity{
		Properties:          map[string]any{"Account": map[string]any{"$value": "Alice <alice@x.com>"}},
		ProviderDisplayName: types.ToPtr("Alice"),
	}, nil)

	tp, err := printer.NewTablePrinter(d.stdout, false, 200)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(d.cmd, &listOptions{
		scopeArg:      "org/Fabrikam",
		definitionIDs: []int{1, 2},
		branch:        types.ToPtr("main"),
		buildNumber:   types.ToPtr("build-*"),
		status:        types.ToPtr("completed"),
		result:        types.ToPtr("succeeded"),
		reason:        types.ToPtr("manual"),
		tags:          []string{"release"},
		requestedFor:  types.ToPtr("@me"),
		top:           50,
	})
	require.NoError(t, err)

	require.NotNil(t, capturedArgs.Project)
	assert.Equal(t, "Fabrikam", *capturedArgs.Project)
	require.NotNil(t, capturedArgs.Definitions)
	assert.Equal(t, []int{1, 2}, *capturedArgs.Definitions)
	require.NotNil(t, capturedArgs.BranchName)
	assert.Equal(t, "refs/heads/main", *capturedArgs.BranchName)
	require.NotNil(t, capturedArgs.BuildNumber)
	assert.Equal(t, "build-*", *capturedArgs.BuildNumber)
	require.NotNil(t, capturedArgs.StatusFilter)
	assert.Equal(t, build.BuildStatus("completed"), *capturedArgs.StatusFilter)
	require.NotNil(t, capturedArgs.ResultFilter)
	assert.Equal(t, build.BuildResult("succeeded"), *capturedArgs.ResultFilter)
	require.NotNil(t, capturedArgs.ReasonFilter)
	assert.Equal(t, build.BuildReason("manual"), *capturedArgs.ReasonFilter)
	require.NotNil(t, capturedArgs.TagFilters)
	assert.Equal(t, []string{"release"}, *capturedArgs.TagFilters)
	require.NotNil(t, capturedArgs.RequestedFor)
	assert.Equal(t, "Alice <alice@x.com>", *capturedArgs.RequestedFor)
	require.NotNil(t, capturedArgs.Top)
	assert.Equal(t, 50, *capturedArgs.Top)
}

func TestList_MaxItemsCap(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "org")

	items := make([]build.Build, 5)
	for i := 0; i < 5; i++ {
		items[i] = sampleBuild(i+1, fmt.Sprintf("%d", i+1), "completed", "succeeded", "manual", "web", "main", "Alice")
	}
	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).Return(&build.GetBuildsResponseValue{Value: items}, nil)

	tp, err := printer.NewTablePrinter(d.stdout, false, 200)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(d.cmd, &listOptions{scopeArg: "org/Fabrikam", maxItems: 3})
	require.NoError(t, err)
	assert.Contains(t, d.stdout.String(), "1")
	assert.Contains(t, d.stdout.String(), "3")
	assert.NotContains(t, d.stdout.String(), "4")
}

func TestList_JSONOutput(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "org")

	items := []build.Build{sampleBuild(42, "42", "completed", "succeeded", "manual", "web-app", "main", "Alice")}
	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).Return(&build.GetBuildsResponseValue{Value: items}, nil)

	exporter := util.NewJSONExporter()
	err := runList(d.cmd, &listOptions{scopeArg: "org/Fabrikam", exporter: exporter})
	require.NoError(t, err)

	var parsed []map[string]any
	err = json.Unmarshal(d.stdout.Bytes(), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, float64(42), parsed[0]["id"])
	assert.Equal(t, "42", parsed[0]["buildNumber"])
	assert.Equal(t, "completed", parsed[0]["status"])
	assert.Equal(t, "succeeded", parsed[0]["result"])
}

func TestList_PaginationAcrossPages_CappedByMaxItems(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "org")
	callCount := 0

	page1 := make([]build.Build, 5)
	for i := 0; i < 5; i++ {
		page1[i] = sampleBuild(i+1, fmt.Sprintf("%d", i+1), "completed", "succeeded", "manual", "web", "main", "Alice")
	}
	page2 := make([]build.Build, 5)
	for i := 0; i < 5; i++ {
		page2[i] = sampleBuild(i+6, fmt.Sprintf("%d", i+6), "completed", "succeeded", "manual", "web", "main", "Alice")
	}

	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			callCount++
			switch callCount {
			case 1:
				return &build.GetBuildsResponseValue{Value: page1, ContinuationToken: "tok-1"}, nil
			case 2:
				return &build.GetBuildsResponseValue{Value: page2}, nil
			default:
				return nil, fmt.Errorf("unexpected call %d", callCount)
			}
		},
	).Times(2)

	tp, err := printer.NewTablePrinter(d.stdout, false, 200)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(d.cmd, &listOptions{scopeArg: "org/Fabrikam", maxItems: 6})
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
	assert.Contains(t, d.stdout.String(), "6")
	assert.NotContains(t, d.stdout.String(), "7")
	assert.NotContains(t, d.stdout.String(), "8")
}

func TestList_RequestedFor_ResolvesAtMe(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "org")

	d.clientFact.EXPECT().Extensions(gomock.Any(), "org").Return(d.ext, nil)
	d.ext.EXPECT().ResolveCurrentIdentity(gomock.Any()).Return(&identity.Identity{
		Properties:          map[string]any{"Account": map[string]any{"$value": "Alice <alice@x.com>"}},
		ProviderDisplayName: types.ToPtr("Alice"),
	}, nil)

	var capturedArgs build.GetBuildsArgs
	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			capturedArgs = args
			return &build.GetBuildsResponseValue{}, nil
		},
	)

	tp, err := printer.NewTablePrinter(d.stdout, false, 200)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(d.cmd, &listOptions{scopeArg: "org/Fabrikam", requestedFor: types.ToPtr("@me")})
	require.NoError(t, err)

	require.NotNil(t, capturedArgs.RequestedFor)
	assert.Equal(t, "Alice <alice@x.com>", *capturedArgs.RequestedFor)
}

func TestList_ScopeArg_ParsesOrgSlashProject(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "")

	var buildOrg string
	d.clientFact.EXPECT().Build(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, org string) (*mocks.MockBuildClient, error) {
			buildOrg = org
			return d.build, nil
		},
	).AnyTimes()

	var project string
	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			project = *args.Project
			return &build.GetBuildsResponseValue{}, nil
		},
	)

	tp, err := printer.NewTablePrinter(d.stdout, false, 200)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(d.cmd, &listOptions{scopeArg: "myOrg/myProject"})
	require.NoError(t, err)
	assert.Equal(t, "myOrg", buildOrg)
	assert.Equal(t, "myProject", project)
}

func TestList_ScopeArg_DefaultOrg(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "")
	d.setupDefaultOrg("default-org")

	d.clientFact.EXPECT().Build(gomock.Any(), "default-org").Return(d.build, nil).AnyTimes()

	var project string
	d.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			project = *args.Project
			return &build.GetBuildsResponseValue{}, nil
		},
	)

	tp, err := printer.NewTablePrinter(d.stdout, false, 200)
	require.NoError(t, err)
	d.cmd.EXPECT().Printer("list").Return(tp, nil).AnyTimes()

	err = runList(d.cmd, &listOptions{scopeArg: "myProject"})
	require.NoError(t, err)
	assert.Equal(t, "myProject", project)
}

func TestList_InvalidMaxItems(t *testing.T) {
	t.Parallel()
	d := newDeps(t, "")
	err := runList(d.cmd, &listOptions{scopeArg: "org/Fabrikam", maxItems: -1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--max-items")
}
