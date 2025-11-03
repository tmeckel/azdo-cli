package update

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestUpdate_SetsACE_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	// Mocks
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensionsClient := mocks.NewMockAzDOExtension(ctrl)
	mIdentityClient := mocks.NewMockIdentityClient(ctrl)
	mSecurityClient := mocks.NewMockSecurityClient(ctrl)

	// Baseline expectations (AnyTimes)
	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	// Client factory returns clients for the requested organization
	mClientFactory.EXPECT().Extensions(gomock.Any(), gomock.Any()).Return(mExtensionsClient, nil).AnyTimes()
	mClientFactory.EXPECT().Identity(gomock.Any(), gomock.Any()).Return(mIdentityClient, nil).AnyTimes()
	mClientFactory.EXPECT().Security(gomock.Any(), gomock.Any()).Return(mSecurityClient, nil).AnyTimes()

	// Simulate ResolveSubject -> returns a member with Descriptor set
	ident := identity.Identity{
		Descriptor: types.ToPtr("storage-descriptor"),
	}
	mExtensionsClient.EXPECT().ResolveIdentity(gomock.Any(), gomock.Any()).Return(&ident, nil)

	// security.QuerySecurityNamespaces returns a non-nil slice (actions may be empty)
	ns := security.SecurityNamespaceDescription{
		Name: types.ToPtr("testns"),
	}
	nsSlice := &[]security.SecurityNamespaceDescription{ns}
	mSecurityClient.EXPECT().QuerySecurityNamespaces(gomock.Any(), gomock.Any()).Return(nsSlice, nil)

	// Build options matching command invocation
	o := &opts{
		rawTarget:   "org/user@example.com",
		namespaceID: "00000000-0000-0000-0000-000000000000",
		token:       "token123",
		allowBits:   []string{"0x1"},
		denyBits:    []string{},
		yes:         true,
	}

	// Expect SetAccessControlEntries to be called with a container containing token, merge flag and ACE with descriptor "acl-descriptor"
	mSecurityClient.EXPECT().SetAccessControlEntries(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, args security.SetAccessControlEntriesArgs) (*[]interface{}, error) {
			// Basic runtime checks to ensure arguments look correct
			if args.SecurityNamespaceId == nil {
				return nil, fmt.Errorf("SecurityNamespaceId is nil")
			}
			if !strings.EqualFold(args.SecurityNamespaceId.String(), o.namespaceID) {
				return nil, fmt.Errorf("SecurityNamespaceId mismatch, expected %q got %q", args.SecurityNamespaceId.String(), o.namespaceID)
			}

			if args.Container == nil {
				return nil, fmt.Errorf("container is nil")
			}
			// Verify token present - Container is typed as interface{} in the SDK, so assert via type switch
			switch c := args.Container.(type) {
			case AccessControlEntryUpdate:
				if c.Token != o.token {
					return nil, fmt.Errorf("token does not equal %q", o.token)
				}

				if len(c.AccessControlEntries) == 0 {
					return nil, fmt.Errorf("accessControlEntries empty")
				}
				if c.AccessControlEntries[0].Descriptor == nil || types.GetValue(c.AccessControlEntries[0].Descriptor, "") != *ident.Descriptor {
					return nil, fmt.Errorf("ace descriptor mismatch, expected %q got %q", *ident.Descriptor, types.GetValue(c.AccessControlEntries[0].Descriptor, ""))
				}
			default:
				return nil, fmt.Errorf("container has unexpected type")
			}
			return &[]any{}, nil
		},
	)

	// Run command
	err := runCommand(mCmdCtx, o)
	if err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	// Assert output contains success message
	require.NotEmpty(t, out.String(), "expected output to contain success message")
}

func TestUpdate_ErrorWhenNoBits(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensionsClient := mocks.NewMockAzDOExtension(ctrl)
	mIdentityClient := mocks.NewMockIdentityClient(ctrl)
	mSecurityClient := mocks.NewMockSecurityClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(gomock.Any(), gomock.Any()).Return(mExtensionsClient, nil).AnyTimes()
	mClientFactory.EXPECT().Identity(gomock.Any(), gomock.Any()).Return(mIdentityClient, nil).AnyTimes()
	mClientFactory.EXPECT().Security(gomock.Any(), gomock.Any()).Return(mSecurityClient, nil).AnyTimes()

	ident := identity.Identity{
		Descriptor: types.ToPtr("storage-descriptor"),
	}
	mExtensionsClient.EXPECT().ResolveIdentity(gomock.Any(), gomock.Any()).Return(&ident, nil).AnyTimes()

	ns := security.SecurityNamespaceDescription{
		Name: types.ToPtr("testns"),
	}
	nsSlice := &[]security.SecurityNamespaceDescription{ns}
	mSecurityClient.EXPECT().QuerySecurityNamespaces(gomock.Any(), gomock.Any()).Return(nsSlice, nil).AnyTimes()

	o := &opts{
		rawTarget:   "org/user@example.com",
		namespaceID: "00000000-0000-0000-0000-000000000000",
		token:       "token123",
		allowBits:   []string{},
		denyBits:    []string{},
		yes:         true,
	}

	err := runCommand(mCmdCtx, o)
	require.Error(t, err)
}

func TestUpdate_SetsDenyBit(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	// Mocks
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensionsClient := mocks.NewMockAzDOExtension(ctrl)
	mIdentityClient := mocks.NewMockIdentityClient(ctrl)
	mSecurityClient := mocks.NewMockSecurityClient(ctrl)

	// Baseline expectations (AnyTimes)
	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()

	mClientFactory.EXPECT().Extensions(gomock.Any(), gomock.Any()).Return(mExtensionsClient, nil).AnyTimes()
	mClientFactory.EXPECT().Identity(gomock.Any(), gomock.Any()).Return(mIdentityClient, nil).AnyTimes()
	mClientFactory.EXPECT().Security(gomock.Any(), gomock.Any()).Return(mSecurityClient, nil).AnyTimes()

	ident := identity.Identity{
		Descriptor: types.ToPtr("storage-descriptor"),
	}
	mExtensionsClient.EXPECT().ResolveIdentity(gomock.Any(), gomock.Any()).Return(&ident, nil)

	ns := security.SecurityNamespaceDescription{
		Name: types.ToPtr("testns"),
	}
	nsSlice := &[]security.SecurityNamespaceDescription{ns}
	mSecurityClient.EXPECT().QuerySecurityNamespaces(gomock.Any(), gomock.Any()).Return(nsSlice, nil)

	// Expect SetAccessControlEntries and assert deny bit presence
	mSecurityClient.EXPECT().SetAccessControlEntries(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, args security.SetAccessControlEntriesArgs) (*[]interface{}, error) {
			if args.Container == nil {
				return nil, fmt.Errorf("container is nil")
			}
			switch c := args.Container.(type) {
			case map[string]interface{}:
				// check deny exists on ACE
				if acesI, ok := c["accessControlEntries"]; ok {
					switch a := acesI.(type) {
					case []security.AccessControlEntry:
						if a[0].Deny == nil || types.GetValue(a[0].Deny, 0) == 0 {
							return nil, fmt.Errorf("deny bit not set")
						}
					}
				}
			}
			return &[]any{}, nil
		},
	)

	o := &opts{
		rawTarget:   "org/user@example.com",
		namespaceID: "00000000-0000-0000-0000-000000000000",
		token:       "token123",
		allowBits:   []string{},
		denyBits:    []string{"0x2"},
		yes:         true,
	}

	err := runCommand(mCmdCtx, o)
	require.NoError(t, err)
	require.NotEmpty(t, out.String())
}

func TestUpdate_NoMergeFlag(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	// Mocks
	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensionsClient := mocks.NewMockAzDOExtension(ctrl)
	mIdentityClient := mocks.NewMockIdentityClient(ctrl)
	mSecurityClient := mocks.NewMockSecurityClient(ctrl)

	// Baseline
	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
	mClientFactory.EXPECT().Extensions(gomock.Any(), gomock.Any()).Return(mExtensionsClient, nil).AnyTimes()
	mClientFactory.EXPECT().Identity(gomock.Any(), gomock.Any()).Return(mIdentityClient, nil).AnyTimes()
	mClientFactory.EXPECT().Security(gomock.Any(), gomock.Any()).Return(mSecurityClient, nil).AnyTimes()

	ident := identity.Identity{
		Descriptor: types.ToPtr("storage-descriptor"),
	}
	mExtensionsClient.EXPECT().ResolveIdentity(gomock.Any(), gomock.Any()).Return(&ident, nil)

	ns := security.SecurityNamespaceDescription{
		Name: types.ToPtr("testns"),
	}
	nsSlice := &[]security.SecurityNamespaceDescription{ns}
	mSecurityClient.EXPECT().QuerySecurityNamespaces(gomock.Any(), gomock.Any()).Return(nsSlice, nil)

	// Expect SetAccessControlEntries and assert merge flag false
	mSecurityClient.EXPECT().SetAccessControlEntries(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, args security.SetAccessControlEntriesArgs) (*[]interface{}, error) {
			if args.Container == nil {
				return nil, fmt.Errorf("container is nil")
			}
			switch c := args.Container.(type) {
			case map[string]interface{}:
				if mv, ok := c["merge"].(bool); !ok {
					return nil, fmt.Errorf("merge flag missing or not bool")
				} else if mv {
					return nil, fmt.Errorf("merge flag expected false")
				}
			}
			return &[]any{}, nil
		},
	)

	o := &opts{
		rawTarget:   "org/user@example.com",
		namespaceID: "00000000-0000-0000-0000-000000000000",
		token:       "token123",
		allowBits:   []string{"0x1"},
		denyBits:    []string{},
		yes:         true,
		merge:       false,
	}

	err := runCommand(mCmdCtx, o)
	require.NoError(t, err)
	require.NotEmpty(t, out.String())
}

func TestUpdate_MergeFlagTrue(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	mCmdCtx := mocks.NewMockCmdContext(ctrl)
	mClientFactory := mocks.NewMockClientFactory(ctrl)
	mExtensionsClient := mocks.NewMockAzDOExtension(ctrl)
	mIdentityClient := mocks.NewMockIdentityClient(ctrl)
	mSecurityClient := mocks.NewMockSecurityClient(ctrl)

	mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
	mClientFactory.EXPECT().Extensions(gomock.Any(), gomock.Any()).Return(mExtensionsClient, nil).AnyTimes()
	mClientFactory.EXPECT().Identity(gomock.Any(), gomock.Any()).Return(mIdentityClient, nil).AnyTimes()
	mClientFactory.EXPECT().Security(gomock.Any(), gomock.Any()).Return(mSecurityClient, nil).AnyTimes()

	ident := identity.Identity{
		Descriptor: types.ToPtr("storage-descriptor"),
	}
	mExtensionsClient.EXPECT().ResolveIdentity(gomock.Any(), gomock.Any()).Return(&ident, nil)

	ns := security.SecurityNamespaceDescription{
		Name: types.ToPtr("testns"),
	}
	nsSlice := &[]security.SecurityNamespaceDescription{ns}
	mSecurityClient.EXPECT().QuerySecurityNamespaces(gomock.Any(), gomock.Any()).Return(nsSlice, nil)

	mSecurityClient.EXPECT().SetAccessControlEntries(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, args security.SetAccessControlEntriesArgs) (*[]any, error) {
			switch c := args.Container.(type) {
			case AccessControlEntryUpdate:
				if !c.Merge {
					return nil, fmt.Errorf("merge flag expected true: %v", c.Merge)
				}
				if len(c.AccessControlEntries) == 0 {
					return nil, fmt.Errorf("aces missing or empty")
				}
				if allow := types.GetValue(c.AccessControlEntries[0].Allow, 0); allow != 1 {
					return nil, fmt.Errorf("allow bit mismatch got %d", allow)
				}
				if deny := types.GetValue(c.AccessControlEntries[0].Deny, 0); deny != 2 {
					return nil, fmt.Errorf("deny bit mismatch got %d", deny)
				}
			default:
				return nil, fmt.Errorf("unexpected container type %T", args.Container)
			}
			return &[]any{}, nil
		},
	)

	o := &opts{
		rawTarget:   "org/user@example.com",
		namespaceID: "00000000-0000-0000-0000-000000000000",
		token:       "token123",
		allowBits:   []string{"0x1"},
		denyBits:    []string{"0x2"},
		merge:       true,
		yes:         true,
	}

	err := runCommand(mCmdCtx, o)
	require.NoError(t, err)
	require.NotEmpty(t, out.String())
}

func TestParseBits(t *testing.T) {
	actions := []security.ActionDefinition{
		{
			Bit:         types.ToPtr(1),
			Name:        types.ToPtr("Read"),
			DisplayName: types.ToPtr("Read"),
		},
		{
			Bit:         types.ToPtr(2),
			Name:        types.ToPtr("Edit"),
			DisplayName: types.ToPtr("Modify"),
		},
		{
			Bit:         types.ToPtr(4),
			Name:        types.ToPtr("Contribute"),
			DisplayName: types.ToPtr("Contribute"),
		},
	}

	tests := []struct {
		name    string
		input   []string
		want    int
		wantErr bool
	}{
		{
			name:  "hexadecimal value",
			input: []string{"0x4"},
			want:  4,
		},
		{
			name:  "decimal value",
			input: []string{"2"},
			want:  2,
		},
		{
			name:  "textual name",
			input: []string{"Read"},
			want:  1,
		},
		{
			name:  "display name",
			input: []string{"Modify"},
			want:  2,
		},
		{
			name:  "combined values",
			input: []string{"Read", "0x2"},
			want:  3,
		},
		{
			name:    "invalid token",
			input:   []string{"UnknownPermission"},
			wantErr: true,
		},
		{
			name:    "unknown expression with hex",
			input:   []string{"Unknown (0x4)"},
			wantErr: true,
		},
		{
			name:    "unknown bit",
			input:   []string{"0x8"},
			wantErr: true,
		},
		{
			name:  "empty",
			input: []string{},
			want:  0,
		},
		{
			name:    "combined invalid values",
			input:   []string{"Read", "0x2", "0x8"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBits(actions, tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
