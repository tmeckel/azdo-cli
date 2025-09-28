package create

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestRunCreate_ParameterValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	ios, _, _, _ := iostreams.Test()
	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	mConfig := mocks.NewMockConfig(ctrl)
	mCmdCtx.EXPECT().Config().Return(mConfig, nil).AnyTimes()
	mAuth := mocks.NewMockAuthConfig(ctrl)
	mAuth.EXPECT().GetDefaultOrganization().Return("org1", nil).AnyTimes()
	mConfig.EXPECT().Authentication().Return(mAuth).AnyTimes()
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mGit := mocks.NewMockAzDOGitClient(ctrl)
	mClientFactory.EXPECT().Git(gomock.Any(), gomock.Any()).Return(mGit, nil).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
	mPrinter := mocks.NewMockPrinter(ctrl)
	mPrinter.EXPECT().AddColumns(gomock.Any()).AnyTimes()
	mPrinter.EXPECT().AddField(gomock.Any()).AnyTimes()
	mPrinter.EXPECT().EndRow().AnyTimes()
	mPrinter.EXPECT().Render().AnyTimes()
	mCmdCtx.EXPECT().Printer(gomock.Any()).Return(mPrinter, nil).AnyTimes()

	cases := []struct {
		name    string
		opts    *createOptions
		wantErr string
	}{
		{
			name:    "invalid new repo path too short",
			opts:    &createOptions{repo: "onlyproj"},
			wantErr: "invalid value",
		},
		{
			name:    "invalid new repo path too long",
			opts:    &createOptions{repo: "a/b/c/d"},
			wantErr: "invalid value",
		},
		{
			name:    "parent repo in different org",
			opts:    &createOptions{repo: "org1/proj1/repo1", parentRepo: "org2/pX/rX"},
			wantErr: "organization for new repo and parent repo must match",
		},
		{
			name:    "parent repo in different default org",
			opts:    &createOptions{repo: "proj1/repo1", parentRepo: "org2/pX/rX"},
			wantErr: "organization for new repo and parent repo must match",
		},
		{
			name:    "invalid parent repo path",
			opts:    &createOptions{repo: "org1/proj1/repo1", parentRepo: "a/b/c/d/e"},
			wantErr: "invalid parent value",
		},
		{
			name:    "source-branch without parent",
			opts:    &createOptions{repo: "proj1/repo1", sourceBranch: "main"},
			wantErr: "--source-branch can only be used with --parent",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := runCreate(mCmdCtx, tc.opts)
			if tc.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRunCreate_Fork(t *testing.T) {
	cases := []struct {
		name        string
		opts        *createOptions
		expectedRef *string
	}{
		{
			name: "fork without source branch",
			opts: &createOptions{repo: "proj1/repo1", parentRepo: "org1/pX/rX"},
		},
		{
			name:        "fork with source branch",
			opts:        &createOptions{repo: "proj1/repo1", parentRepo: "org1/pX/rX", sourceBranch: "main"},
			expectedRef: types.ToPtr("refs/heads/main"),
		},
		{
			name:        "fork with full source branch ref",
			opts:        &createOptions{repo: "proj1/repo1", parentRepo: "org1/pX/rX", sourceBranch: "refs/heads/main"},
			expectedRef: types.ToPtr("refs/heads/main"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mCmdCtx := mocks.NewMockCmdContext(ctrl)
			mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
			ios, _, _, _ := iostreams.Test()
			mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
			mConfig := mocks.NewMockConfig(ctrl)
			mCmdCtx.EXPECT().Config().Return(mConfig, nil).AnyTimes()
			mAuth := mocks.NewMockAuthConfig(ctrl)
			mAuth.EXPECT().GetDefaultOrganization().Return("org1", nil).AnyTimes()
			mConfig.EXPECT().Authentication().Return(mAuth).AnyTimes()
			mClientFactory := mocks.NewMockClientFactory(ctrl)
			mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
			mGit := mocks.NewMockAzDOGitClient(ctrl)
			mClientFactory.EXPECT().Git(gomock.Any(), gomock.Any()).Return(mGit, nil).AnyTimes()
			mCore := mocks.NewMockCoreClient(ctrl)
			mClientFactory.EXPECT().Core(gomock.Any(), gomock.Any()).Return(mCore, nil).AnyTimes()
			mPrinter := mocks.NewMockPrinter(ctrl)
			mCmdCtx.EXPECT().Printer(gomock.Any()).Return(mPrinter, nil).AnyTimes()
			mPrinter.EXPECT().AddColumns(gomock.Any()).AnyTimes()
			mPrinter.EXPECT().AddField(gomock.Any()).AnyTimes()
			mPrinter.EXPECT().EndRow().AnyTimes()
			mPrinter.EXPECT().Render().AnyTimes()

			mCore.EXPECT().GetProject(gomock.Any(), gomock.Any()).Return(&core.TeamProject{Id: types.ToPtr(uuid.New())}, nil)
			mGit.EXPECT().GetRepository(gomock.Any(), gomock.Any()).Return(&git.GitRepository{Id: types.ToPtr(uuid.New())}, nil)
			repo := git.GitRepository{
				Id:     types.ToPtr(uuid.New()),
				Name:   types.ToPtr("repo1"),
				WebUrl: types.ToPtr("http://example.com"),
				SshUrl: types.ToPtr("ssh://example"),
			}

			mGit.EXPECT().CreateRepository(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, args git.CreateRepositoryArgs) (*git.GitRepository, error) {
					assert.Equal(t, tc.expectedRef, args.SourceRef)
					return &repo, nil
				},
			)

			err := runCreate(mCmdCtx, tc.opts)
			assert.NoError(t, err)
		})
	}
}
func TestRunCreate_APIInvocationAndOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mMockConfig := mocks.NewMockConfig(ctrl)
	mMockAlias := mocks.NewMockAliasConfig(ctrl)
	mMockAuth := mocks.NewMockAuthConfig(ctrl)
	mMockAuth.EXPECT().GetDefaultOrganization().Return("org1", nil).AnyTimes()
	mMockConfig.EXPECT().Authentication().Return(mMockAuth).AnyTimes()
	mMockConfig.EXPECT().Aliases().Return(mMockAlias).AnyTimes()
	mCmdCtx.EXPECT().Config().Return(mMockConfig, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()

	ios, _, stdout, _ := iostreams.Test()
	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	jp, _ := printer.NewJSONPrinter(ios.Out)
	mCmdCtx.EXPECT().Printer(gomock.Any()).Return(jp, nil).AnyTimes()

	mockConnFac := mocks.NewMockConnectionFactory(ctrl)
	mCmdCtx.EXPECT().ConnectionFactory().Return(mockConnFac).AnyTimes()
	mockConnFac.EXPECT().Connection("org1").Return(nil, nil).AnyTimes()

	mockClientFac := mocks.NewMockClientFactory(ctrl)
	mCmdCtx.EXPECT().ClientFactory().Return(mockClientFac).AnyTimes()
	mockGitClient := mocks.NewMockAzDOGitClient(ctrl)
	mockClientFac.EXPECT().Git(gomock.Any(), "org1").Return(mockGitClient, nil).AnyTimes()

	// Expect repository creation call
	uuidPtr := uuid.New()
	mockGitClient.EXPECT().CreateRepository(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ interface{}, args git.CreateRepositoryArgs) (*git.GitRepository, error) {
			// Spec‑driven assertions
			require.NotNil(t, args.GitRepositoryToCreate)
			require.Equal(t, "repo1", *args.GitRepositoryToCreate.Name)
			require.NotNil(t, args.Project)
			repo := git.GitRepository{
				Id:     &uuidPtr,
				Name:   types.ToPtr(*args.GitRepositoryToCreate.Name),
				WebUrl: types.ToPtr("http://example.com"),
				SshUrl: types.ToPtr("ssh://example"),
			}
			return &repo, nil
		},
	).Times(1)

	opts := &createOptions{
		repo:   "proj1/repo1",
		format: "json",
	}

	err := runCreate(mCmdCtx, opts)
	require.NoError(t, err)
	outStr := stdout.String()
	assert.Contains(t, outStr, "repo1")
	assert.Contains(t, outStr, "proj1")
}

func TestRunCreate_NoDefaultOrganizationConfigured(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	ios, _, _, _ := iostreams.Test()
	mCmdCtx.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	mConfig := mocks.NewMockConfig(ctrl)
	mCmdCtx.EXPECT().Config().Return(mConfig, nil).AnyTimes()
	mAuth := mocks.NewMockAuthConfig(ctrl)
	mAuth.EXPECT().GetDefaultOrganization().Return("", nil).AnyTimes()
	mConfig.EXPECT().Authentication().Return(mAuth).AnyTimes()

	opts := &createOptions{
		repo:   "proj1/repo1",
		format: "table",
	}
	t.Run("EmptyDefaultOrganization", func(t *testing.T) {
		mAuth := mocks.NewMockAuthConfig(ctrl)
		mAuth.EXPECT().GetDefaultOrganization().Return("", nil).AnyTimes()
		mConfig.EXPECT().Authentication().Return(mAuth).AnyTimes()
		err := runCreate(mCmdCtx, opts)
		assert.Error(t, err)
	})
	t.Run("ErrorGettingDefaultOrganization", func(t *testing.T) {
		mAuth := mocks.NewMockAuthConfig(ctrl)
		mAuth.EXPECT().GetDefaultOrganization().Return("", fmt.Errorf("no default org")).AnyTimes()
		mConfig.EXPECT().Authentication().Return(mAuth).AnyTimes()
		err := runCreate(mCmdCtx, opts)
		assert.Error(t, err)
	})
}
