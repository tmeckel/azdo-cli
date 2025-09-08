package create

import (
	"context"
	"testing"

	"github.com/google/uuid"
	core "github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	aidentity "github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	igit "github.com/tmeckel/azdo-cli/internal/git"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

// (intentionally empty) – helpers will be added as we implement more tests

// NOTE
// These are hollow test stubs outlining full coverage for the PR create command.
// Each test documents the intent and expectations. We will wire them up with
// proper mocks (git, REST clients, identity, IO, repo context) in a later pass.

func TestPullRequest_Create_WithTitleDescription_DefaultBaseAndHead(t *testing.T) {
	// Under test: end-to-end flow with explicit title/description,
	// base inferred from repo default, head is current branch, interactive allowed.
	// Expect: pushes head if needed, checks for existing PRs, creates PR, prints URL.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()
	// Non-interactive is fine; we provide title/description explicitly
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	// Mocks
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	// IO streams and contexts
	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repository model with default branch
	repoID := uuid.New()
	defaultBranch := "refs/heads/main"
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{
		Id: &repoID,
		Project: &core.TeamProjectReference{
			Name: types.ToPtr(projectName),
		},
		DefaultBranch: &defaultBranch,
	}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)

	// Resolve remote (only name used later)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git command interactions
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	// current == head triggers uncommitted change check
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	// head exists locally and remotely
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST git client
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	// no existing PRs
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	// Identity client is constructed but not used (no reviewers). Provide organization and identity client.
	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)

	// Expect CreatePullRequest with IsDraft=false and normalized refs
	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.CreatePullRequestArgs) (*azdogit.GitPullRequest, error) {
			// Validate target identifiers
			require.NotNil(t, args.GitPullRequestToCreate)
			pr := args.GitPullRequestToCreate
			assert.Equal(t, "refs/heads/feature", *pr.SourceRefName)
			assert.Equal(t, "refs/heads/main", *pr.TargetRefName)
			assert.Equal(t, false, *pr.IsDraft)
			assert.Equal(t, "Title", *pr.Title)
			assert.Equal(t, "Description", *pr.Description)

			// Validate repo + project routing
			require.NotNil(t, args.RepositoryId)
			require.NotNil(t, args.Project)
			assert.Equal(t, repoID.String(), *args.RepositoryId)
			assert.Equal(t, projectName, *args.Project)

			// Return created PR
			id := 100
			url := "https://example.org/pr/100"
			return &azdogit.GitPullRequest{PullRequestId: &id, Url: &url}, nil
		},
	)

	// Execute
	opts := &createOptions{
		isDraft:     false,
		title:       "Title",
		description: "Description",
		// baseBranch empty -> from repo default
		// headBranch empty -> current branch
	}

	err := runCmd(mCmdCtx, opts)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request #100 created: https://example.org/pr/100")
	_ = errOut
}

func TestPullRequest_Create_WithTitleDescription_ExplicitBaseAndHead(t *testing.T) {
	// Under test: title/description provided, base and head explicitly passed.
	// Expect: branch names normalized, remote existence validated, PR created.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	// Mocks
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	// IO streams and contexts
	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repository model (default branch should not be used because base is explicit)
	repoID := uuid.New()
	defaultBranch := "refs/heads/main"
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{
		Id: &repoID,
		Project: &core.TeamProjectReference{
			Name: types.ToPtr(projectName),
		},
		DefaultBranch: &defaultBranch,
	}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)

	// Resolve remote (origin)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git interactions
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("other", nil) // current != head
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature-x").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature-x").Return(true)

	// REST client
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	// Validate GetPullRequests called with valid criteria per 7.1 docs
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.GetPullRequestsArgs) (*[]azdogit.GitPullRequest, error) {
			require.NotNil(t, args.RepositoryId)
			require.NotNil(t, args.Project)
			require.NotNil(t, args.SearchCriteria)
			// source/target must be fully qualified refs
			assert.Equal(t, "refs/heads/feature-x", *args.SearchCriteria.SourceRefName)
			assert.Equal(t, "refs/heads/develop", *args.SearchCriteria.TargetRefName)
			// also assert status and top are set
			assert.NotNil(t, args.SearchCriteria.Status)
			assert.Equal(t, azdogit.PullRequestStatusValues.Active, *args.SearchCriteria.Status)
			assert.Equal(t, 1, *args.Top)
			return &empty, nil
		},
	)

	// Identity client for reviewers (not used here)
	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)

	// Create PR expectation
	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.CreatePullRequestArgs) (*azdogit.GitPullRequest, error) {
			require.NotNil(t, args.GitPullRequestToCreate)
			pr := args.GitPullRequestToCreate
			assert.Equal(t, "refs/heads/feature-x", *pr.SourceRefName)
			assert.Equal(t, "refs/heads/develop", *pr.TargetRefName)
			assert.Equal(t, false, *pr.IsDraft)
			assert.Equal(t, "Explicit Title", *pr.Title)
			assert.Equal(t, "Explicit Description", *pr.Description)
			// routing
			require.NotNil(t, args.RepositoryId)
			require.NotNil(t, args.Project)
			assert.Equal(t, repoID.String(), *args.RepositoryId)
			assert.Equal(t, projectName, *args.Project)
			id := 102
			url := "https://example.org/pr/102"
			return &azdogit.GitPullRequest{PullRequestId: &id, Url: &url}, nil
		},
	)

	// Execute with explicit base/head
	opts := &createOptions{
		isDraft:     false,
		title:       "Explicit Title",
		description: "Explicit Description",
		baseBranch:  "develop",
		headBranch:  "feature-x",
	}

	err := runCmd(mCmdCtx, opts)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request #102 created: https://example.org/pr/102")
	_ = errOut
}

func TestPullRequest_Create_Draft(t *testing.T) {
	// Under test: --draft flag. Expect PR created with IsDraft=true.
	// Flow: determines base from repo default branch, head from current branch,
	// ensures head exists and is on remote, verifies no existing PR, creates PR
	// with IsDraft=true, and prints created URL.

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()
	// Non-interactive is fine; we provide title/description explicitly
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	// Mocks
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	// IO streams and contexts
	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repository model with default branch
	repoID := uuid.New()
	defaultBranch := "refs/heads/main"
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{
		Id: &repoID,
		Project: &core.TeamProjectReference{
			Name: types.ToPtr(projectName),
		},
		DefaultBranch: &defaultBranch,
	}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)

	// Resolve remote (only name used later)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git command interactions
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	// current == head triggers uncommitted change check
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	// head exists locally and remotely
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST git client
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	// no existing PRs
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	// Identity client is constructed but not used (no reviewers). Provide organization and identity client.
	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)

	// Expect CreatePullRequest with IsDraft=true and normalized refs
	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.CreatePullRequestArgs) (*azdogit.GitPullRequest, error) {
			// Validate target identifiers
			require.NotNil(t, args.GitPullRequestToCreate)
			pr := args.GitPullRequestToCreate
			assert.Equal(t, "refs/heads/feature", *pr.SourceRefName)
			assert.Equal(t, "refs/heads/main", *pr.TargetRefName)
			assert.Equal(t, true, *pr.IsDraft)
			assert.Equal(t, "Draft Title", *pr.Title)
			assert.Equal(t, "Draft Description", *pr.Description)

			// Validate repo + project routing
			require.NotNil(t, args.RepositoryId)
			require.NotNil(t, args.Project)
			assert.Equal(t, repoID.String(), *args.RepositoryId)
			assert.Equal(t, projectName, *args.Project)

			// Return created PR
			id := 101
			url := "https://example.org/pr/101"
			return &azdogit.GitPullRequest{PullRequestId: &id, Url: &url}, nil
		},
	)

	// Execute
	opts := &createOptions{
		isDraft:     true,
		title:       "Draft Title",
		description: "Draft Description",
		// baseBranch empty -> from repo default
		// headBranch empty -> current branch
	}

	err := runCmd(mCmdCtx, opts)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request #101 created: https://example.org/pr/101")
	_ = errOut
}

func TestPullRequest_BaseBranch_DefaultFromRepository(t *testing.T) {
	// Under test: When --base is not specified, use repo.DefaultBranch (normalized).
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo default main, remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git current is head; head exists
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST check default base used
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.GetPullRequestsArgs) (*[]azdogit.GitPullRequest, error) {
			assert.Equal(t, "refs/heads/feature", *args.SearchCriteria.SourceRefName)
			assert.Equal(t, "refs/heads/main", *args.SearchCriteria.TargetRefName)
			return &empty, nil
		},
	)

	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)
	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.CreatePullRequestArgs) (*azdogit.GitPullRequest, error) {
			pr := args.GitPullRequestToCreate
			assert.Equal(t, "refs/heads/main", *pr.TargetRefName)
			id := 140
			url := "https://example.org/pr/140"
			return &azdogit.GitPullRequest{PullRequestId: &id, Url: &url}, nil
		},
	)

	opts := &createOptions{title: "T", description: "D"}
	err := runCmd(mCmdCtx, opts)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request #140 created:")
}

func TestPullRequest_Error_RepositoryDefaultBranchMissing(t *testing.T) {
	// Negative: repo.DefaultBranch is nil and --base not provided; expect error.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo without default branch; remote still resolved before check
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: nil}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	opts := &createOptions{title: "T", description: "D"}
	err := runCmd(mCmdCtx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repository does not specify a default branch")
}

func TestPullRequest_Error_CurrentBranchEqualsBase(t *testing.T) {
    // Negative: current branch is the same as base; cannot create PR from a branch to itself.
    ctrl := gomock.NewController(t)
    t.Cleanup(ctrl.Finish)

    io, _, _, _ := iostreams.Test()
    io.SetStdinTTY(false)
    io.SetStdoutTTY(false)

    mCmdCtx := mocks.NewMockCmdContext(ctrl)
    mRepoCtx := mocks.NewMockRepoContext(ctrl)
    mGitCmd := mocks.NewMockGitCommand(ctrl)

    mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
    mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
    mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
    mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

    // Repo + remote
    repoID := uuid.New()
    projectName := "myproj"
    repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
    mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
    mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

    // Git: current branch equals base
    mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
    mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("main", nil)

    opts := &createOptions{title: "T", description: "D", baseBranch: "main"}
    err := runCmd(mCmdCtx, opts)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "current branch 'main' is the same as base branch")
}

func TestPullRequest_Error_HeadEqualsBase(t *testing.T) {
    // Negative: head branch equals base branch; expect failure.
    ctrl := gomock.NewController(t)
    t.Cleanup(ctrl.Finish)

    io, _, _, _ := iostreams.Test()
    io.SetStdinTTY(false)
    io.SetStdoutTTY(false)

    mCmdCtx := mocks.NewMockCmdContext(ctrl)
    mRepoCtx := mocks.NewMockRepoContext(ctrl)
    mGitCmd := mocks.NewMockGitCommand(ctrl)

    mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
    mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
    mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
    mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

    // Repo + remote
    repoID := uuid.New()
    projectName := "myproj"
    repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
    mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
    mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

    // Git: current branch different, but head equals base
    mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
    mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)

    opts := &createOptions{title: "T", description: "D", baseBranch: "main", headBranch: "main"}
    err := runCmd(mCmdCtx, opts)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "head branch 'main' is the same as base branch")
}

func TestPullRequest_Error_HeadBranchDoesNotExist(t *testing.T) {
    // Negative: head branch does not exist locally nor remotely; expect error.
    ctrl := gomock.NewController(t)
    t.Cleanup(ctrl.Finish)

    io, _, _, _ := iostreams.Test()
    io.SetStdinTTY(false)
    io.SetStdoutTTY(false)

    mCmdCtx := mocks.NewMockCmdContext(ctrl)
    mRepoCtx := mocks.NewMockRepoContext(ctrl)
    mGitCmd := mocks.NewMockGitCommand(ctrl)

    mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
    mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
    mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
    mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

    // Repo + remote
    repoID := uuid.New()
    projectName := "myproj"
    repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
    mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
    mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

    // Git: current different; head does not exist locally or remotely
    mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
    mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("other", nil)
    mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(false)
    mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(false)

    opts := &createOptions{title: "T", description: "D", baseBranch: "main", headBranch: "feature"}
    err := runCmd(mCmdCtx, opts)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "head branch 'feature' does not exist")
}

func TestPullRequest_WarnsOnUncommittedChangesWhenHeadIsCurrent(t *testing.T) {
	// Under test: when current==head and there are uncommitted changes, a warning is printed to stderr.
	// Expect: no failure; warning message emitted.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, errOut := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	defaultBranch := "refs/heads/main"
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: &defaultBranch}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git current==head, uncommitted changes > 0; head exists locally
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(3, nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST clients
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	// Identity client
	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)

	// Create PR
	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.CreatePullRequestArgs) (*azdogit.GitPullRequest, error) {
			id := 110
			url := "https://example.org/pr/110"
			return &azdogit.GitPullRequest{PullRequestId: &id, Url: &url}, nil
		},
	)

	opts := &createOptions{title: "T", description: "D"}
	err := runCmd(mCmdCtx, opts)
	require.NoError(t, err)
	assert.Contains(t, errOut.String(), "Warning: current branch contains 3 uncommitted changes")
	assert.Contains(t, out.String(), "Pull request #110 created:")
}

func TestPullRequest_PushesHeadBranchWhenMissingOnRemote(t *testing.T) {
	// Under test: ensures head branch is pushed to remote when not present.
	// Expect: git.Push called; no error on success.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	defaultBranch := "refs/heads/main"
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: &defaultBranch}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git: current is different to avoid uncommitted check; head exists locally but not on remote
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("other", nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(false)
	mGitCmd.EXPECT().Push(gomock.Any(), "origin", "feature").Return(nil)

	// REST
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	// Identity client
	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)

	// Create PR
	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.CreatePullRequestArgs) (*azdogit.GitPullRequest, error) {
			id := 111
			url := "https://example.org/pr/111"
			return &azdogit.GitPullRequest{PullRequestId: &id, Url: &url}, nil
		},
	)

	opts := &createOptions{title: "T", description: "D", headBranch: "feature"}
	err := runCmd(mCmdCtx, opts)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request #111 created:")
}

func TestPullRequest_Error_PushingHeadBranchFails(t *testing.T) {
	// Negative: pushing head branch to remote fails; expect error propagated.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	defaultBranch := "refs/heads/main"
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: &defaultBranch}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git: current different, head local exists but remote missing; push fails
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("other", nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(false)
	mGitCmd.EXPECT().Push(gomock.Any(), "origin", "feature").Return(assert.AnError)

	opts := &createOptions{title: "T", description: "D", headBranch: "feature"}
	err := runCmd(mCmdCtx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to push head branch 'feature' to remote")
}

func TestPullRequest_Error_ExistingPROpen(t *testing.T) {
	// Negative: an existing active PR for source->target exists; expect error with URL included.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST existing PR returns one item
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	url := "https://example.org/pr/existing"
	prs := []azdogit.GitPullRequest{{Url: &url}}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&prs, nil)

	opts := &createOptions{title: "T", description: "D"}
	err := runCmd(mCmdCtx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Contains(t, err.Error(), url)
}

// (moved below) TestPullRequest_CreatesWithReviewers_RequiredAndOptional implemented later

func TestPullRequest_Error_ReviewerDescriptorsMissing(t *testing.T) {
	// Negative: reviewers specified but descriptors cannot be resolved; expect error from GetReviewerDescriptors.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST existing PR check passes
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	// Identity: return only one identity for two reviewer handles
	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)
	mIdentity.EXPECT().ReadIdentities(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args aidentity.ReadIdentitiesArgs) (*[]aidentity.Identity, error) {
			desc := "vssgp.Uy0xLTkt...one"
			id := uuid.New()
			out := []aidentity.Identity{{Descriptor: &desc, Id: &id}}
			return &out, nil
		},
	)

	opts := &createOptions{title: "T", description: "D", requiredReviewer: []string{"a@example.org"}, optionalReviewer: []string{"b@example.org"}}
	err := runCmd(mCmdCtx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get reviewer descriptors")
}

func TestPullRequest_CreatesWithReviewers_RequiredAndOptional(t *testing.T) {
	// Under test: required and optional reviewers specified; descriptors resolved and set correctly.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST existing PR check passes
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	// Identity: return descriptors for required and optional
	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)
	mIdentity.EXPECT().ReadIdentities(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args aidentity.ReadIdentitiesArgs) (*[]aidentity.Identity, error) {
			d1, d2 := "vssgp.Uy0xLTkt...alice", "vssgp.Uy0xLTkt...bob"
			id1, id2 := uuid.New(), uuid.New()
			out := []aidentity.Identity{{Descriptor: &d1, Id: &id1}, {Descriptor: &d2, Id: &id2}}
			return &out, nil
		},
	)

	// Create PR expects reviewers with required/optional flags
	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.CreatePullRequestArgs) (*azdogit.GitPullRequest, error) {
			pr := args.GitPullRequestToCreate
			require.NotNil(t, pr.Reviewers)
			rs := *pr.Reviewers
			require.Len(t, rs, 2)
			assert.True(t, *rs[0].IsRequired)
			assert.False(t, *rs[1].IsRequired)
			id := 130
			url := "https://example.org/pr/130"
			return &azdogit.GitPullRequest{PullRequestId: &id, Url: &url}, nil
		},
	)

	opts := &createOptions{title: "T", description: "D", requiredReviewer: []string{"alice@example.org"}, optionalReviewer: []string{"bob@example.org"}}
	err := runCmd(mCmdCtx, opts)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request #130 created:")
}

func TestPullRequest_Error_IdentityClientCreationFailure(t *testing.T) {
	// Negative: identity client creation fails; expect error wrapped.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git current==head; head exists
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST existing PR check passes with empty
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	// Identity client creation fails
	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(nil, assert.AnError)

	opts := &createOptions{title: "T", description: "D"}
	err := runCmd(mCmdCtx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create Identity client")
}

func TestPullRequest_Error_GitRestClientFailure(t *testing.T) {
	// Negative: acquiring Git REST client from RepoContext fails; expect error.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git current==head; head exists
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// Fail to get Git client
	mRepoCtx.EXPECT().GitClient().Return(nil, assert.AnError)

	opts := &createOptions{title: "T", description: "D"}
	err := runCmd(mCmdCtx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get Git REST client")
}

func TestPullRequest_Error_GetCurrentBranchFailure(t *testing.T) {
	// Negative: git.CurrentBranch errors; expect failure.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git current branch fails
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("", assert.AnError)

	opts := &createOptions{title: "T", description: "D"}
	err := runCmd(mCmdCtx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get current branch")
}

func TestPullRequest_Error_RemoteResolutionFailure(t *testing.T) {
	// Negative: no matching remote found for repository; expect error from RepoContext.Remote.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(nil, assert.AnError)

	opts := &createOptions{title: "T", description: "D"}
	err := runCmd(mCmdCtx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find local remote for repository")
}

func TestPullRequest_Error_CreatePullRequestFailure(t *testing.T) {
	// Negative: REST call to create the PR fails; expect wrapped error.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git current==head; head exists
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST existing PR check passes
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	// Identity client ok
	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)

	// Create PR fails
	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	opts := &createOptions{title: "T", description: "D"}
	err := runCmd(mCmdCtx, opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create pull request")
}

func TestPullRequest_HeadBranchProvidedWithRefsHeads_Normalized(t *testing.T) {
	// Under test: passing --head as "refs/heads/feature" is normalized to "feature" internally.
	// Expect: git checks use trimmed name; REST uses fully qualified name.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git: current != head; HasLocal/Remote called with trimmed head name
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("other", nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST calls
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).Return(&empty, nil)

	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)

	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.CreatePullRequestArgs) (*azdogit.GitPullRequest, error) {
			pr := args.GitPullRequestToCreate
			assert.Equal(t, "refs/heads/feature", *pr.SourceRefName)
			assert.Equal(t, "refs/heads/main", *pr.TargetRefName)
			id := 120
			url := "https://example.org/pr/120"
			return &azdogit.GitPullRequest{PullRequestId: &id, Url: &url}, nil
		},
	)

	opts := &createOptions{title: "T", description: "D", headBranch: "refs/heads/feature"}
	err := runCmd(mCmdCtx, opts)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request #120 created:")
}

func TestPullRequest_BaseBranchProvidedWithRefsHeads_Normalized(t *testing.T) {
	// Under test: passing --base as "refs/heads/develop" is normalized to "develop" internally, and
	// REST calls still receive fully qualified refs.
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdinTTY(false)
	io.SetStdoutTTY(false)

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mGitCmd := mocks.NewMockGitCommand(ctrl)
	mRestGit := mocks.NewMockAzDOGitClient(ctrl)
	mAzRepo := mocks.NewMockRepository(ctrl)
	mIdentity := mocks.NewMockIdentityClient(ctrl)
	mConnFactory := mocks.NewMockConnectionFactory(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().ConnectionFactory().Return(mConnFactory).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mRepoCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Repo + remote
	repoID := uuid.New()
	projectName := "myproj"
	repoModel := &azdogit.GitRepository{Id: &repoID, Project: &core.TeamProjectReference{Name: types.ToPtr(projectName)}, DefaultBranch: types.ToPtr("refs/heads/main")}
	mRepoCtx.EXPECT().GitRepository().Return(repoModel, nil)
	mRepoCtx.EXPECT().Remote(repoModel).Return(&azdo.Remote{Remote: &igit.Remote{Name: "origin"}}, nil)

	// Git
	mRepoCtx.EXPECT().GitCommand().Return(mGitCmd, nil)
	mGitCmd.EXPECT().CurrentBranch(gomock.Any()).Return("feature", nil)
	mGitCmd.EXPECT().UncommittedChangeCount(gomock.Any()).Return(0, nil)
	mGitCmd.EXPECT().HasLocalBranch(gomock.Any(), "feature").Return(true)
	mGitCmd.EXPECT().HasRemoteBranch(gomock.Any(), "origin", "feature").Return(true)

	// REST
	mRepoCtx.EXPECT().GitClient().Return(mRestGit, nil)
	empty := []azdogit.GitPullRequest{}
	mRestGit.EXPECT().GetPullRequests(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.GetPullRequestsArgs) (*[]azdogit.GitPullRequest, error) {
			assert.Equal(t, "refs/heads/feature", *args.SearchCriteria.SourceRefName)
			assert.Equal(t, "refs/heads/develop", *args.SearchCriteria.TargetRefName)
			return &empty, nil
		},
	)

	mRepoCtx.EXPECT().Repo().Return(mAzRepo, nil)
	mAzRepo.EXPECT().Organization().Return("org")
	mConnFactory.EXPECT().Identity(gomock.Any(), "org").Return(mIdentity, nil)

	mRestGit.EXPECT().CreatePullRequest(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args azdogit.CreatePullRequestArgs) (*azdogit.GitPullRequest, error) {
			pr := args.GitPullRequestToCreate
			assert.Equal(t, "refs/heads/develop", *pr.TargetRefName)
			id := 121
			url := "https://example.org/pr/121"
			return &azdogit.GitPullRequest{PullRequestId: &id, Url: &url}, nil
		},
	)

	opts := &createOptions{title: "T", description: "D", baseBranch: "refs/heads/develop"}
	err := runCmd(mCmdCtx, opts)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Pull request #121 created:")
}
