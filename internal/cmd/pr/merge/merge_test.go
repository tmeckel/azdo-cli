package merge

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/stretchr/testify/assert"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	igit "github.com/tmeckel/azdo-cli/internal/git"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestRunCmd_SuccessfulMerge_PRID(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)
	mockRepoCtx := mocks.NewMockRepoContext(ctrl)
	mockGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mockRepo := mocks.NewMockRepository(ctrl)
	mockConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mockCmdCtx.EXPECT().RepoContext().Return(mockRepoCtx).AnyTimes()
	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ConnectionFactory().Return(mockConnFactory).AnyTimes()

	prID := 123
	pr := &azdogit.GitPullRequest{
		PullRequestId: &prID,
		Status:        &azdogit.PullRequestStatusValues.Active,
	}

	mockRepoCtx.EXPECT().Repo().Return(mockRepo, nil).AnyTimes()
	mockRepoCtx.EXPECT().GitClient().Return(mockGitClient, nil)

	// prRepo interactions inside runCmd
	repoID := uuid.New()
	mockRepo.EXPECT().GitClient(gomock.Any(), mockConnFactory).Return(mockGitClient, nil)
	mockRepo.EXPECT().GitRepository(gomock.Any(), mockGitClient).Return(&azdogit.GitRepository{Id: &repoID}, nil)
	mockRepo.EXPECT().Project().Return("project").AnyTimes()
	mockRepo.EXPECT().FullName().Return("org/project/repo").AnyTimes()

	mockGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(pr, nil)
	mockGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args azdogit.UpdatePullRequestArgs) (*azdogit.GitPullRequest, error) {
		assert.Equal(t, prID, *args.PullRequestId)
		assert.NotNil(t, args.GitPullRequestToUpdate.CompletionOptions)
		opts := args.GitPullRequestToUpdate.CompletionOptions
		assert.True(t, *opts.DeleteSourceBranch)
		assert.Equal(t, "Test merge", *opts.MergeCommitMessage)
		assert.Equal(t, azdogit.GitPullRequestMergeStrategyValues.Squash, *opts.MergeStrategy)
		assert.True(t, *opts.TransitionWorkItems)
		// ensure repo/project are wired
		assert.Equal(t, repoID.String(), *args.RepositoryId)
		assert.Equal(t, "project", *args.Project)
		return pr, nil
	})

	opts := &mergeOptions{
		selectorArg:                "123",
		mergeStrategy:              "squash",
		completionMessage:          "Test merge",
		deleteSourceBranch:         true,
		transitionWorkItemStatuses: true,
	}

	err := runCmd(mockCmdCtx, opts)

	assert.NoError(t, err)
	// Validate user-facing output
	s := out.String()
	assert.Contains(t, s, "Merged pull request org/project/repo#123")
}

func TestRunCmd_SuccessfulMerge_Branch(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)
	mockRepoCtx := mocks.NewMockRepoContext(ctrl)
	mockGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mockRepo := mocks.NewMockRepository(ctrl)
	mockConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mockCmdCtx.EXPECT().RepoContext().Return(mockRepoCtx).AnyTimes()
	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ConnectionFactory().Return(mockConnFactory).AnyTimes()

	prID := 456
	pr := &azdogit.GitPullRequest{
		PullRequestId: &prID,
		Status:        &azdogit.PullRequestStatusValues.Active,
	}
	prList := []azdogit.GitPullRequest{*pr}

	mockRepoCtx.EXPECT().Repo().Return(mockRepo, nil).AnyTimes()
	mockRepoCtx.EXPECT().GitClient().Return(mockGitClient, nil)
	// current branch path: need GitCommand to supply branch name and config
	mockGitCmd := mocks.NewMockGitCommand(ctrl)
	mockRepoCtx.EXPECT().GitCommand().Return(mockGitCmd, nil)
	mockGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature-branch", nil)
	mockGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "feature-branch").Return(igit.BranchConfig{})
	repoID := uuid.New()
	mockRepoCtx.EXPECT().GitRepository().Return(&azdogit.GitRepository{Id: &repoID}, nil)
	mockRepo.EXPECT().GitClient(gomock.Any(), mockConnFactory).Return(mockGitClient, nil)
	mockRepo.EXPECT().GitRepository(gomock.Any(), mockGitClient).Return(&azdogit.GitRepository{Id: &repoID}, nil)
	mockRepo.EXPECT().Project().Return("project").AnyTimes()
	mockRepo.EXPECT().FullName().Return("org/project/repo").AnyTimes()

	mockGitClient.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args azdogit.GetPullRequestsArgs) (*[]azdogit.GitPullRequest, error) {
		assert.Equal(t, repoID.String(), *args.RepositoryId)
		assert.Equal(t, "refs/heads/feature-branch", *args.SearchCriteria.SourceRefName)
		// Finder limits to Top=1 for branch lookup
		assert.NotNil(t, args.Top)
		assert.Equal(t, 1, *args.Top)
		return &prList, nil
	})
	mockGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args azdogit.UpdatePullRequestArgs) (*azdogit.GitPullRequest, error) {
		assert.Equal(t, prID, *args.PullRequestId)
		opts := args.GitPullRequestToUpdate.CompletionOptions
		assert.False(t, *opts.DeleteSourceBranch)
		assert.Equal(t, "", *opts.MergeCommitMessage)
		assert.Equal(t, azdogit.GitPullRequestMergeStrategyValues.NoFastForward, *opts.MergeStrategy)
		assert.False(t, *opts.TransitionWorkItems)
		assert.Equal(t, repoID.String(), *args.RepositoryId)
		assert.Equal(t, "project", *args.Project)
		return pr, nil
	})

	opts := &mergeOptions{
		selectorArg:                "",
		mergeStrategy:              "noFastForward",
		completionMessage:          "",
		deleteSourceBranch:         false,
		transitionWorkItemStatuses: false,
	}

	err := runCmd(mockCmdCtx, opts)

	assert.NoError(t, err)
	s := out.String()
	assert.Contains(t, s, "Merged pull request org/project/repo#456")
}

func TestRunCmd_SuccessfulMerge_Branch_MultiplePRs_PicksFirst(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)
	mockRepoCtx := mocks.NewMockRepoContext(ctrl)
	mockGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mockRepo := mocks.NewMockRepository(ctrl)
	mockConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mockCmdCtx.EXPECT().RepoContext().Return(mockRepoCtx).AnyTimes()
	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ConnectionFactory().Return(mockConnFactory).AnyTimes()

	prID1 := 101
	prID2 := 202
	pr1 := azdogit.GitPullRequest{PullRequestId: &prID1, Status: &azdogit.PullRequestStatusValues.Active}
	pr2 := azdogit.GitPullRequest{PullRequestId: &prID2, Status: &azdogit.PullRequestStatusValues.Active}
	prList := []azdogit.GitPullRequest{pr1, pr2}

	mockRepoCtx.EXPECT().Repo().Return(mockRepo, nil).AnyTimes()
	mockRepoCtx.EXPECT().GitClient().Return(mockGitClient, nil)
	// current branch path: need GitCommand to supply branch name and config
	mockGitCmd := mocks.NewMockGitCommand(ctrl)
	mockRepoCtx.EXPECT().GitCommand().Return(mockGitCmd, nil)
	mockGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature-branch", nil)
	mockGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "feature-branch").Return(igit.BranchConfig{})

	repoID := uuid.New()
	mockRepoCtx.EXPECT().GitRepository().Return(&azdogit.GitRepository{Id: &repoID}, nil)
	mockRepo.EXPECT().GitClient(gomock.Any(), mockConnFactory).Return(mockGitClient, nil)
	mockRepo.EXPECT().GitRepository(gomock.Any(), mockGitClient).Return(&azdogit.GitRepository{Id: &repoID}, nil)
	mockRepo.EXPECT().Project().Return("project").AnyTimes()
	mockRepo.EXPECT().FullName().Return("org/project/repo").AnyTimes()

	mockGitClient.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args azdogit.GetPullRequestsArgs) (*[]azdogit.GitPullRequest, error) {
		assert.Equal(t, repoID.String(), *args.RepositoryId)
		assert.Equal(t, "refs/heads/feature-branch", *args.SearchCriteria.SourceRefName)
		// Finder limits to Top=1 for branch lookup; we still return two to ensure first is used
		assert.NotNil(t, args.Top)
		assert.Equal(t, 1, *args.Top)
		return &prList, nil
	})
	mockGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args azdogit.UpdatePullRequestArgs) (*azdogit.GitPullRequest, error) {
		assert.Equal(t, prID1, *args.PullRequestId)
		opts := args.GitPullRequestToUpdate.CompletionOptions
		assert.Equal(t, azdogit.GitPullRequestMergeStrategyValues.NoFastForward, *opts.MergeStrategy)
		assert.Equal(t, repoID.String(), *args.RepositoryId)
		assert.Equal(t, "project", *args.Project)
		// return pr1
		return &pr1, nil
	})

	opts := &mergeOptions{
		selectorArg:                "",
		mergeStrategy:              "noFastForward",
		completionMessage:          "",
		deleteSourceBranch:         false,
		transitionWorkItemStatuses: false,
	}

	err := runCmd(mockCmdCtx, opts)

	assert.NoError(t, err)
	s := out.String()
	assert.Contains(t, s, "Merged pull request org/project/repo#101")
}

func TestRunCmd_PRNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)
	mockRepoCtx := mocks.NewMockRepoContext(ctrl)
	mockGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mockRepo := mocks.NewMockRepository(ctrl)
	mockConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mockCmdCtx.EXPECT().RepoContext().Return(mockRepoCtx).AnyTimes()
	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ConnectionFactory().Return(mockConnFactory).AnyTimes()

	mockRepoCtx.EXPECT().Repo().Return(mockRepo, nil).AnyTimes()
	mockRepoCtx.EXPECT().GitClient().Return(mockGitClient, nil)
	// simulate current branch path to allow finder to return nil without state check
	mockGitCmd := mocks.NewMockGitCommand(ctrl)
	mockRepoCtx.EXPECT().GitCommand().Return(mockGitCmd, nil)
	mockGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature-branch", nil)
	mockGitCmd.EXPECT().ReadBranchConfig(gomock.Any(), "feature-branch").Return(igit.BranchConfig{})
	repoID := uuid.New()
	mockRepoCtx.EXPECT().GitRepository().Return(&azdogit.GitRepository{Id: &repoID}, nil)
	mockGitClient.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args azdogit.GetPullRequestsArgs) (*[]azdogit.GitPullRequest, error) {
		// return empty list to simulate not found
		empty := []azdogit.GitPullRequest{}
		return &empty, nil
	})

	opts := &mergeOptions{
		selectorArg:                "",
		mergeStrategy:              "rebase",
		completionMessage:          "",
		deleteSourceBranch:         false,
		transitionWorkItemStatuses: true,
	}

	err := runCmd(mockCmdCtx, opts)
	if assert.Error(t, err) {
		// Harmonized: errors.Is with util.NoResultsError
		assert.True(t, errors.Is(err, util.NoResultsError{}))
	}
}

func TestRunCmd_GitClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)
	mockRepoCtx := mocks.NewMockRepoContext(ctrl)
	mockGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mockRepo := mocks.NewMockRepository(ctrl)
	mockConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mockCmdCtx.EXPECT().RepoContext().Return(mockRepoCtx).AnyTimes()
	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ConnectionFactory().Return(mockConnFactory).AnyTimes()

	mockRepoCtx.EXPECT().Repo().Return(mockRepo, nil).AnyTimes()
	mockRepoCtx.EXPECT().GitClient().Return(mockGitClient, nil)
	mockRepo.EXPECT().Project().Return("project").AnyTimes()
	// Finder will try to get PR by ID before runCmd uses prRepo.GitClient
	mockGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&azdogit.GitPullRequest{PullRequestId: types.ToPtr(123), Status: &azdogit.PullRequestStatusValues.Active}, nil)
	// prRepo.GitClient fails next
	mockRepo.EXPECT().GitClient(gomock.Any(), mockConnFactory).Return(nil, assert.AnError)

	opts := &mergeOptions{
		selectorArg:                "123",
		mergeStrategy:              "rebaseMerge",
		completionMessage:          "",
		deleteSourceBranch:         true,
		transitionWorkItemStatuses: true,
	}

	err := runCmd(mockCmdCtx, opts)

	assert.ErrorContains(t, err, "failed to get Git REST client")
}

func TestRunCmd_UpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdoutTTY(false)

	mockCmdCtx := mocks.NewMockCmdContext(ctrl)
	mockRepoCtx := mocks.NewMockRepoContext(ctrl)
	mockGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mockRepo := mocks.NewMockRepository(ctrl)
	mockConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mockCmdCtx.EXPECT().RepoContext().Return(mockRepoCtx).AnyTimes()
	mockCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mockCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mockCmdCtx.EXPECT().ConnectionFactory().Return(mockConnFactory).AnyTimes()

	prID := 123
	pr := &azdogit.GitPullRequest{
		PullRequestId: &prID,
		Status:        &azdogit.PullRequestStatusValues.Active,
	}

	mockRepoCtx.EXPECT().Repo().Return(mockRepo, nil).AnyTimes()
	mockRepoCtx.EXPECT().GitClient().Return(mockGitClient, nil)
	repoID := uuid.New()
	mockRepo.EXPECT().GitClient(gomock.Any(), mockConnFactory).Return(mockGitClient, nil)
	mockRepo.EXPECT().GitRepository(gomock.Any(), mockGitClient).Return(&azdogit.GitRepository{Id: &repoID}, nil)
	mockRepo.EXPECT().Project().Return("project").AnyTimes()
	mockRepo.EXPECT().FullName().Return("org/project/repo").AnyTimes()

	mockGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(pr, nil)
	mockGitClient.EXPECT().UpdatePullRequest(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	opts := &mergeOptions{
		selectorArg:                "123",
		mergeStrategy:              "noFastForward",
		completionMessage:          "",
		deleteSourceBranch:         false,
		transitionWorkItemStatuses: true,
	}

	err := runCmd(mockCmdCtx, opts)

	assert.ErrorContains(t, err, "failed to merge pull request")
}
