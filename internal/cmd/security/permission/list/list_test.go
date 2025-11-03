package list

import (
	"context"
	"fmt"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestList_UsesSubjectDescriptorWhenIdentityMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensionsClient := mocks.NewMockAzDOExtension(ctrl)
	mIdentityClient := mocks.NewMockIdentityClient(ctrl)
	mSecurityClient := mocks.NewMockSecurityClient(ctrl)

	sid := "S-1-9-1551374245-1204400969-2402986413-2179408616-0-0-0-0-1"
	namespaceID := "00000000-0000-0000-0000-000000000000"

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(gomock.Any(), gomock.Any()).Return(mExtensionsClient, nil).AnyTimes()
	mClientFactory.EXPECT().Identity(gomock.Any(), gomock.Any()).Return(mIdentityClient, nil).AnyTimes()
	mClientFactory.EXPECT().Security(gomock.Any(), gomock.Any()).Return(mSecurityClient, nil).AnyTimes()

	// The implementation now calls ResolveIdentity; return an identity with Descriptor set.
	ident := identity.Identity{
		Descriptor: types.ToPtr(sid),
	}
	mExtensionsClient.EXPECT().ResolveIdentity(gomock.Any(), sid).Return(&ident, nil)

	// Identity lookup is skipped when the resolved subject does not contain
	// enough information to perform the lookup. The subject descriptor is
	// used as a fallback, so no call to ReadIdentities is expected.

	mSecurityClient.EXPECT().QueryAccessControlLists(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, args security.QueryAccessControlListsArgs) (*[]security.AccessControlList, error) {
			if args.Descriptors == nil {
				return nil, fmt.Errorf("descriptors unexpectedly nil")
			}
			if got := types.GetValue(args.Descriptors, ""); got != sid {
				return nil, fmt.Errorf("expected descriptor %q, got %q", sid, got)
			}
			result := []security.AccessControlList{}
			return &result, nil
		},
	)

	o := &opts{
		rawTarget:   fmt.Sprintf("org/%s", sid),
		namespaceID: namespaceID,
	}

	err := runCommand(mCmdCtx, o)
	require.NoError(t, err)
	require.Contains(t, out.String(), "No permissions found.")
}
