package vote

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"go.uber.org/mock/gomock"
)

func TestMapVote(t *testing.T) {
	cases := map[string]int{
		"approve":                  10,
		"approve-with-suggestions": 5,
		"reset":                    0,
		"wait-for-author":          -5,
		"reject":                   -10,
	}
	for in, want := range cases {
		got, err := mapVote(in)
		if err != nil {
			t.Fatalf("mapVote(%q) unexpected error: %v", in, err)
		}
		if got != want {
			t.Fatalf("mapVote(%q) = %d, want %d", in, got, want)
		}
	}

	if _, err := mapVote("nope"); err == nil {
		t.Fatalf("mapVote invalid value expected error")
	}
}

func TestPRVote_Approve_ByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	mCmd := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mRepo := mocks.NewMockRepository(ctrl)
	mConnFac := mocks.NewMockConnectionFactory(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mGit := mocks.NewMockAzDOGitClient(ctrl)
	mExt := mocks.NewMockAzDOExtension(ctrl)

	// IO and contexts
	mCmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmd.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmd.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
	mCmd.EXPECT().ConnectionFactory().Return(mConnFac).AnyTimes()
	mCmd.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Resolve PR by ID via Finder
	mRepoCtx.EXPECT().Repo().Return(mRepo, nil)
	mRepoCtx.EXPECT().GitClient().Return(mGit, nil)
	prID := 123
	mRepo.EXPECT().Project().Return("project").AnyTimes()
	mGit.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{PullRequestId: &prID}, nil)

	// Vote path: get git client and repo details
	mRepo.EXPECT().GitClient(gomock.Any(), mConnFac).Return(mGit, nil)
	repoID := uuid.New()
	mRepo.EXPECT().GitRepository(gomock.Any(), mGit).Return(&git.GitRepository{Id: &repoID}, nil)

	// Org/connection and fullname
	mRepo.EXPECT().Organization().Return("org").AnyTimes()
	mClientFactory.EXPECT().Extensions(gomock.Any(), "org").Return(mExt, nil)
	mExt.EXPECT().GetSelfID(gomock.Any()).Return(uuid.MustParse("00000000-0000-0000-0000-000000000000"), nil)
	mRepo.EXPECT().FullName().Return("org/project/repo")

	// Expect CreatePullRequestReviewer called
	mGit.EXPECT().CreatePullRequestReviewer(gomock.Any(), gomock.Any()).Return(&git.IdentityRefWithVote{}, nil)

	opts := &voteOptions{selectorArg: fmt.Sprint(prID), vote: "approve"}
	err := runCmd(mCmd, opts)
	require.NoError(t, err)
	require.Contains(t, out.String(), "Set vote to 'approve'")
}

func TestPRVote_Reject_ByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	mCmd := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mRepo := mocks.NewMockRepository(ctrl)
	mConnFac := mocks.NewMockConnectionFactory(ctrl)
	mGit := mocks.NewMockAzDOGitClient(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)

	mCmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmd.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmd.EXPECT().ConnectionFactory().Return(mConnFac).AnyTimes()
	mCmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmd.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mRepo.EXPECT().Organization().Return("org").AnyTimes()
	mRepo.EXPECT().Project().Return("project").AnyTimes()
	repoID := uuid.New()
	mRepo.EXPECT().FullName().Return(fmt.Sprintf("org/project/%s", repoID.String()))
	mRepo.EXPECT().GitClient(gomock.Any(), mConnFac).Return(mGit, nil)
	mRepo.EXPECT().GitRepository(gomock.Any(), mGit).Return(&git.GitRepository{Id: &repoID}, nil)

	prID := 456
	mGit.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{PullRequestId: &prID}, nil)
	mGit.EXPECT().CreatePullRequestReviewer(gomock.Any(), gomock.Any()).Return(&git.IdentityRefWithVote{}, nil)

	mExt := mocks.NewMockAzDOExtension(ctrl)
	mExt.EXPECT().GetSelfID(gomock.Any()).Return(uuid.MustParse("00000000-0000-0000-0000-000000000000"), nil)

	mClientFactory.EXPECT().Extensions(mCmd.Context(), mRepo.Organization()).Return(mExt, nil).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mRepo, nil)
	mRepoCtx.EXPECT().GitClient().Return(mGit, nil)

	opts := &voteOptions{selectorArg: fmt.Sprint(prID), vote: "reject"}
	err := runCmd(mCmd, opts)
	require.NoError(t, err)
	require.Contains(t, out.String(), "Set vote to 'reject'")
}

func TestPRVote_APIError_Propagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()

	mCmd := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mRepo := mocks.NewMockRepository(ctrl)
	mConnFac := mocks.NewMockConnectionFactory(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mGit := mocks.NewMockAzDOGitClient(ctrl)
	mExt := mocks.NewMockAzDOExtension(ctrl)

	mCmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmd.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmd.EXPECT().ConnectionFactory().Return(mConnFac).AnyTimes()
	mCmd.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
	mCmd.EXPECT().Context().Return(context.Background()).AnyTimes()

	mRepoCtx.EXPECT().Repo().Return(mRepo, nil)
	mRepoCtx.EXPECT().GitClient().Return(mGit, nil)
	prID := 789
	mRepo.EXPECT().Project().Return("project").AnyTimes()
	mGit.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{PullRequestId: &prID}, nil)

	mRepo.EXPECT().GitClient(gomock.Any(), mConnFac).Return(mGit, nil)
	repoID := uuid.New()
	mRepo.EXPECT().GitRepository(gomock.Any(), mGit).Return(&git.GitRepository{Id: &repoID}, nil)

	mRepo.EXPECT().Organization().Return("org").AnyTimes()
	mClientFactory.EXPECT().Extensions(gomock.Any(), "org").Return(mExt, nil)
	mExt.EXPECT().GetSelfID(gomock.Any()).Return(uuid.MustParse("00000000-0000-0000-0000-000000000000"), nil)

	mGit.EXPECT().CreatePullRequestReviewer(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("boom"))

	opts := &voteOptions{selectorArg: fmt.Sprint(prID), vote: "approve"}
	err := runCmd(mCmd, opts)
	require.Error(t, err)
}
