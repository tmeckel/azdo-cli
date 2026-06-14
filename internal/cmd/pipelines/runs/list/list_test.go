package list

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
)

type dependencies struct {
	cmd        *mocks.MockCmdContext
	clientFact *mocks.MockClientFactory
	build      *mocks.MockBuildClient
	ext        *mocks.MockAzDOExtension
	ident      *mocks.MockIdentityClient
	cfg        *mocks.MockConfig
	auth       *mocks.MockAuthConfig
	stdout     *bytes.Buffer
	stderr     *bytes.Buffer
}

func newDependencies(t *testing.T, organization string) *dependencies {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, serr := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	deps := &dependencies{
		cmd:        mocks.NewMockCmdContext(ctrl),
		clientFact: mocks.NewMockClientFactory(ctrl),
		build:      mocks.NewMockBuildClient(ctrl),
		ext:        mocks.NewMockAzDOExtension(ctrl),
		ident:      mocks.NewMockIdentityClient(ctrl),
		cfg:        mocks.NewMockConfig(ctrl),
		auth:       mocks.NewMockAuthConfig(ctrl),
		stdout:     out,
		stderr:     serr,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ClientFactory().Return(deps.clientFact).AnyTimes()
	deps.clientFact.EXPECT().Build(gomock.Any(), organization).Return(deps.build, nil).AnyTimes()

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	return deps
}

func newDependenciesWithConfig(t *testing.T, defaultOrg string) *dependencies {
	deps := newDependencies(t, defaultOrg)
	deps.cmd.EXPECT().Config().Return(deps.cfg, nil).AnyTimes()
	deps.cfg.EXPECT().Authentication().Return(deps.auth).AnyTimes()
	deps.auth.EXPECT().GetDefaultOrganization().Return(defaultOrg, nil).AnyTimes()
	return deps
}

func sampleBuild(id int) build.Build {
	idPtr := id
	bnum := strconv.Itoa(id)
	pipelineName := "MyPipeline"
	sourceBranch := "refs/heads/main"
	dispName := "Alice"
	uniqName := "alice@x.com"
	return build.Build{
		Id:           &idPtr,
		BuildNumber:  &bnum,
		Status:       &build.BuildStatusValues.Completed,
		Result:       &build.BuildResultValues.Succeeded,
		Reason:       &build.BuildReasonValues.Manual,
		Definition:   &build.DefinitionReference{Name: &pipelineName, Id: &idPtr},
		SourceBranch: &sourceBranch,
		RequestedFor: &webapi.IdentityRef{DisplayName: &dispName, UniqueName: &uniqName},
		StartTime:    &azuredevops.Time{},
		FinishTime:   &azuredevops.Time{},
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

func TestRunList_DefaultNoFilters(t *testing.T) {
	t.Parallel()
	deps := newDependencies(t, "MyOrg")

	builds := []build.Build{sampleBuild(1)}
	deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
		Return(&build.GetBuildsResponseValue{Value: builds, ContinuationToken: ""}, nil)

	err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam"})
	require.NoError(t, err)
}

func TestRunList_ScopeParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		scopeArg string
		wantOrg  string
		wantProj string
		withCfg  bool
	}{
		{name: "project without org uses config default", scopeArg: "Fabrikam", wantOrg: "default-org", wantProj: "Fabrikam", withCfg: true},
		{name: "org/project parses both parts", scopeArg: "MyOrg/Fabrikam", wantOrg: "MyOrg", wantProj: "Fabrikam", withCfg: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deps *dependencies
			if tt.withCfg {
				deps = newDependenciesWithConfig(t, tt.wantOrg)
			} else {
				deps = newDependencies(t, tt.wantOrg)
			}

			builds := []build.Build{sampleBuild(1)}
			deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
					assert.Equal(t, tt.wantProj, *args.Project)
					return &build.GetBuildsResponseValue{Value: builds, ContinuationToken: ""}, nil
				})

			err := runCmd(deps.cmd, &runOptions{scopeArg: tt.scopeArg})
			require.NoError(t, err)
		})
	}
}

func TestRunList_PipelineID(t *testing.T) {
	t.Parallel()
	deps := newDependencies(t, "MyOrg")

	deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			require.NotNil(t, args.Definitions)
			require.Len(t, *args.Definitions, 1)
			assert.Equal(t, 42, (*args.Definitions)[0])
			return &build.GetBuildsResponseValue{ContinuationToken: ""}, nil
		})

	err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam", pipelineIDs: []int{42}})
	require.NoError(t, err)
}

func TestRunList_BranchRefsHeadsPrepended(t *testing.T) {
	t.Parallel()
	deps := newDependencies(t, "MyOrg")

	deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			require.NotNil(t, args.BranchName)
			assert.Equal(t, "refs/heads/main", *args.BranchName)
			return &build.GetBuildsResponseValue{ContinuationToken: ""}, nil
		})

	err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam", branches: []string{"main"}})
	require.NoError(t, err)
}

func TestRunList_BranchRefsUnchanged(t *testing.T) {
	t.Parallel()
	deps := newDependencies(t, "MyOrg")

	deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			require.NotNil(t, args.BranchName)
			assert.Equal(t, "refs/tags/v1.0", *args.BranchName)
			return &build.GetBuildsResponseValue{ContinuationToken: ""}, nil
		})

	err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam", branches: []string{"refs/tags/v1.0"}})
	require.NoError(t, err)
}

func TestRunList_InvalidFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts runOptions
		want string
	}{
		{name: "invalid status", opts: runOptions{scopeArg: "MyOrg/Fabrikam", statuses: []string{"INVALID_STATUS"}}, want: "unknown --status"},
		{name: "invalid result", opts: runOptions{scopeArg: "MyOrg/Fabrikam", results: []string{"INVALID_RESULT"}}, want: "unknown --result"},
		{name: "invalid reason", opts: runOptions{scopeArg: "MyOrg/Fabrikam", reasons: []string{"INVALID_REASON"}}, want: "unknown --reason"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newDependencies(t, "MyOrg")
			err := runCmd(deps.cmd, &tt.opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRunList_RequestedForAtMe(t *testing.T) {
	t.Parallel()
	deps := newDependencies(t, "MyOrg")

	selfID := uuid.New()
	aliceUUID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	dispName := "Alice"
	identities := []identity.Identity{
		{ProviderDisplayName: &dispName, Id: &aliceUUID},
	}

	deps.clientFact.EXPECT().Extensions(gomock.Any(), "MyOrg").Return(deps.ext, nil)
	deps.clientFact.EXPECT().Identity(gomock.Any(), "MyOrg").Return(deps.ident, nil)
	deps.ext.EXPECT().GetSelfID(gomock.Any()).Return(selfID, nil)

	idStr := selfID.String()
	deps.ident.EXPECT().ReadIdentities(gomock.Any(), identity.ReadIdentitiesArgs{IdentityIds: &idStr}).
		Return(&identities, nil)

	var capturedRequestedFor string
	deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			if args.RequestedFor != nil {
				capturedRequestedFor = *args.RequestedFor
			}
			return &build.GetBuildsResponseValue{ContinuationToken: ""}, nil
		})

	err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam", requestedFor: "@me"})
	require.NoError(t, err)
	assert.Equal(t, "Alice", capturedRequestedFor)
}

func TestRunList_QueryOrder(t *testing.T) {
	t.Parallel()
	deps := newDependencies(t, "MyOrg")

	deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			require.NotNil(t, args.QueryOrder)
			assert.Equal(t, build.BuildQueryOrderValues.QueueTimeDescending, *args.QueryOrder)
			return &build.GetBuildsResponseValue{ContinuationToken: ""}, nil
		})

	err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam", queryOrder: "queueTimeDescending"})
	require.NoError(t, err)
}

func TestRunList_Top(t *testing.T) {
	t.Parallel()
	deps := newDependencies(t, "MyOrg")

	deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
			require.NotNil(t, args.Top)
			assert.Equal(t, 50, *args.Top)
			return &build.GetBuildsResponseValue{ContinuationToken: ""}, nil
		})

	err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam", top: 50})
	require.NoError(t, err)
}

func TestRunList_Pagination(t *testing.T) {
	t.Parallel()

	t.Run("paginates across pages with token propagation", func(t *testing.T) {
		deps := newDependencies(t, "MyOrg")

		page1 := []build.Build{sampleBuild(1), sampleBuild(2)}
		page2 := []build.Build{sampleBuild(3)}

		var capturedToken string
		firstCall := true
		deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
				if firstCall {
					assert.Nil(t, args.ContinuationToken, "first call must have no token")
					firstCall = false
				} else {
					require.NotNil(t, args.ContinuationToken)
					capturedToken = *args.ContinuationToken
				}
				if capturedToken == "" {
					return &build.GetBuildsResponseValue{Value: page1, ContinuationToken: "next-token"}, nil
				}
				return &build.GetBuildsResponseValue{Value: page2, ContinuationToken: ""}, nil
			}).Times(2)

		err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam"})
		require.NoError(t, err)
		assert.Equal(t, "next-token", capturedToken)

		output := deps.stdout.String()
		assert.Contains(t, output, "3")
	})

	t.Run("max-items truncates and skips remaining pages", func(t *testing.T) {
		deps := newDependencies(t, "MyOrg")

		builds := []build.Build{sampleBuild(1), sampleBuild(2), sampleBuild(3)}
		deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, args build.GetBuildsArgs) (*build.GetBuildsResponseValue, error) {
				assert.Nil(t, args.ContinuationToken, "only first page fetched")
				return &build.GetBuildsResponseValue{Value: builds, ContinuationToken: "more-token"}, nil
			}).Times(1)

		err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam", maxItems: 2})
		require.NoError(t, err)

		output := deps.stdout.String()
		assert.Contains(t, output, "1")
		assert.Contains(t, output, "2")
		assert.NotContains(t, output, "3")
	})
}

func TestRunList_JSONOutput(t *testing.T) {
	t.Parallel()
	deps := newDependencies(t, "MyOrg")

	builds := []build.Build{sampleBuild(1)}
	deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
		Return(&build.GetBuildsResponseValue{Value: builds, ContinuationToken: ""}, nil)

	spy := &spyExporter{}
	err := runCmd(deps.cmd, &runOptions{
		scopeArg: "MyOrg/Fabrikam",
		exporter: spy,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, spy.writes)
	require.NotNil(t, spy.got)

	gotBuilds, ok := spy.got.([]build.Build)
	require.True(t, ok, "exporter must receive []build.Build")
	require.Len(t, gotBuilds, 1)
	assert.Equal(t, 1, *gotBuilds[0].Id)
}

func TestRunList_TableOutput(t *testing.T) {
	t.Parallel()
	deps := newDependencies(t, "MyOrg")

	builds := []build.Build{sampleBuild(1)}
	deps.build.EXPECT().GetBuilds(gomock.Any(), gomock.Any()).
		Return(&build.GetBuildsResponseValue{Value: builds, ContinuationToken: ""}, nil)

	err := runCmd(deps.cmd, &runOptions{scopeArg: "MyOrg/Fabrikam"})
	require.NoError(t, err)

	output := deps.stdout.String()
	assert.Contains(t, output, "1\t1\tcompleted\tsucceeded\tmanual\tMyPipeline\trefs/heads/main\tAlice")
}
