package add

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
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
	cfg  *mocks.MockConfig
	auth *mocks.MockAuthConfig
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
		cfg:  mocks.NewMockConfig(ctrl),
		auth: mocks.NewMockAuthConfig(ctrl),
		ios:  io,
		out:  out,
	}
	d.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	d.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	d.cmd.EXPECT().ClientFactory().Return(d.fact).AnyTimes()
	return d
}

// setupDefaultOrg mocks the default organization lookup used by the
// ParseTargetWithDefaultOrganization wrapper.
func (d *deps) setupDefaultOrg(org string) {
	d.cmd.EXPECT().Config().Return(d.cfg, nil).AnyTimes()
	d.cfg.EXPECT().Authentication().Return(d.auth).AnyTimes()
	d.auth.EXPECT().GetDefaultOrganization().Return(org, nil).AnyTimes()
}

func (d *deps) expectProjectScope(t *testing.T, org, project string) *mocks.MockGraphClient {
	coreClient := mocks.NewMockCoreClient(d.ctrl)
	graphClient := mocks.NewMockGraphClient(d.ctrl)
	d.fact.EXPECT().Core(gomock.Any(), org).Return(coreClient, nil).AnyTimes()
	d.fact.EXPECT().Graph(gomock.Any(), org).Return(graphClient, nil).AnyTimes()
	coreClient.EXPECT().GetProject(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, args core.GetProjectArgs) (*core.TeamProject, error) {
			assert.Equal(t, project, *args.ProjectId)
			return &core.TeamProject{Id: types.ToPtr(uuid.New())}, nil
		}).AnyTimes()
	graphClient.EXPECT().GetDescriptor(gomock.Any(), gomock.Any()).
		Return(&graph.GraphDescriptorResult{Value: types.ToPtr("vssgp.project-scope")}, nil).AnyTimes()
	return graphClient
}

// expectHappyAdd wires the Extensions and Graph clients for a successful add
// and asserts the organization and group name the command resolves. When
// graphClient is provided it is reused instead of creating a new one.
func (d *deps) expectHappyAdd(org, groupName string, graphClient *mocks.MockGraphClient) {
	ext := mocks.NewMockAzDOExtension(d.ctrl)
	d.fact.EXPECT().Extensions(gomock.Any(), org).Return(ext, nil).AnyTimes()
	ext.EXPECT().FindGroupsByDisplayName(gomock.Any(), groupName, gomock.Any()).
		Return([]*graph.GraphGroup{{
			Descriptor:  types.ToPtr("vssgp.Uy0xLTItMw=="),
			DisplayName: types.ToPtr(groupName),
		}}, nil)
	ext.EXPECT().ResolveSubject(gomock.Any(), "user@example.com").
		Return(&graph.GraphSubject{
			Descriptor:  types.ToPtr("aad.user-id"),
			DisplayName: types.ToPtr("Alice"),
		}, nil)

	if graphClient == nil {
		graphClient = mocks.NewMockGraphClient(d.ctrl)
		d.fact.EXPECT().Graph(gomock.Any(), org).Return(graphClient, nil).AnyTimes()
	}
	graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).
		Return(&azuredevops.WrappedError{StatusCode: types.ToPtr(http.StatusNotFound)})
	graphClient.EXPECT().AddMembership(gomock.Any(), graph.AddMembershipArgs{
		ContainerDescriptor: types.ToPtr("vssgp.Uy0xLTItMw=="),
		SubjectDescriptor:   types.ToPtr("aad.user-id"),
	}).Return(&graph.GraphMembership{
		MemberDescriptor: types.ToPtr("aad.user-id"),
	}, nil)
}

func execute(t *testing.T, d *deps, args ...string) error {
	t.Helper()
	cmd := NewCmd(d.cmd)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunAdd_DefaultOrgOrgLevelGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.setupDefaultOrg("myorg")
	d.expectHappyAdd("myorg", "Project Administrators", nil)

	err := execute(t, d, "/Project Administrators", "--member", "user@example.com", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"memberDisplayName":"Alice"`)
	assert.Contains(t, d.out.String(), `"status":"added"`)
}

func TestRunAdd_DefaultOrgProjectFirstGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.setupDefaultOrg("myorg")
	graphClient := d.expectProjectScope(t, "myorg", "MyProject")
	d.expectHappyAdd("myorg", "Contributors", graphClient)

	err := execute(t, d, "MyProject/Contributors", "--member", "user@example.com", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"status":"added"`)
}

func TestRunAdd_ExplicitOrgOrgLevelGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.expectHappyAdd("otherorg", "Contributors", nil)

	err := execute(t, d, "otherorg:/Contributors", "--member", "user@example.com", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"status":"added"`)
}

func TestRunAdd_ExplicitOrgProjectGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	graphClient := d.expectProjectScope(t, "otherorg", "MyProject")
	d.expectHappyAdd("otherorg", "Contributors", graphClient)

	err := execute(t, d, "otherorg:MyProject/Contributors", "--member", "user@example.com", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"status":"added"`)
}

func TestRunAdd_LegacySlashError(t *testing.T) {
	t.Parallel()

	d := setup(t)
	err := execute(t, d, "otherorg/MyProject/Contributors", "--member", "user@example.com", "--json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")
}

func TestRunAdd_LegacySlashReinterpretedAsProjectFirst(t *testing.T) {
	t.Parallel()

	// A legacy "ORG/GROUP" input is reinterpreted as the canonical
	// project-first form: the leading segment becomes the project and the
	// default organization is used.
	d := setup(t)
	d.setupDefaultOrg("myorg")
	graphClient := d.expectProjectScope(t, "myorg", "otherorg")
	d.expectHappyAdd("myorg", "Contributors", graphClient)

	err := execute(t, d, "otherorg/Contributors", "--member", "user@example.com", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"status":"added"`)
}

func TestNewCmd_UseDocumentsCanonicalSyntax(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	require.True(t, strings.HasPrefix(cmd.Use, "add [ORG:][PROJECT/]GROUP"))
	assert.Contains(t, cmd.Example, "MyOrg:MyProject/Readers")
}
