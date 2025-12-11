package comment_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commentcmd "github.com/tmeckel/azdo-cli/internal/cmd/pr/comment"
	igit "github.com/tmeckel/azdo-cli/internal/git"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/prompter"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

// fakePrompter is a simple test double for util.Prompter interface
type fakePrompter struct{ val string }

var _ prompter.Prompter = (*fakePrompter)(nil)

func (f *fakePrompter) Select(msg, def string, opts []string) (int, error)        { return 0, nil }
func (f *fakePrompter) MultiSelect(msg string, def, opts []string) ([]int, error) { return nil, nil }
func (f *fakePrompter) Input(label, def string) (string, error)                   { return f.val, nil }
func (f *fakePrompter) InputOrganizationName() (string, error)                    { return "", nil }
func (f *fakePrompter) Password(prompt string) (string, error)                    { return "", nil }
func (f *fakePrompter) AuthToken() (string, error)                                { return "", nil }
func (f *fakePrompter) Confirm(msg string, def bool) (bool, error)                { return false, nil }
func (f *fakePrompter) ConfirmDeletion(required string) error                     { return nil }
func (f *fakePrompter) Secret(prompt string) (result string, err error)           { return "", nil }

func setupCommonMocks(ctrl *gomock.Controller) (*mocks.MockCmdContext, *mocks.MockRepository, *mocks.MockAzDOGitClient, *mocks.MockConnectionFactory, *bytes.Buffer, *bytes.Buffer) {
	io, _, out, errOut := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	return mCmdCtx, mRepo, mGitClient, mConnFactory, out, errOut
}

func TestCommentCmd_NewThread(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx, mRepo, mGitClient, _, out, _ := setupCommonMocks(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)

	repoID := uuid.New()
	projID := uuid.New()
	// make finder resolve to PR 101 via branch config -> PR head ref
	mGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "branch").AnyTimes().Return(igit.BranchConfig{MergeRef: "refs/pull/101/head"})
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).AnyTimes().Return(&git.GitPullRequest{PullRequestId: types.ToPtr(101), Repository: &git.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Id: &projID}}}, nil)

	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mRepoCtx.EXPECT().Repo().Return(mRepo, nil).AnyTimes()
	mRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil).AnyTimes()
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).AnyTimes().Return("branch", nil)
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()

	mRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)

	mGitClient.EXPECT().CreateThread(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, args git.CreateThreadArgs) (*git.GitPullRequestCommentThread, error) {
		require.NotNil(t, args.CommentThread)
		require.GreaterOrEqual(t, len(*args.CommentThread.Comments), 1)
		assert.Equal(t, "hello world", *(*args.CommentThread.Comments)[0].Content)
		return &git.GitPullRequestCommentThread{Comments: &[]git.Comment{{Id: types.ToPtr(42)}}}, nil
	})

	cmd := commentcmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"--comment", "hello world"})
	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Created comment: 42")
}

func TestCommentCmd_ReplyToThread(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx, mRepo, mGitClient, _, out, _ := setupCommonMocks(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)

	repoID := uuid.New()

	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mRepoCtx.EXPECT().Repo().Return(mRepo, nil).AnyTimes()
	mRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil).AnyTimes()
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).AnyTimes().Return("branch", nil)
	projID := uuid.New()
	mGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "branch").AnyTimes().Return(igit.BranchConfig{RemoteName: "remoteName", MergeRef: "refs/pull/101/head"})
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).AnyTimes().Return(&git.GitPullRequest{PullRequestId: types.ToPtr(101), Repository: &git.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Id: &projID}}}, nil)
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()

	mRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)

	mGitClient.EXPECT().CreateComment(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, args git.CreateCommentArgs) (*git.Comment, error) {
		require.NotNil(t, args.Comment)
		assert.Equal(t, "reply text", *args.Comment.Content)
		return &git.Comment{Id: types.ToPtr(77)}, nil
	})

	cmd := commentcmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"--comment", "reply text", "--thread", "5"})
	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Created comment: 77")
}

func TestCommentCmd_PRNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx, mRepo, _, _, _, _ := setupCommonMocks(ctrl)

	// Simulate finder error -> PR not found
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mRepoCtx.EXPECT().Repo().Return(mRepo, nil).AnyTimes()
	mRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil).AnyTimes()
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).AnyTimes().Return("branch", nil)
	mGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "branch").AnyTimes().Return(igit.BranchConfig{MergeRef: "refs/pull/123/head"})
	mRepoCtx.EXPECT().GitClient().Return(nil, assert.AnError)

	cmd := commentcmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"--comment", "hello"})
	_, err := cmd.ExecuteC()
	require.Error(t, err)
}

func TestCommentCmd_CreateThreadFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx, mRepo, mGitClient, _, _, _ := setupCommonMocks(ctrl)

	repoID := uuid.New()
	_ = &git.GitPullRequest{PullRequestId: types.ToPtr(303), Repository: &git.GitRepository{Id: &repoID}}

	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mRepoCtx.EXPECT().Repo().Return(mRepo, nil).AnyTimes()
	mRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil).AnyTimes()
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).AnyTimes().Return("branch", nil)
	mGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "branch").AnyTimes().Return(igit.BranchConfig{MergeRef: "refs/pull/303/head"})
	projID := uuid.New()
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).AnyTimes().Return(&git.GitPullRequest{PullRequestId: types.ToPtr(303), Repository: &git.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Id: &projID}}}, nil)

	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)
	mGitClient.EXPECT().CreateThread(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	cmd := commentcmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"--comment", "thread fail"})
	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create comment thread")
}

func TestCommentCmd_CreateCommentFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx, mRepo, mGitClient, _, _, _ := setupCommonMocks(ctrl)

	repoID := uuid.New()
	_ = &git.GitPullRequest{PullRequestId: types.ToPtr(404), Repository: &git.GitRepository{Id: &repoID}}

	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mRepoCtx.EXPECT().Repo().Return(mRepo, nil).AnyTimes()
	mRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil).AnyTimes()
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).AnyTimes().Return("branch", nil)
	mGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "branch").AnyTimes().Return(igit.BranchConfig{MergeRef: "refs/pull/404/head"})
	projID := uuid.New()
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).AnyTimes().Return(&git.GitPullRequest{PullRequestId: types.ToPtr(404), Repository: &git.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Id: &projID}}}, nil)

	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)
	mGitClient.EXPECT().CreateComment(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	cmd := commentcmd.NewCmd(mCmdCtx)
	cmd.SetArgs([]string{"--comment", "reply fail", "--thread", "1"})
	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create comment")
}

func TestCommentCmd_EmptyCommentPrompts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx, mRepo, mGitClient, _, out, _ := setupCommonMocks(ctrl)
	repoID := uuid.New()
	_ = &git.GitPullRequest{PullRequestId: types.ToPtr(505), Repository: &git.GitRepository{Id: &repoID}}

	// Expect a prompt
	mCmdCtx.EXPECT().Prompter().Return(&fakePrompter{val: "prompted comment"}, nil)

	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mRepoCtx.EXPECT().Repo().Return(mRepo, nil).AnyTimes()
	mRepo.EXPECT().Project().Return("my-project").AnyTimes()
	mRepo.EXPECT().FullName().Return("my-project/my-repo").AnyTimes()
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil).AnyTimes()
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).AnyTimes().Return("branch", nil)
	mGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "branch").AnyTimes().Return(igit.BranchConfig{MergeRef: "refs/pull/505/head"})
	projID := uuid.New()
	mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).AnyTimes().Return(&git.GitPullRequest{PullRequestId: types.ToPtr(505), Repository: &git.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Id: &projID}}}, nil)

	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()
	mRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)
	mGitClient.EXPECT().CreateThread(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, args git.CreateThreadArgs) (*git.GitPullRequestCommentThread, error) {
		assert.Equal(t, "prompted comment", *(*args.CommentThread.Comments)[0].Content)
		return &git.GitPullRequestCommentThread{Comments: &[]git.Comment{{Id: types.ToPtr(88)}}}, nil
	})

	cmd := commentcmd.NewCmd(mCmdCtx)
	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Created comment: 88")
}
