package remove

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
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

func (d *deps) expectExistingMembership(org, groupName string, graphClient *mocks.MockGraphClient, expectRemove bool) {
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
	graphClient.EXPECT().CheckMembershipExistence(gomock.Any(), gomock.Any()).Return(nil)
	if expectRemove {
		graphClient.EXPECT().RemoveMembership(gomock.Any(), graph.RemoveMembershipArgs{
			ContainerDescriptor: types.ToPtr("vssgp.Uy0xLTItMw=="),
			SubjectDescriptor:   types.ToPtr("aad.user-id"),
		}).Return(nil)
	}
}

func execute(t *testing.T, d *deps, args ...string) error {
	t.Helper()
	cmd := NewCmd(d.cmd)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestRunRemove_DefaultOrgOrgLevelGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.setupDefaultOrg("myorg")
	d.expectExistingMembership("myorg", "Project Administrators", nil, true)

	err := execute(t, d, "/Project Administrators", "--member", "user@example.com", "--yes", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"status":"removed"`)
}

func TestRunRemove_DefaultOrgProjectFirstGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.setupDefaultOrg("myorg")
	graphClient := d.expectProjectScope(t, "myorg", "MyProject")
	d.expectExistingMembership("myorg", "Contributors", graphClient, true)

	err := execute(t, d, "MyProject/Contributors", "--member", "user@example.com", "--yes", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"status":"removed"`)
}

func TestRunRemove_ExplicitOrgProjectGroup(t *testing.T) {
	t.Parallel()

	d := setup(t)
	graphClient := d.expectProjectScope(t, "otherorg", "MyProject")
	d.expectExistingMembership("otherorg", "Contributors", graphClient, true)

	err := execute(t, d, "otherorg:MyProject/Contributors", "--member", "user@example.com", "--yes", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"status":"removed"`)
}

func TestRunRemove_PromptConfirms(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.expectExistingMembership("otherorg", "Contributors", nil, true)

	prompter := mocks.NewMockPrompter(d.ctrl)
	d.cmd.EXPECT().Prompter().Return(prompter, nil).AnyTimes()
	prompter.EXPECT().Confirm(gomock.Any(), false).Return(true, nil)

	err := execute(t, d, "otherorg:/Contributors", "--member", "user@example.com", "--json")
	require.NoError(t, err)
	assert.Contains(t, d.out.String(), `"status":"removed"`)
}

func TestRunRemove_PromptDeclinedReturnsCancel(t *testing.T) {
	t.Parallel()

	d := setup(t)
	d.expectExistingMembership("otherorg", "Contributors", nil, false)

	prompter := mocks.NewMockPrompter(d.ctrl)
	d.cmd.EXPECT().Prompter().Return(prompter, nil).AnyTimes()
	prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)

	err := execute(t, d, "otherorg:/Contributors", "--member", "user@example.com", "--json")
	require.ErrorIs(t, err, util.ErrCancel)
}

func TestRunRemove_LegacySlashErrorAndReinterpretation(t *testing.T) {
	t.Parallel()

	d := setup(t)
	err := execute(t, d, "otherorg/MyProject/Contributors", "--member", "user@example.com", "--yes", "--json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")

	d2 := setup(t)
	d2.setupDefaultOrg("myorg")
	graphClient := d2.expectProjectScope(t, "myorg", "otherorg")
	d2.expectExistingMembership("myorg", "Contributors", graphClient, true)

	err = execute(t, d2, "otherorg/Contributors", "--member", "user@example.com", "--yes", "--json")
	require.NoError(t, err)
	assert.Contains(t, d2.out.String(), `"status":"removed"`)
}

func TestNewCmd_UseDocumentsCanonicalSyntax(t *testing.T) {
	t.Parallel()

	cmd := NewCmd(nil)
	require.True(t, strings.HasPrefix(cmd.Use, "remove [ORG:][PROJECT/]GROUP"))
}
