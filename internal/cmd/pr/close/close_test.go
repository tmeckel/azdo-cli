package close_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	closecmd "github.com/tmeckel/azdo-cli/internal/cmd/pr/close"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestCloseCmd_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	// CmdContext wiring
	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()

	// Finder path
	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(123),
		Status:        &git.PullRequestStatusValues.Active,
		Title:         types.ToPtr("test pr"),
	}, nil)
	mAzdoRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()

	// Close path
	repoID := uuid.New()
	mAzdoRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)
	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), mGitClient).Return(&git.GitRepository{Id: &repoID}, nil)
	mGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(123),
		SourceRefName: types.ToPtr("refs/heads/feature/my-branch"),
	}, nil)
	mGitClient.EXPECT().CreateThread(gomock.Any(), gomock.Any()).Return(&git.GitPullRequestCommentThread{}, nil)

	cmd := closecmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"123"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Equal(t, "", out.String())
	assert.Equal(t, "", errOut.String())
}

func TestCloseCmd_WithComment(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(123),
		Status:        &git.PullRequestStatusValues.Active,
		Title:         types.ToPtr("test pr"),
	}, nil)
	mAzdoRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()

	repoID := uuid.New()
	mAzdoRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)
	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), mGitClient).Return(&git.GitRepository{Id: &repoID}, nil)
	mGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(123),
	}, nil)
	mGitClient.EXPECT().CreateThread(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args git.CreateThreadArgs) (*git.GitPullRequestCommentThread, error) {
			require.NotNil(t, args.CommentThread)
			require.NotNil(t, args.CommentThread.Comments)
			require.GreaterOrEqual(t, len(*args.CommentThread.Comments), 1)
			require.NotNil(t, (*args.CommentThread.Comments)[0].Content)
			assert.Equal(t, "closing this pr", *(*args.CommentThread.Comments)[0].Content)
			return &git.GitPullRequestCommentThread{}, nil
		},
	)

	cmd := closecmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"-c", "closing this pr", "123"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Equal(t, "", out.String())
	assert.Equal(t, "", errOut.String())
}

func TestCloseCmd_DeleteBranch(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil).AnyTimes()

	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(123),
		Status:        &git.PullRequestStatusValues.Active,
		Title:         types.ToPtr("test pr"),
		SourceRefName: types.ToPtr("refs/heads/feature/my-branch"),
	}, nil)
	mAzdoRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()

	repoID := uuid.New()
	mAzdoRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)
	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), mGitClient).Return(&git.GitRepository{Id: &repoID, DefaultBranch: types.ToPtr("refs/heads/main")}, nil)
	mGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(123),
		SourceRefName: types.ToPtr("refs/heads/feature/my-branch"),
	}, nil)
	mGitClient.EXPECT().CreateThread(gomock.Any(), gomock.Any()).Return(&git.GitPullRequestCommentThread{}, nil)
	// GetRefs to resolve current object ID for deletion
	mGitClient.EXPECT().GetRefs(gomock.Any(), gomock.Any()).Return(&git.GetRefsResponseValue{
		Value: []git.GitRef{{
			Name:     types.ToPtr("refs/heads/feature/my-branch"),
			ObjectId: types.ToPtr("1111111111111111111111111111111111111111"),
		}},
	}, nil)

	// local branch flow + remote deletion
	// pretend local branch does not exist; skip local operations
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "refs/heads/feature/my-branch").Return(false).AnyTimes()
	mGitClient.EXPECT().UpdateRefs(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args git.UpdateRefsArgs) (*[]git.GitRefUpdateResult, error) {
			require.NotNil(t, args.RefUpdates)
			require.GreaterOrEqual(t, len(*args.RefUpdates), 1)
			ru := (*args.RefUpdates)[0]
			require.NotNil(t, ru.Name)
			assert.Equal(t, "refs/heads/feature/my-branch", *ru.Name)
			require.NotNil(t, ru.OldObjectId)
			assert.Equal(t, "1111111111111111111111111111111111111111", *ru.OldObjectId)
			require.NotNil(t, ru.NewObjectId)
			assert.Equal(t, "0000000000000000000000000000000000000000", *ru.NewObjectId)
			return &[]git.GitRefUpdateResult{}, nil
		},
	).AnyTimes()

	cmd := closecmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"-d", "123"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Equal(t, "", out.String())
	assert.Contains(t, errOut.String(), "Deleted branch")
}

func TestCloseCmd_AlreadyClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()

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
		PullRequestId: types.ToPtr(123),
		Status:        &git.PullRequestStatusValues.Abandoned,
		Title:         types.ToPtr("test pr"),
	}, nil)
	mAzdoRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()

	cmd := closecmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"123"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Equal(t, "", out.String())
	assert.Contains(t, errOut.String(), "Unable to close pull request")
}

func TestCloseCmd_DeleteBranch_SwitchesWhenOnSourceBranch(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil).AnyTimes()

	// PR with full ref source
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(789),
		Status:        &git.PullRequestStatusValues.Active,
		Title:         types.ToPtr("pr"),
		SourceRefName: types.ToPtr("refs/heads/source-branch"),
	}, nil)
	mAzdoRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()

	repoID := uuid.New()
	mAzdoRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)
	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), mGitClient).Return(&git.GitRepository{Id: &repoID, DefaultBranch: types.ToPtr("refs/heads/main"), Name: types.ToPtr("myrepo")}, nil)
	mGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{PullRequestId: types.ToPtr(789), SourceRefName: types.ToPtr("refs/heads/source-branch")}, nil)
	mGitClient.EXPECT().CreateThread(gomock.Any(), gomock.Any()).Return(&git.GitPullRequestCommentThread{}, nil)

	// Simulate being currently on the source branch (short name)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "refs/heads/source-branch").Return(true)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("source-branch", nil)
	mGitCmd.EXPECT().CheckoutBranch(gomock.Any(), "refs/heads/main").Return(nil)
	mGitCmd.EXPECT().DeleteLocalBranch(gomock.Any(), "refs/heads/source-branch").Return(nil)

	// Resolve ref and delete remotely
	mGitClient.EXPECT().GetRefs(gomock.Any(), gomock.Any()).Return(&git.GetRefsResponseValue{
		Value: []git.GitRef{{Name: types.ToPtr("refs/heads/source-branch"), ObjectId: types.ToPtr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")}},
	}, nil)
	mGitClient.EXPECT().UpdateRefs(gomock.Any(), gomock.Any()).Return(&[]git.GitRefUpdateResult{}, nil)

	cmd := closecmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"-d", "789"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Equal(t, "", out.String())
	assert.Contains(t, errOut.String(), "Deleted branch")
}

func TestCloseCmd_DeleteBranch_FromFork_SkipsRemoteDeletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil).AnyTimes()

	// Finder PR
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(456),
		Status:        &git.PullRequestStatusValues.Active,
		Title:         types.ToPtr("fork pr"),
		SourceRefName: types.ToPtr("refs/heads/fork-branch"),
		ForkSource:    &git.GitForkRef{},
	}, nil)
	mAzdoRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()

	// Close path
	repoID := uuid.New()
	mAzdoRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)
	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), mGitClient).Return(&git.GitRepository{Id: &repoID, DefaultBranch: types.ToPtr("refs/heads/main")}, nil)
	mGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: types.ToPtr(456),
		SourceRefName: types.ToPtr("refs/heads/fork-branch"),
		ForkSource:    &git.GitForkRef{},
	}, nil)
	mGitClient.EXPECT().CreateThread(gomock.Any(), gomock.Any()).Return(&git.GitPullRequestCommentThread{}, nil)

	// Local branch checks skipped
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "refs/heads/fork-branch").Return(false).AnyTimes()

	// Ensure UpdateRefs is NOT called when ForkSource != nil by not setting expectation
	// We still need to expect Git client setup per runCmd

	cmd := closecmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"-d", "456"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Equal(t, "", out.String())
	assert.Contains(t, errOut.String(), "Skipped deleting the remote branch of a pull request from fork")
}

func TestCloseCmd_PRNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

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
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
	mAzdoRepo.EXPECT().Project().Return("my-project").AnyTimes()

	cmd := closecmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"123"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
}
