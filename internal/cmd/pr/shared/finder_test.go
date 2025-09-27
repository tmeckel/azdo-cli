package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	igit "github.com/tmeckel/azdo-cli/internal/git"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestPRRegex(t *testing.T) {
	type resultData struct {
		Org  string
		Proj string
		Repo string
		PrID int
	}
	tests := []struct {
		S          string
		wantResult resultData
		wantErr    bool
	}{
		{
			S: "my org/proj   skjfsdfkj898595838/repo sdfuia9q3459--:89",
			wantResult: resultData{
				Org:  "my org",
				Proj: "proj   skjfsdfkj898595838",
				Repo: "repo sdfuia9q3459--",
				PrID: 89,
			},
		},
		{
			S: "6477",
			wantResult: resultData{
				PrID: 6477,
			},
		},
		{
			S: "#7843",
			wantResult: resultData{
				PrID: 7843,
			},
		},
		{
			S:       "#78s43",
			wantErr: true,
		},
	}

	for _, tst := range tests {
		org, prj, repo, prID, err := parseSelector(tst.S)
		if tst.wantErr {
		} else {
		}
		require.Condition(t, func() bool {
			return (tst.wantErr && err != nil) || (!tst.wantErr && err == nil)
		})
		assert.Equal(t, tst.wantResult.Org, org)
		assert.Equal(t, tst.wantResult.Proj, prj)
		assert.Equal(t, tst.wantResult.Repo, repo)
		assert.Equal(t, tst.wantResult.PrID, prID)
	}
}

// --- Additional tests for finder.Find() ---
// These use gomock with mocks from internal/mocks to simulate RepoContext, Repository, GitClient, and GitCommand.

func TestFind_ByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()

	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(42),
		Status:        &git.PullRequestStatusValues.Active,
	}, nil)
	mAzdoRepo.EXPECT().Project().Return("proj").AnyTimes()

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	pr, repo, err := f.Find(FindOptions{Selector: "42"})

	require.NoError(t, err)
	assert.Equal(t, 42, *pr.PullRequestId)
	assert.Equal(t, mAzdoRepo, repo)
}

func TestFind_NoResults_ByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()

	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(nil, nil)
	mAzdoRepo.EXPECT().Project().Return("proj").AnyTimes()

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	_, _, err := f.Find(FindOptions{Selector: "99"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pull request not found")
	var nre util.NoResultsError
	assert.ErrorAs(t, err, &nre)
}

func TestFind_InvalidRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(nil, errors.New("boom"))

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	_, _, err := f.Find(FindOptions{Selector: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get repo")
}

func TestFind_StateFilterMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()

	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(77),
		Status:        &git.PullRequestStatusValues.Completed,
	}, nil)
	mAzdoRepo.EXPECT().Project().Return("proj").AnyTimes()

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	_, _, err := f.Find(FindOptions{Selector: "77", States: []string{"active"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in any of the specified states")
	var nre util.NoResultsError
	assert.ErrorAs(t, err, &nre)
}

func TestFind_parseCurrentBranch_GitCommandError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()
	mRepoCtx := mocks.NewMockRepoContext(ctrl)

	mRepoCtx.EXPECT().GitCommand().Return(nil, errors.New("gc boom"))

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	_, _, err := f.Find(FindOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get git command")
}

func TestFind_parseCurrentBranch_CurrentBranchError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)

	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("", errors.New("cb boom"))

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	_, _, err := f.Find(FindOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get current branch")
}

func TestFind_branchSearch_EmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitRepo := git.GitRepository{Id: types.ToPtr(uuid.New())}

	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "feature").Return(igit.BranchConfig{})

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil)
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil)
	mRepoCtx.EXPECT().GitRepository().Return(&mGitRepo, nil)

	mGitClient.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(nil, nil)

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	_, _, err := f.Find(FindOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pull request not found")
	var nre util.NoResultsError
	assert.ErrorAs(t, err, &nre)
}

func TestFind_branchSearch_ClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitRepo := git.GitRepository{Id: types.ToPtr(uuid.New())}

	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "feature").Return(igit.BranchConfig{})

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil)
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil)
	mRepoCtx.EXPECT().GitRepository().Return(&mGitRepo, nil)

	mGitClient.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(nil, errors.New("gp boom"))

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	_, _, err := f.Find(FindOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get PR list from git repo")
}

func TestFind_BaseBranchMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()

	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(101),
		Status:        &git.PullRequestStatusValues.Active,
		SourceRefName: types.ToPtr("refs/heads/other-branch"),
	}, nil)
	mAzdoRepo.EXPECT().Project().Return("proj").AnyTimes()

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	_, _, err := f.Find(FindOptions{Selector: "101", BaseBranch: "expected-branch"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not have base branch")
	var nre util.NoResultsError
	assert.ErrorAs(t, err, &nre)
}

func TestFind_GetPRById_ClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	io, _, _, _ := iostreams.Test()
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil)
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil)
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(nil, errors.New("gpid boom"))
	mAzdoRepo.EXPECT().Project().Return("proj")

	f := &finder{repoCtx: mRepoCtx, progress: io, ctx: context.Background()}
	_, _, err := f.Find(FindOptions{Selector: "1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get PR by ID")
}
