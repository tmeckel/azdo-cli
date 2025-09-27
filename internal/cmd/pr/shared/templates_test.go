package shared

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// mockReadCloser is a helper to simulate io.ReadCloser for testing
type mockReadCloser struct {
	buffer *bytes.Buffer
	err    error
}

func (mrc *mockReadCloser) Read(p []byte) (n int, err error) {
	if mrc.err != nil {
		return 0, mrc.err
	}
	return mrc.buffer.Read(p)
}

func (mrc *mockReadCloser) Close() error {
	return nil
}

func newMockReadCloser(content []byte, readErr error) io.ReadCloser {
	return &mockReadCloser{
		buffer: bytes.NewBuffer(content),
		err:    readErr,
	}
}

var _ io.ReadCloser = (*mockReadCloser)(nil)

func TestTemplateManager_GetTemplate_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ios, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl) // Add this
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	// Mock the call to RepoContext.GitClient() made by NewTemplateManager
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()

	// Mock the call to repo.GitRepository() made by GetTemplate
	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed to get repository"))

	tm, err := NewTemplateManager(mCmdCtx)
	require.NoError(t, err)

	_, err = tm.GetTemplate(context.Background(), mAzdoRepo, "main") // Pass mAzdoRepo here
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get repository")
}

func TestTemplateManager_GetTemplate_NoDefaultBranch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ios, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mockGitRepo := &git.GitRepository{
		Id: types.ToPtr(uuid.New()),
		// DefaultBranch is nil
	}
	mRepoCtx.EXPECT().GitRepository().Return(mockGitRepo, nil).AnyTimes()
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes() // Needed for NewTemplateManager

	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), gomock.Any()).Return(mockGitRepo, nil).AnyTimes() // Add this
	mAzdoRepo.EXPECT().FullName().Return("org/proj/repo").AnyTimes() // For error message

	tm, err := NewTemplateManager(mCmdCtx)
	require.NoError(t, err)

	_, err = tm.GetTemplate(context.Background(), mAzdoRepo, "main")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repository org/proj/repo does not have a default branch")
}

func TestTemplateManager_GetTemplate_GetItemContentNon404Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ios, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mockGitRepo := &git.GitRepository{
		Id:            types.ToPtr(uuid.New()),
		DefaultBranch: types.ToPtr("refs/heads/main"),
	}
	mRepoCtx.EXPECT().GitRepository().Return(mockGitRepo, nil).AnyTimes() // Mock GitRepository call
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()      // Needed for NewTemplateManager

	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), gomock.Any()).Return(mockGitRepo, nil).AnyTimes() // Add this
	mAzdoRepo.EXPECT().Project().Return("proj").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("org/proj/repo").AnyTimes()

	// Expect GetItemContent to be called for the first path and return a non-404 error
	mGitClient.EXPECT().
		GetItemContent(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(ctx context.Context, args git.GetItemContentArgs) (io.ReadCloser, error) {
				assert.Equal(t, mockGitRepo.Id.String(), *args.RepositoryId)
				return nil, errors.New("network error")
			},
		).AnyTimes()

	tm, err := NewTemplateManager(mCmdCtx)
	require.NoError(t, err)

	_, err = tm.GetTemplate(context.Background(), mAzdoRepo, "feature-branch")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get content for template")
	assert.Contains(t, err.Error(), "network error")
}

func TestTemplateManager_GetTemplate_ReadContentError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ios, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mockGitRepo := &git.GitRepository{
		Id:            types.ToPtr(uuid.New()),
		DefaultBranch: types.ToPtr("refs/heads/main"),
	}
	mRepoCtx.EXPECT().GitRepository().Return(mockGitRepo, nil).AnyTimes() // Mock GitRepository call
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()      // Needed for NewTemplateManager

	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), gomock.Any()).Return(mockGitRepo, nil).AnyTimes() // Add this
	mAzdoRepo.EXPECT().Project().Return("proj").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("org/proj/repo").AnyTimes()

	// Expect GetItemContent to return a mockReadCloser that errors on Read
	mGitClient.EXPECT().GetItemContent(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args git.GetItemContentArgs) (io.ReadCloser, error) {
		assert.Equal(t, mockGitRepo.Id.String(), *args.RepositoryId)
		return newMockReadCloser([]byte("some content"), errors.New("read error")), nil
	}).AnyTimes()

	tm, err := NewTemplateManager(mCmdCtx)
	require.NoError(t, err)

	_, err = tm.GetTemplate(context.Background(), mAzdoRepo, "feature-branch")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read content for template")
	assert.Contains(t, err.Error(), "read error")
}

func TestTemplateManager_GetTemplate_NoTemplateFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ios, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mockGitRepo := &git.GitRepository{
		Id:            types.ToPtr(uuid.New()),
		DefaultBranch: types.ToPtr("refs/heads/main"),
	}
	mRepoCtx.EXPECT().GitRepository().Return(mockGitRepo, nil).AnyTimes() // Mock GitRepository call
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()      // Needed for NewTemplateManager

	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), gomock.Any()).Return(mockGitRepo, nil).AnyTimes() // Add this
	mAzdoRepo.EXPECT().Project().Return("proj").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("org/proj/repo").AnyTimes()

	// Expect GetItemContent to be called for ALL possible paths and return 404
	mGitClient.EXPECT().GetItemContent(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args git.GetItemContentArgs) (io.ReadCloser, error) {
		assert.Equal(t, mockGitRepo.Id.String(), *args.RepositoryId)
		return nil, &azuredevops.WrappedError{
			StatusCode: types.ToPtr(404),
			Message:    types.ToPtr("Not Found"), // Fix: Message needs to be *string
		}
	}).AnyTimes()

	tm, err := NewTemplateManager(mCmdCtx)
	require.NoError(t, err)

	template, err := tm.GetTemplate(context.Background(), mAzdoRepo, "non-existent-branch")
	assert.NoError(t, err) // Should not return an error, just nil template
	assert.Nil(t, template)
}

func TestTemplateManager_GetTemplate_EmptyTemplateFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ios, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mockGitRepo := &git.GitRepository{
		Id:            types.ToPtr(uuid.New()),
		DefaultBranch: types.ToPtr("refs/heads/main"),
	}
	mRepoCtx.EXPECT().GitRepository().Return(mockGitRepo, nil).AnyTimes() // Mock GitRepository call
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()      // Needed for NewTemplateManager

	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), gomock.Any()).Return(mockGitRepo, nil).AnyTimes() // Add this
	mAzdoRepo.EXPECT().Project().Return("proj").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("org/proj/repo").AnyTimes()

	// Expect GetItemContent to return an empty content stream for the first path
	mGitClient.EXPECT().GetItemContent(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args git.GetItemContentArgs) (io.ReadCloser, error) {
		assert.Equal(t, mockGitRepo.Id.String(), *args.RepositoryId)
		return newMockReadCloser([]byte(""), nil), nil
	}).AnyTimes()

	tm, err := NewTemplateManager(mCmdCtx)
	require.NoError(t, err)

	template, err := tm.GetTemplate(context.Background(), mAzdoRepo, "feature-branch")
	assert.NoError(t, err)
	assert.Nil(t, template) // Should be nil because empty content is treated as not found
}

func TestTemplateManager_GetTemplate_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ios, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mRepoCtx := mocks.NewMockRepoContext(ctrl)
	mAzdoRepo := mocks.NewMockRepository(ctrl)
	mGitClient := mocks.NewMockAzDOGitClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	mockGitRepo := &git.GitRepository{
		Id:            types.ToPtr(uuid.New()),
		DefaultBranch: types.ToPtr("refs/heads/main"),
	}
	mRepoCtx.EXPECT().GitRepository().Return(mockGitRepo, nil).AnyTimes() // Mock GitRepository call
	mRepoCtx.EXPECT().GitClient().Return(mGitClient, nil).AnyTimes()      // Needed for NewTemplateManager

	mAzdoRepo.EXPECT().GitRepository(gomock.Any(), gomock.Any()).Return(mockGitRepo, nil).AnyTimes() // Add this
	mAzdoRepo.EXPECT().Project().Return("proj").AnyTimes()
	mAzdoRepo.EXPECT().FullName().Return("org/proj/repo").AnyTimes()

	// Simulate a 404 for the first few paths, then a successful find
	gomock.InOrder(
		mGitClient.EXPECT().GetItemContent(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args git.GetItemContentArgs) (io.ReadCloser, error) {
			assert.Equal(t, mockGitRepo.Id.String(), *args.RepositoryId)
			return nil, &azuredevops.WrappedError{
				StatusCode: types.ToPtr(404),
				Message:    types.ToPtr("Not Found"), // Fix: Message needs to be *string
			}
		}),
		mGitClient.EXPECT().GetItemContent(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args git.GetItemContentArgs) (io.ReadCloser, error) {
			assert.Equal(t, mockGitRepo.Id.String(), *args.RepositoryId)
			return nil, &azuredevops.WrappedError{
				StatusCode: types.ToPtr(404),
				Message:    types.ToPtr("Not Found"), // Fix: Message needs to be *string
			}
		}),
		mGitClient.EXPECT().GetItemContent(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, args git.GetItemContentArgs) (io.ReadCloser, error) {
			assert.Equal(t, mockGitRepo.Id.String(), *args.RepositoryId)
			return newMockReadCloser([]byte("## My Template\n\nThis is a test template."), nil), nil
		}),
	)

	tm, err := NewTemplateManager(mCmdCtx)
	require.NoError(t, err)

	template, err := tm.GetTemplate(context.Background(), mAzdoRepo, "feature-branch")
	assert.NoError(t, err)
	assert.NotNil(t, template)
	assert.Contains(t, string(template.Body()), "## My Template")
	// Path assertion is tricky with AnyTimes, but the content confirms success
}
