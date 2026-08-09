package list

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deps struct {
	ctrl *gomock.Controller
	cmd  *mocks.MockCmdContext
	fact *mocks.MockClientFactory
	ios  *iostreams.IOStreams
	out  *bytes.Buffer
}

func setup(t *testing.T) *deps {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()

	d := &deps{
		ctrl: ctrl,
		cmd:  mocks.NewMockCmdContext(ctrl),
		fact: mocks.NewMockClientFactory(ctrl),
		ios:  io,
		out:  out,
	}
	d.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	d.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	d.cmd.EXPECT().ClientFactory().Return(d.fact).AnyTimes()
	return d
}

func (d *deps) setupDefaultOrg(org string) {
	cfg := mocks.NewMockConfig(d.ctrl)
	auth := mocks.NewMockAuthConfig(d.ctrl)
	d.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

// expectIdentityLookup wires the Identity client and asserts the org and
// group name used for the member lookup.
func (d *deps) expectIdentityLookup(t *testing.T, org, groupName string, subjectDescriptor string) {
	identityClient := mocks.NewMockIdentityClient(d.ctrl)
	d.fact.EXPECT().Identity(gomock.Any(), org).Return(identityClient, nil).AnyTimes()
	identityClient.EXPECT().ReadIdentities(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args identity.ReadIdentitiesArgs) (*[]identity.Identity, error) {
			assert.Equal(t, "LocalGroupName", *args.SearchFilter)
			assert.Equal(t, groupName, *args.FilterValue)
			return &[]identity.Identity{{
				SubjectDescriptor: types.ToPtr(subjectDescriptor),
			}}, nil
		})
}

func (d *deps) expectMembershipTraversal(t *testing.T, org, subjectDescriptor string) {
	graphClient := mocks.NewMockGraphClient(d.ctrl)
	d.fact.EXPECT().Graph(gomock.Any(), org).Return(graphClient, nil).AnyTimes()
	graphClient.EXPECT().ListMemberships(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args graph.ListMembershipsArgs) (*[]graph.GraphMembership, error) {
			assert.Equal(t, subjectDescriptor, *args.SubjectDescriptor)
			return &[]graph.GraphMembership{{
				MemberDescriptor: types.ToPtr("aad.user-id"),
			}}, nil
		})
	graphClient.EXPECT().LookupSubjects(gomock.Any(), gomock.Any()).
		Return(&map[string]graph.GraphSubject{
			"aad.user-id": {
				Descriptor:  types.ToPtr("aad.user-id"),
				DisplayName: types.ToPtr("Alice"),
				SubjectKind: types.ToPtr("user"),
			},
		}, nil)
}

func execute(t *testing.T, d *deps, args ...string) error {
	t.Helper()
	cmd := NewCmd(d.cmd)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunList_DefaultOrgOrgLevelGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.setupDefaultOrg("myorg")
	d.expectIdentityLookup(t, "myorg", "Project Administrators", "vssgp.Uy0xLTItMw==")
	d.expectMembershipTraversal(t, "myorg", "vssgp.Uy0xLTItMw==")

	err := execute(t, d, "/Project Administrators", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"displayName":"Alice"`)
}

func TestRunList_DefaultOrgProjectFirstGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.setupDefaultOrg("myorg")
	coreClient := mocks.NewMockCoreClient(d.ctrl)
	graphClient := mocks.NewMockGraphClient(d.ctrl)
	d.fact.EXPECT().Core(gomock.Any(), "myorg").Return(coreClient, nil).AnyTimes()
	d.fact.EXPECT().Graph(gomock.Any(), "myorg").Return(graphClient, nil).AnyTimes()
	coreClient.EXPECT().GetProject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetProjectArgs) (*core.TeamProject, error) {
			assert.Equal(t, "MyProject", *args.ProjectId)
			return &core.TeamProject{Id: types.ToPtr(uuid.New())}, nil
		}).AnyTimes()
	graphClient.EXPECT().GetDescriptor(gomock.Any(), gomock.Any()).
		Return(&graph.GraphDescriptorResult{Value: types.ToPtr("vssgp.project-scope")}, nil).AnyTimes()

	d.expectIdentityLookup(t, "myorg", "Contributors", "vssgp.project-scope")

	// Membership traversal runs on the same Graph client the command
	// created after the descriptor resolution.
	graphClient.EXPECT().ListMemberships(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args graph.ListMembershipsArgs) (*[]graph.GraphMembership, error) {
			assert.Equal(t, "vssgp.project-scope", *args.SubjectDescriptor)
			return &[]graph.GraphMembership{{
				MemberDescriptor: types.ToPtr("aad.user-id"),
			}}, nil
		})
	graphClient.EXPECT().LookupSubjects(gomock.Any(), gomock.Any()).
		Return(&map[string]graph.GraphSubject{
			"aad.user-id": {
				Descriptor:  types.ToPtr("aad.user-id"),
				DisplayName: types.ToPtr("Alice"),
				SubjectKind: types.ToPtr("user"),
			},
		}, nil)

	err := execute(t, d, "MyProject/Contributors", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"displayName":"Alice"`)
}

func TestRunList_ExplicitOrgOrgLevelGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.expectIdentityLookup(t, "otherorg", "Contributors", "vssgp.Uy0xLTItMw==")
	d.expectMembershipTraversal(t, "otherorg", "vssgp.Uy0xLTItMw==")

	err := execute(t, d, "otherorg:/Contributors", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"displayName":"Alice"`)
}

func TestRunList_LegacySlashErrorAndReinterpretation(t *testing.T) {
	t.Parallel()

	d := setup(t)
	err := execute(t, d, "otherorg/MyProject/Contributors", "--json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")

	d2 := setup(t)
	d2.setupDefaultOrg("myorg")
	coreClient := mocks.NewMockCoreClient(d2.ctrl)
	graphClient := mocks.NewMockGraphClient(d2.ctrl)
	d2.fact.EXPECT().Core(gomock.Any(), "myorg").Return(coreClient, nil).AnyTimes()
	d2.fact.EXPECT().Graph(gomock.Any(), "myorg").Return(graphClient, nil).AnyTimes()
	coreClient.EXPECT().GetProject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetProjectArgs) (*core.TeamProject, error) {
			assert.Equal(t, "otherorg", *args.ProjectId)
			return &core.TeamProject{Id: types.ToPtr(uuid.New())}, nil
		}).AnyTimes()
	graphClient.EXPECT().GetDescriptor(gomock.Any(), gomock.Any()).
		Return(&graph.GraphDescriptorResult{Value: types.ToPtr("vssgp.project-scope")}, nil).AnyTimes()

	d2.expectIdentityLookup(t, "myorg", "Contributors", "vssgp.project-scope")
	graphClient.EXPECT().ListMemberships(gomock.Any(), gomock.Any()).
		Return(&[]graph.GraphMembership{{MemberDescriptor: types.ToPtr("aad.user-id")}}, nil)
	graphClient.EXPECT().LookupSubjects(gomock.Any(), gomock.Any()).
		Return(&map[string]graph.GraphSubject{
			"aad.user-id": {
				Descriptor:  types.ToPtr("aad.user-id"),
				DisplayName: types.ToPtr("Alice"),
				SubjectKind: types.ToPtr("user"),
			},
		}, nil)

	err = execute(t, d2, "otherorg/Contributors", "--json")
	require.NoError(t, err)
	assert.Contains(t, d2.out.String(), `"displayName":"Alice"`)
}

func TestNewCmd_UseDocumentsCanonicalSyntax(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	require.True(t, strings.HasPrefix(cmd.Use, "list [ORG:][PROJECT/]GROUP"))
}
