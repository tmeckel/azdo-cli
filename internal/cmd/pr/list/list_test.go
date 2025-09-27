package list

import (
	"context"
	"testing"

	"github.com/google/uuid"
	core "github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	webapi "github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

// The tests below mirror the gomock style used in pr/create tests and
// validate Azure DevOps REST v7.1 search criteria for listing PRs.

func TestPRList_DefaultActive_NoFilters_JSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mCmd := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mRepo := mocks.NewMockRepository(ctrl)
	mConn := mocks.NewMockConnectionFactory(ctrl)
	mClient := mocks.NewMockClientFactory(ctrl)
	mGit := mocks.NewMockAzDOGitClient(ctrl)
	mExt := mocks.NewMockAzDOExtension(ctrl)

	// IO and context
	mCmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmd.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmd.EXPECT().ConnectionFactory().Return(mConn).AnyTimes()
	mCmd.EXPECT().ClientFactory().Return(mClient).AnyTimes()
	mCmd.EXPECT().Context().Return(context.Background()).AnyTimes()

	mRepo.EXPECT().Organization().Return("org").AnyTimes()
	mRepo.EXPECT().Project().Return("myproj").AnyTimes()
	mRepo.EXPECT().Name().Return("myrepo").AnyTimes()

	mClient.EXPECT().Git(gomock.Any(), "org").Return(mGit, nil)
	mClient.EXPECT().Extensions(gomock.Any(), "org").Return(mExt, nil)
	mRepoCtx.EXPECT().Repo().Return(mRepo, nil)

	repoID := uuid.New()
	mRepo.EXPECT().GitRepository(gomock.Any(), mGit).Return(&azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr("myproj")}}, nil)

	// Printer: use real table printer to capture output in tests
	tp, _ := printer.NewTablePrinter(out, false, 80)
	mCmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	// Expect GetPullRequests called with default status=active and default top=30
	prs := []azdogit.GitPullRequest{
		{
			PullRequestId: types.ToPtr(1),
			Title:         types.ToPtr("Fix bug"),
			SourceRefName: types.ToPtr("refs/heads/feature"),
			CreatedBy:     &webapi.IdentityRef{DisplayName: types.ToPtr("Alice"), UniqueName: types.ToPtr("alice@example.org")},
			Status:        (*azdogit.PullRequestStatus)(types.ToPtr(string(azdogit.PullRequestStatusValues.Active))),
			IsDraft:       types.ToPtr(false),
			MergeStatus:   types.ToPtr(azdogit.PullRequestAsyncStatusValues.Succeeded),
		},
		{
			PullRequestId: types.ToPtr(2),
			Title:         types.ToPtr("Add feature"),
			SourceRefName: types.ToPtr("refs/heads/feat2"),
			CreatedBy:     &webapi.IdentityRef{DisplayName: types.ToPtr("Bob"), UniqueName: types.ToPtr("bob@example.org")},
			Status:        (*azdogit.PullRequestStatus)(types.ToPtr(string(azdogit.PullRequestStatusValues.Active))),
			IsDraft:       types.ToPtr(true),
			// MergeStatus nil -> prints "unknown" in table/json
		},
	}

	mGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.GetPullRequestsArgs) (*[]azdogit.GitPullRequest, error) {
			require.NotNil(t, args.RepositoryId)
			require.NotNil(t, args.SearchCriteria)
			require.NotNil(t, args.Top)
			assert.Equal(t, repoID.String(), *args.RepositoryId)
			assert.Equal(t, 30, *args.Top)
			// Per REST 7.1, default state is active
			require.NotNil(t, args.SearchCriteria.Status)
			assert.Equal(t, azdogit.PullRequestStatusValues.Active, *args.SearchCriteria.Status)
			// No source/target filters by default
			assert.Nil(t, args.SearchCriteria.SourceRefName)
			assert.Nil(t, args.SearchCriteria.TargetRefName)
			return &prs, nil
		},
	)

	opts := &listOptions{limitResults: 30, state: string(azdogit.PullRequestStatusValues.Active)}
	err := runCmd(mCmd, opts)
	require.NoError(t, err)
	// Basic spot checks on JSON output
	s := out.String()
	assert.Contains(t, s, "Fix bug")
	assert.Contains(t, s, "Add feature")
}

func TestPRList_WithBaseHeadAndLimit_BuildsSearchCriteria(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mCmd := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mRepo := mocks.NewMockRepository(ctrl)
	mConn := mocks.NewMockConnectionFactory(ctrl)
	mClient := mocks.NewMockClientFactory(ctrl)
	mGit := mocks.NewMockAzDOGitClient(ctrl)
	mExt := mocks.NewMockAzDOExtension(ctrl)

	mCmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmd.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmd.EXPECT().ConnectionFactory().Return(mConn).AnyTimes()
	mCmd.EXPECT().ClientFactory().Return(mClient).AnyTimes()
	mCmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	tp, _ := printer.NewTablePrinter(out, false, 80)
	mCmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mRepo, nil)
	mRepo.EXPECT().Organization().Return("org").AnyTimes()
	mRepo.EXPECT().Project().Return("myproj").AnyTimes()
	mRepo.EXPECT().Name().Return("myrepo").AnyTimes()
	mClient.EXPECT().Git(gomock.Any(), "org").Return(mGit, nil)
	mClient.EXPECT().Extensions(gomock.Any(), "org").Return(mExt, nil)

	repoID := uuid.New()
	mRepo.EXPECT().GitRepository(gomock.Any(), mGit).Return(&azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr("myproj")}}, nil)

	prs := []azdogit.GitPullRequest{{
		PullRequestId: types.ToPtr(42),
		Title:         types.ToPtr("PR-42"),
		SourceRefName: types.ToPtr("refs/heads/feature-x"),
		CreatedBy:     &webapi.IdentityRef{DisplayName: types.ToPtr("Dev"), UniqueName: types.ToPtr("dev@example.org")},
		Status:        (*azdogit.PullRequestStatus)(types.ToPtr(string(azdogit.PullRequestStatusValues.Active))),
		IsDraft:       types.ToPtr(false),
	}}

	mGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.GetPullRequestsArgs) (*[]azdogit.GitPullRequest, error) {
			require.NotNil(t, args.SearchCriteria)
			assert.Equal(t, "refs/heads/feature-x", *args.SearchCriteria.SourceRefName)
			assert.Equal(t, "refs/heads/develop", *args.SearchCriteria.TargetRefName)
			// limit passed through
			require.NotNil(t, args.Top)
			assert.Equal(t, 5, *args.Top)
			return &prs, nil
		},
	)

	opts := &listOptions{baseBranch: "develop", headBranch: "feature-x", limitResults: 5, state: "active"}
	err := runCmd(mCmd, opts)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "PR-42")
}

func TestPRList_FilterByDraftLabelsAndMergeState(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mCmd := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mRepo := mocks.NewMockRepository(ctrl)
	mConn := mocks.NewMockConnectionFactory(ctrl)
	mClient := mocks.NewMockClientFactory(ctrl)
	mGit := mocks.NewMockAzDOGitClient(ctrl)
	mExt := mocks.NewMockAzDOExtension(ctrl)

	mCmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmd.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmd.EXPECT().ConnectionFactory().Return(mConn).AnyTimes()
	mCmd.EXPECT().ClientFactory().Return(mClient).AnyTimes()
	mCmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	tp, _ := printer.NewTablePrinter(out, false, 80)
	mCmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mRepo, nil)
	mRepo.EXPECT().Organization().Return("org").AnyTimes()
	mRepo.EXPECT().Project().Return("myproj").AnyTimes()
	mRepo.EXPECT().Name().Return("myrepo").AnyTimes()

	mClient.EXPECT().Git(gomock.Any(), "org").Return(mGit, nil)
	mClient.EXPECT().Extensions(gomock.Any(), "org").Return(mExt, nil)

	repoID := uuid.New()
	mRepo.EXPECT().GitRepository(gomock.Any(), mGit).Return(&azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr("myproj")}}, nil)

	lbl := func(name string) core.WebApiTagDefinition { return core.WebApiTagDefinition{Name: types.ToPtr(name)} }
	labels := []core.WebApiTagDefinition{lbl("bug"), lbl("p1")}

	prs := []azdogit.GitPullRequest{
		{
			PullRequestId: types.ToPtr(10),
			Title:         types.ToPtr("Match me"),
			SourceRefName: types.ToPtr("refs/heads/fx"),
			CreatedBy:     &webapi.IdentityRef{DisplayName: types.ToPtr("Ann"), UniqueName: types.ToPtr("ann@example.org")},
			Status:        (*azdogit.PullRequestStatus)(types.ToPtr(string(azdogit.PullRequestStatusValues.Active))),
			IsDraft:       types.ToPtr(true),
			MergeStatus:   types.ToPtr(azdogit.PullRequestAsyncStatusValues.Succeeded),
			Labels:        &labels,
		},
		{
			PullRequestId: types.ToPtr(11),
			Title:         types.ToPtr("Filtered out: not draft"),
			SourceRefName: types.ToPtr("refs/heads/fy"),
			CreatedBy:     &webapi.IdentityRef{DisplayName: types.ToPtr("Ben"), UniqueName: types.ToPtr("ben@example.org")},
			Status:        (*azdogit.PullRequestStatus)(types.ToPtr(string(azdogit.PullRequestStatusValues.Active))),
			IsDraft:       types.ToPtr(false),
			MergeStatus:   types.ToPtr(azdogit.PullRequestAsyncStatusValues.Conflicts),
			Labels:        &labels,
		},
	}

	mGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&prs, nil)

	dr := true
	opts := &listOptions{draft: &dr, labels: []string{"bug", "p1"}, mergeState: "succeeded", limitResults: 30, state: "active"}
	err := runCmd(mCmd, opts)
	require.NoError(t, err)
	s := out.String()
	assert.Contains(t, s, "Match me")
	assert.NotContains(t, s, "Filtered out: not draft")
}

func TestPRList_NoAPIResults_ReturnsNoResultsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mCmd := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mRepo := mocks.NewMockRepository(ctrl)
	mConn := mocks.NewMockConnectionFactory(ctrl)
	mClient := mocks.NewMockClientFactory(ctrl)
	mGit := mocks.NewMockAzDOGitClient(ctrl)
	mExt := mocks.NewMockAzDOExtension(ctrl)

	mCmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmd.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmd.EXPECT().ConnectionFactory().Return(mConn).AnyTimes()
	mCmd.EXPECT().ClientFactory().Return(mClient).AnyTimes()
	mCmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	// Printer won't be used since we error before rendering

	mRepoCtx.EXPECT().Repo().Return(mRepo, nil)
	mRepo.EXPECT().Organization().Return("org").AnyTimes()
	mRepo.EXPECT().Project().Return("myproj").AnyTimes()
	mRepo.EXPECT().Name().Return("myrepo").AnyTimes()

	mClient.EXPECT().Git(gomock.Any(), "org").Return(mGit, nil)
	mClient.EXPECT().Extensions(gomock.Any(), "org").Return(mExt, nil)

	repoID := uuid.New()
	mRepo.EXPECT().GitRepository(gomock.Any(), mGit).Return(&azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr("myproj")}}, nil)

	empty := []azdogit.GitPullRequest{}
	mGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	opts := &listOptions{limitResults: 30, state: "active"}
	err := runCmd(mCmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No Pull Requests found for repository")
}

func TestPRList_FiltersRemoveAll_ReturnsFilteredNoResultsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mCmd := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mRepo := mocks.NewMockRepository(ctrl)
	mConn := mocks.NewMockConnectionFactory(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mGit := mocks.NewMockAzDOGitClient(ctrl)
	mExt := mocks.NewMockAzDOExtension(ctrl)

	mCmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmd.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmd.EXPECT().ConnectionFactory().Return(mConn).AnyTimes()
	mCmd.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
	mCmd.EXPECT().Context().Return(context.Background()).AnyTimes()

	mRepo.EXPECT().Organization().Return("org").AnyTimes()
	mRepo.EXPECT().Project().Return("myproj").AnyTimes()
	mRepo.EXPECT().Name().Return("myrepo").AnyTimes()

	mClientFactory.EXPECT().Git(gomock.Any(), "org").Return(mGit, nil)
	mRepoCtx.EXPECT().Repo().Return(mRepo, nil)

	mClientFactory.EXPECT().Extensions(gomock.Any(), "org").Return(mExt, nil)

	repoID := uuid.New()
	mRepo.EXPECT().GitRepository(gomock.Any(), mGit).Return(&azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr("myproj")}}, nil)

	prs := []azdogit.GitPullRequest{{
		PullRequestId: types.ToPtr(99),
		Title:         types.ToPtr("Only not-draft PR"),
		SourceRefName: types.ToPtr("refs/heads/x"),
		CreatedBy:     &webapi.IdentityRef{DisplayName: types.ToPtr("Cara"), UniqueName: types.ToPtr("cara@example.org")},
		Status:        (*azdogit.PullRequestStatus)(types.ToPtr(string(azdogit.PullRequestStatusValues.Active))),
		IsDraft:       types.ToPtr(false),
	}}
	mGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&prs, nil)

	dr := true
	opts := &listOptions{draft: &dr, limitResults: 30, state: "active"}
	err := runCmd(mCmd, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "using specified filters")
}
