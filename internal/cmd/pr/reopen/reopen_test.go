package reopen_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	reopencmd "github.com/tmeckel/azdo-cli/internal/cmd/pr/reopen"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"go.uber.org/mock/gomock"
)

func TestReopenCmd_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	// CmdContext expectations
	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()

	// Finder expectations: reopened PR must be abandoned initially
	repoID := uuid.New()
	prID := 101
	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: &prID,
		Repository:    &git.GitRepository{Id: &repoID},
		Status:        &git.PullRequestStatusValues.Abandoned,
	}, nil).AnyTimes()
	mAzdoRepo.EXPECT().Project().Return("project").AnyTimes()

	// Call to GitClient from reopen path
	mAzdoRepo.EXPECT().GitClient(gomock.Any(), mConnFactory).Return(mGitClient, nil)
	mGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: &prID,
		Repository:    &git.GitRepository{Id: &repoID},
	}, nil)

	cmd := reopencmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"101"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request 101 reopened successfully.")
	assert.Equal(t, "", errOut.String())
}

func TestReopenCmd_WithCommentFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	prID := 202
	repoID := uuid.New()

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mAzdoRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: &prID,
		Repository:    &git.GitRepository{Id: &repoID},
		Status:        &git.PullRequestStatusValues.Abandoned,
	}, nil).AnyTimes()
	mAzdoRepo.EXPECT().Project().Return("project").AnyTimes()

	mAzdoRepo.EXPECT().GitClient(gomock.Any(), mConnFactory).Return(mGitClient, nil)
	mGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
		PullRequestId: &prID,
		Repository:    &git.GitRepository{Id: &repoID},
	}, nil)
	mGitClient.EXPECT().CreateComment(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed comment"))

	cmd := reopencmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"-c", "my comment", "202"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request 202 reopened successfully.")
	assert.Contains(t, errOut.String(), "Warning: Failed to add comment")
}

func TestReopenCmd_NoPRFound(t *testing.T) {
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
	mAzdoRepo.EXPECT().Project().Return("project").AnyTimes()

	cmd := reopencmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"303"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
    assert.Contains(t, err.Error(), "failed to find abandoned pull request")
}
