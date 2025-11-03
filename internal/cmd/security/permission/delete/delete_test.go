package delete

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestDelete_SuccessIgnoresInheritedEntries(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensions := mocks.NewMockAzDOExtension(ctrl)
	mSecurity := mocks.NewMockSecurityClient(ctrl)

	ctx := context.Background()
	org := "org"
	subject := "user@example.com"
	target := org + "/" + subject
	namespaceID := "11111111-1111-1111-1111-111111111111"
	namespaceUUID := uuid.MustParse(namespaceID)
	token := "repoV2/project/repo"
	descriptor := "vssgp.Subject"

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().RemoveAccessControlEntries(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.RemoveAccessControlEntriesArgs) (*bool, error) {
			require.NotNil(t, args.SecurityNamespaceId)
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			require.NotNil(t, args.Token)
			require.Equal(t, token, *args.Token)
			require.NotNil(t, args.Descriptors)
			require.Equal(t, descriptor, *args.Descriptors)
			return types.ToPtr(true), nil
		},
	)

	mSecurity.EXPECT().QueryAccessControlLists(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QueryAccessControlListsArgs) (*[]security.AccessControlList, error) {
			require.NotNil(t, args.Descriptors)
			require.Equal(t, descriptor, *args.Descriptors)
			require.NotNil(t, args.Token)
			require.Equal(t, token, *args.Token)

			zero := 0
			ace := security.AccessControlEntry{
				Descriptor: types.ToPtr(descriptor),
				Allow:      &zero,
				Deny:       &zero,
			}
			acls := []security.AccessControlList{
				{
					Token: types.ToPtr(token),
					AcesDictionary: &map[string]security.AccessControlEntry{
						descriptor: ace,
					},
				},
			}
			return &acls, nil
		},
	)
	opts := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		yes:         true,
	}

	err := runCommand(mCmdCtx, opts)
	require.NoError(t, err)
	require.Equal(t, "Permissions deleted.\n", out.String())
}

func TestDelete_SuccessSkipsDefaultTokenEntry(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensions := mocks.NewMockAzDOExtension(ctrl)
	mSecurity := mocks.NewMockSecurityClient(ctrl)

	ctx := context.Background()
	org := "org"
	subject := "user@example.com"
	target := org + "/" + subject
	namespaceID := "22222222-2222-2222-2222-222222222222"
	namespaceUUID := uuid.MustParse(namespaceID)
	token := "repoV2"
	descriptor := "vssgp.Subject"

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().RemoveAccessControlEntries(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.RemoveAccessControlEntriesArgs) (*bool, error) {
			require.NotNil(t, args.SecurityNamespaceId)
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			require.NotNil(t, args.Token)
			require.Equal(t, token, *args.Token)
			require.NotNil(t, args.Descriptors)
			require.Equal(t, descriptor, *args.Descriptors)
			return types.ToPtr(true), nil
		},
	)

	mSecurity.EXPECT().QueryAccessControlLists(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QueryAccessControlListsArgs) (*[]security.AccessControlList, error) {
			require.NotNil(t, args.Descriptors)
			require.Equal(t, descriptor, *args.Descriptors)
			require.NotNil(t, args.Token)
			require.Equal(t, token, *args.Token)

			acls := []security.AccessControlList{
				{
					Token: types.ToPtr(token),
					AcesDictionary: &map[string]security.AccessControlEntry{
						descriptor: {
							Descriptor: types.ToPtr(descriptor),
							Allow:      types.ToPtr(0),
							Deny:       types.ToPtr(0),
						},
					},
				},
			}
			return &acls, nil
		},
	)

	opts := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		yes:         true,
	}

	err := runCommand(mCmdCtx, opts)
	require.NoError(t, err)
	require.Equal(t, "Permissions deleted.\n", out.String())
}

func TestDelete_PromptDeclinedReturnsCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensions := mocks.NewMockAzDOExtension(ctrl)
	mSecurity := mocks.NewMockSecurityClient(ctrl)
	mPrompter := mocks.NewMockPrompter(ctrl)

	ctx := context.Background()
	org := "org"
	subject := "user@example.com"
	target := org + "/" + subject
	namespaceID := "11111111-1111-1111-1111-111111111111"
	token := "repo"
	descriptor := "vssgp.Subject"

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mCmdCtx.EXPECT().Prompter().Return(mPrompter, nil)
	mPrompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)

	mSecurity.EXPECT().RemoveAccessControlEntries(gomock.Any(), gomock.Any()).Times(0)
	mSecurity.EXPECT().QueryAccessControlLists(gomock.Any(), gomock.Any()).Times(0)

	opts := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
	}

	err := runCommand(mCmdCtx, opts)
	require.ErrorIs(t, err, util.ErrCancel)
	require.Empty(t, out.String())
}

func TestDelete_InvalidNamespaceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()

	opts := &opts{
		rawTarget:   "org/user@example.com",
		namespaceID: "not-a-guid",
		token:       "repo",
		yes:         true,
	}

	err := runCommand(mCmdCtx, opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid namespace id")
}

func TestDelete_ResolveIdentityError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensions := mocks.NewMockAzDOExtension(ctrl)

	ctx := context.Background()
	org := "org"
	subject := "user@example.com"

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(nil, fmt.Errorf("boom"))

	opts := &opts{
		rawTarget:   org + "/" + subject,
		namespaceID: "11111111-1111-1111-1111-111111111111",
		token:       "repo",
		yes:         true,
	}

	err := runCommand(mCmdCtx, opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to resolve identity")
}

func TestDelete_RemoveAccessControlEntriesFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensions := mocks.NewMockAzDOExtension(ctrl)
	mSecurity := mocks.NewMockSecurityClient(ctrl)

	ctx := context.Background()
	org := "org"
	subject := "user@example.com"
	descriptor := "vssgp.Subject"

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().RemoveAccessControlEntries(ctx, gomock.Any()).Return(types.ToPtr(false), nil)

	opts := &opts{
		rawTarget:   org + "/" + subject,
		namespaceID: "11111111-1111-1111-1111-111111111111",
		token:       "repo",
		yes:         true,
	}

	err := runCommand(mCmdCtx, opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "service returned no confirmation")
}

func TestDelete_DescriptorStillPresentAfterRemoval(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensions := mocks.NewMockAzDOExtension(ctrl)
	mSecurity := mocks.NewMockSecurityClient(ctrl)

	ctx := context.Background()
	org := "org"
	subject := "user@example.com"
	target := org + "/" + subject
	namespaceID := "11111111-1111-1111-1111-111111111111"
	namespaceUUID := uuid.MustParse(namespaceID)
	token := "repo"
	descriptor := "vssgp.Subject"

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().RemoveAccessControlEntries(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.RemoveAccessControlEntriesArgs) (*bool, error) {
			require.NotNil(t, args.SecurityNamespaceId)
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			return types.ToPtr(true), nil
		},
	)

	mSecurity.EXPECT().QueryAccessControlLists(ctx, gomock.Any()).Return(&[]security.AccessControlList{
		{
			Token: types.ToPtr(token),
			AcesDictionary: &map[string]security.AccessControlEntry{
				descriptor: {
					Descriptor: types.ToPtr(descriptor),
					Allow:      types.ToPtr(4),
				},
			},
		},
	}, nil)

	opts := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		yes:         true,
	}

	err := runCommand(mCmdCtx, opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "still has permissions")
}

func TestDelete_ProjectScopedTargetResolvesDescriptor(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensions := mocks.NewMockAzDOExtension(ctrl)
	mSecurity := mocks.NewMockSecurityClient(ctrl)
	mCore := mocks.NewMockCoreClient(ctrl)
	mGraph := mocks.NewMockGraphClient(ctrl)

	ctx := context.Background()
	org := "org"
	project := "ProjectAlpha"
	subject := "user@example.com"
	target := org + "/" + project + "/" + subject
	namespaceID := "11111111-1111-1111-1111-111111111111"
	namespaceUUID := uuid.MustParse(namespaceID)
	token := "repoV2/project/repo"
	descriptor := "vssgp.Subject"
	projectID := uuid.New()
	projectDescriptor := "vssps://ProjectDescriptor"

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Core(ctx, org).Return(mCore, nil)
	mCore.EXPECT().GetProject(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args core.GetProjectArgs) (*core.TeamProject, error) {
			require.NotNil(t, args.ProjectId)
			require.Equal(t, project, *args.ProjectId)
			return &core.TeamProject{
				Id: &projectID,
			}, nil
		},
	)

	mClientFactory.EXPECT().Graph(ctx, org).Return(mGraph, nil)
	mGraph.EXPECT().GetDescriptor(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args graph.GetDescriptorArgs) (*graph.GraphDescriptorResult, error) {
			require.NotNil(t, args.StorageKey)
			require.Equal(t, projectID, *args.StorageKey)
			return &graph.GraphDescriptorResult{
				Value: types.ToPtr(projectDescriptor),
			}, nil
		},
	)

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().RemoveAccessControlEntries(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.RemoveAccessControlEntriesArgs) (*bool, error) {
			require.NotNil(t, args.SecurityNamespaceId)
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			require.NotNil(t, args.Descriptors)
			require.Equal(t, descriptor, *args.Descriptors)
			return types.ToPtr(true), nil
		},
	)

	mSecurity.EXPECT().QueryAccessControlLists(ctx, gomock.Any()).Return(&[]security.AccessControlList{}, nil)

	opts := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		yes:         true,
	}

	err := runCommand(mCmdCtx, opts)
	require.NoError(t, err)
	require.Equal(t, "Permissions deleted.\n", out.String())
}
