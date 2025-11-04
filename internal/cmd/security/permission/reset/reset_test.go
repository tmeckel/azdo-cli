package reset

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
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestReset_SuccessRendersTable(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, _, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensions := mocks.NewMockAzDOExtension(ctrl)
	mSecurity := mocks.NewMockSecurityClient(ctrl)
	mPrinter := mocks.NewMockPrinter(ctrl)

	ctx := context.Background()
	org := "org"
	subject := "user@example.com"
	target := org + "/" + subject
	namespaceID := "11111111-1111-1111-1111-111111111111"
	namespaceUUID := uuid.MustParse(namespaceID)
	token := "repoV2/project/repo"
	descriptor := "vssgp.Subject"
	actionBit := 0x4

	mCmdCtx.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
	mCmdCtx.EXPECT().Printer("list").Return(mPrinter, nil)

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().QuerySecurityNamespaces(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QuerySecurityNamespacesArgs) (*[]security.SecurityNamespaceDescription, error) {
			require.NotNil(t, args.SecurityNamespaceId)
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)

			actions := []security.ActionDefinition{
				{
					Bit:         types.ToPtr(actionBit),
					Name:        types.ToPtr("Contribute"),
					DisplayName: types.ToPtr("Contribute"),
				},
			}
			descriptions := []security.SecurityNamespaceDescription{
				{
					Actions: &actions,
				},
			}
			return &descriptions, nil
		},
	)

	mSecurity.EXPECT().RemovePermission(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.RemovePermissionArgs) (*security.AccessControlEntry, error) {
			require.NotNil(t, args.SecurityNamespaceId)
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			require.NotNil(t, args.Permissions)
			require.Equal(t, actionBit, *args.Permissions)
			require.NotNil(t, args.Descriptor)
			require.Equal(t, descriptor, *args.Descriptor)
			require.NotNil(t, args.Token)
			require.Equal(t, token, *args.Token)
			return &security.AccessControlEntry{}, nil
		},
	)

	mSecurity.EXPECT().QueryAccessControlLists(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QueryAccessControlListsArgs) (*[]security.AccessControlList, error) {
			require.NotNil(t, args.SecurityNamespaceId)
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			require.NotNil(t, args.Token)
			require.Equal(t, token, *args.Token)
			require.NotNil(t, args.Descriptors)
			require.Equal(t, descriptor, *args.Descriptors)

			allow := 0
			deny := 0
			effectiveAllow := actionBit
			ace := security.AccessControlEntry{
				Descriptor: types.ToPtr(descriptor),
				Allow:      &allow,
				Deny:       &deny,
				ExtendedInfo: &security.AceExtendedInformation{
					EffectiveAllow: &effectiveAllow,
					EffectiveDeny:  types.ToPtr(0),
				},
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

	gomock.InOrder(
		mPrinter.EXPECT().AddColumns("Action", "Bit", "Effective"),
		mPrinter.EXPECT().EndRow(),
		mPrinter.EXPECT().AddField("Contribute"),
		mPrinter.EXPECT().AddField("4 (0x4)"),
		mPrinter.EXPECT().AddField("Allow (inherited)"),
		mPrinter.EXPECT().EndRow(),
		mPrinter.EXPECT().Render().Return(nil),
	)

	options := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		permission:  []string{"Contribute"},
		yes:         true,
	}

	err := runCommand(mCmdCtx, options)
	require.NoError(t, err)
}

func TestReset_PromptDeclinedReturnsCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, out, _ := iostreams.Test()

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
	namespaceUUID := uuid.MustParse(namespaceID)
	token := "repoV2"
	descriptor := "vssgp.Subject"

	mCmdCtx.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().QuerySecurityNamespaces(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QuerySecurityNamespacesArgs) (*[]security.SecurityNamespaceDescription, error) {
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			actions := []security.ActionDefinition{
				{Bit: types.ToPtr(4), Name: types.ToPtr("Contribute")},
			}
			descriptions := []security.SecurityNamespaceDescription{{Actions: &actions}}
			return &descriptions, nil
		},
	)

	mCmdCtx.EXPECT().Prompter().Return(mPrompter, nil)
	mPrompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)

	mSecurity.EXPECT().RemovePermission(gomock.Any(), gomock.Any()).Times(0)
	mSecurity.EXPECT().QueryAccessControlLists(gomock.Any(), gomock.Any()).Times(0)

	options := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		permission:  []string{"Contribute"},
	}

	err := runCommand(mCmdCtx, options)
	require.ErrorIs(t, err, util.ErrCancel)
	require.Empty(t, out.String())
}

func TestReset_InvalidNamespaceID(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, _, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()

	options := &opts{
		rawTarget:   "org/user@example.com",
		namespaceID: "not-a-guid",
		token:       "repo",
		permission:  []string{"Read"},
		yes:         true,
	}

	err := runCommand(mCmdCtx, options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid namespace id")
}

func TestReset_ParsePermissionBitsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, _, _ := iostreams.Test()

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

	mCmdCtx.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().QuerySecurityNamespaces(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QuerySecurityNamespacesArgs) (*[]security.SecurityNamespaceDescription, error) {
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			actions := []security.ActionDefinition{
				{
					Bit:         types.ToPtr(4),
					DisplayName: types.ToPtr("Contribute"),
				},
			}
			descriptions := []security.SecurityNamespaceDescription{{Actions: &actions}}
			return &descriptions, nil
		},
	)

	options := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		permission:  []string{"UnknownPermission"},
		yes:         true,
	}

	err := runCommand(mCmdCtx, options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unrecognized permission token")
}

func TestReset_RemovePermissionFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, _, _ := iostreams.Test()

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

	mCmdCtx.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().QuerySecurityNamespaces(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QuerySecurityNamespacesArgs) (*[]security.SecurityNamespaceDescription, error) {
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			actions := []security.ActionDefinition{
				{Bit: types.ToPtr(4), Name: types.ToPtr("Contribute")},
			}
			descriptions := []security.SecurityNamespaceDescription{{Actions: &actions}}
			return &descriptions, nil
		},
	)

	mSecurity.EXPECT().RemovePermission(ctx, gomock.Any()).Return(nil, fmt.Errorf("boom"))

	options := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		permission:  []string{"Contribute"},
		yes:         true,
	}

	err := runCommand(mCmdCtx, options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to reset permissions")
}

func TestReset_QueryACLFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, _, _ := iostreams.Test()

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

	mCmdCtx.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().QuerySecurityNamespaces(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QuerySecurityNamespacesArgs) (*[]security.SecurityNamespaceDescription, error) {
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			actions := []security.ActionDefinition{
				{Bit: types.ToPtr(4), Name: types.ToPtr("Contribute")},
			}
			descriptions := []security.SecurityNamespaceDescription{{Actions: &actions}}
			return &descriptions, nil
		},
	)

	mSecurity.EXPECT().RemovePermission(ctx, gomock.Any()).Return(&security.AccessControlEntry{}, nil)
	mSecurity.EXPECT().QueryAccessControlLists(ctx, gomock.Any()).Return(nil, fmt.Errorf("boom"))

	options := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		permission:  []string{"Contribute"},
		yes:         true,
	}

	err := runCommand(mCmdCtx, options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to query updated permissions")
}

func TestReset_NoPermissionsChangedMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, out, _ := iostreams.Test()

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

	mCmdCtx.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)
	mClientFactory.EXPECT().Security(ctx, org).Return(mSecurity, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: types.ToPtr(descriptor),
	}, nil)

	mSecurity.EXPECT().QuerySecurityNamespaces(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QuerySecurityNamespacesArgs) (*[]security.SecurityNamespaceDescription, error) {
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			actions := []security.ActionDefinition{
				{Bit: types.ToPtr(4), Name: types.ToPtr("Contribute")},
			}
			descriptions := []security.SecurityNamespaceDescription{{Actions: &actions}}
			return &descriptions, nil
		},
	)

	mSecurity.EXPECT().RemovePermission(ctx, gomock.Any()).Return(&security.AccessControlEntry{}, nil)
	mSecurity.EXPECT().QueryAccessControlLists(ctx, gomock.Any()).Return(&[]security.AccessControlList{}, nil)

	options := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		permission:  []string{"Contribute"},
		yes:         true,
	}

	err := runCommand(mCmdCtx, options)
	require.NoError(t, err)
	require.Equal(t, "No permissions changed.\n", out.String())
}

func TestReset_ProjectScopeResolvesDescriptor(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, _, _ := iostreams.Test()

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

	mCmdCtx.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
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

	mSecurity.EXPECT().QuerySecurityNamespaces(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, args security.QuerySecurityNamespacesArgs) (*[]security.SecurityNamespaceDescription, error) {
			require.Equal(t, namespaceUUID, *args.SecurityNamespaceId)
			actions := []security.ActionDefinition{
				{Bit: types.ToPtr(4), Name: types.ToPtr("Contribute")},
			}
			descriptions := []security.SecurityNamespaceDescription{{Actions: &actions}}
			return &descriptions, nil
		},
	)

	mSecurity.EXPECT().RemovePermission(ctx, gomock.Any()).Return(&security.AccessControlEntry{}, nil)
	mSecurity.EXPECT().QueryAccessControlLists(ctx, gomock.Any()).Return(&[]security.AccessControlList{}, nil)

	options := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		permission:  []string{"Contribute"},
		yes:         true,
	}

	err := runCommand(mCmdCtx, options)
	require.NoError(t, err)
}

func TestReset_IdentityDescriptorMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ioStreams, _, _, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensions := mocks.NewMockAzDOExtension(ctrl)

	ctx := context.Background()
	org := "org"
	subject := "user@example.com"
	target := org + "/" + subject
	namespaceID := "11111111-1111-1111-1111-111111111111"
	token := "repo"

	mCmdCtx.EXPECT().IOStreams().Return(ioStreams, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(ctx).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(ctx, org).Return(mExtensions, nil)

	mExtensions.EXPECT().ResolveIdentity(ctx, subject).Return(&identity.Identity{
		Descriptor: nil,
	}, nil)

	options := &opts{
		rawTarget:   target,
		namespaceID: namespaceID,
		token:       token,
		permission:  []string{"Contribute"},
		yes:         true,
	}

	err := runCommand(mCmdCtx, options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not contain a descriptor")
}
