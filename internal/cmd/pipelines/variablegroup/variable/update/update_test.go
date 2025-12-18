package update_test

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	updatecmd "github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/variable/update"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/mock/gomock"
)

func TestUpdateVariable_ValueAndFlags_JSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, outBuf, errBuf := iostreams.Test()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()

	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.GetVariableGroupsByIdArgs) (*[]taskagent.VariableGroup, error) {
			require.Equal(t, "project", types.GetValue(args.Project, ""))
			require.Len(t, types.GetValue(args.GroupIds, []int{}), 1)
			assert.Equal(t, 123, types.GetValue(&types.GetValue(args.GroupIds, []int{})[0], 0))
			vars := map[string]interface{}{
				"FOO": map[string]interface{}{
					"value":      "old",
					"isSecret":   false,
					"isReadOnly": false,
				},
			}
			return &[]taskagent.VariableGroup{{
				Id:        types.ToPtr(123),
				Name:      types.ToPtr("group"),
				Variables: &vars,
			}}, nil
		},
	)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.UpdateVariableGroupArgs) (*taskagent.VariableGroup, error) {
			require.NotNil(t, args.VariableGroupParameters)
			require.NotNil(t, args.VariableGroupParameters.Variables)
			updated := types.GetValue(args.VariableGroupParameters.Variables, map[string]interface{}{})
			val, ok := updated["FOO"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "new", val["value"])
			assert.Equal(t, false, val["isSecret"])
			assert.Equal(t, true, val["isReadOnly"])
			return &taskagent.VariableGroup{}, nil
		},
	)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--value", "new", "--read-only", "--json"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"FOO","secret":false,"value":"new","readOnly":true}`, outBuf.String())
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_SecretPrompt_RedactsJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, inBuf, outBuf, errBuf := iostreams.Test()
	io.SetStdinTTY(true)
	io.SetStdoutTTY(true)
	io.SetStderrTTY(true)
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)
	prompter := mocks.NewMockPrompter(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	cmdCtx.EXPECT().Prompter().Return(prompter, nil).AnyTimes()

	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:   types.ToPtr(123),
			Name: types.ToPtr("group"),
			Variables: &map[string]interface{}{
				"SECRET": map[string]interface{}{
					"value":      nil,
					"isSecret":   true,
					"isReadOnly": false,
				},
			},
		}}, nil,
	)
	prompter.EXPECT().Secret("Value for secret \"secret\":").Return("s3cr3t", nil)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.UpdateVariableGroupArgs) (*taskagent.VariableGroup, error) {
			updated := types.GetValue(args.VariableGroupParameters.Variables, map[string]interface{}{})
			val, _ := updated["SECRET"].(map[string]interface{})
			assert.Equal(t, "s3cr3t", val["value"])
			assert.Equal(t, true, val["isSecret"])
			return &taskagent.VariableGroup{}, nil
		},
	)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "secret", "--prompt-value", "--json"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"SECRET","secret":true,"readOnly":false}`, outBuf.String())
	assert.Equal(t, "", errBuf.String())
	assert.Equal(t, "", inBuf.String())
}

func TestUpdateVariable_ClearValue_YesSkipsPrompt(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)
	printer := mocks.NewMockPrinter(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	cmdCtx.EXPECT().Printer("list").Return(printer, nil)
	printer.EXPECT().AddColumns("NAME", "VALUE", "IS SECRET", "IS READONLY")
	printer.EXPECT().EndRow()
	printer.EXPECT().AddField("FOO")
	printer.EXPECT().AddField("")
	printer.EXPECT().AddField("false")
	printer.EXPECT().AddField("false")
	printer.EXPECT().EndRow()
	printer.EXPECT().Render().Return(nil)

	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:   types.ToPtr(123),
			Name: types.ToPtr("group"),
			Variables: &map[string]interface{}{
				"FOO": map[string]interface{}{
					"value":      "keepme",
					"isSecret":   false,
					"isReadOnly": false,
				},
			},
		}}, nil,
	)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.UpdateVariableGroupArgs) (*taskagent.VariableGroup, error) {
			val, _ := types.GetValue(args.VariableGroupParameters.Variables, map[string]interface{}{})["FOO"].(map[string]interface{})
			assert.Nil(t, val["value"])
			return &taskagent.VariableGroup{}, nil
		},
	)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--clear-value", "--yes"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_FromJSON_MutualExclusivity(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{Id: types.ToPtr(123), Name: types.ToPtr("group")}}, nil,
	)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--from-json", `{"value":"v"}`, "--value", "x"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--from-json cannot be combined")
}

func TestUpdateVariable_NoChangesProvided(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, _ := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{Id: types.ToPtr(123), Name: types.ToPtr("group")}}, nil,
	)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no changes requested")
}

func TestUpdateVariable_RenameCollision(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test()

	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()

	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:   types.ToPtr(123),
			Name: types.ToPtr("group"),
			Variables: &map[string]interface{}{
				"FOO": map[string]interface{}{"value": "v"},
				"BAR": map[string]interface{}{"value": "x"},
			},
		}}, nil,
	)
	// UpdateVariableGroup must not be called on collision
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--new-name", "bar"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_ClearSecretValueFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()

	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:   types.ToPtr(123),
			Name: types.ToPtr("group"),
			Variables: &map[string]interface{}{
				"SECRET": map[string]interface{}{
					"value":      nil,
					"isSecret":   true,
					"isReadOnly": false,
				},
			},
		}}, nil,
	)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "secret", "--clear-value"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot clear value of a secret variable")
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_KeyVaultRejectsValueChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:        types.ToPtr(123),
			Name:      types.ToPtr("group"),
			Type:      types.ToPtr("AzureKeyVault"),
			Variables: &map[string]interface{}{"FOO": map[string]interface{}{"value": "old", "isSecret": false}},
		}}, nil,
	)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--value", "new"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot modify variable values for an Azure Key Vault-backed variable group")
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_FromJSON_SecretTrueWithoutValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:   types.ToPtr(123),
			Name: types.ToPtr("group"),
			Variables: &map[string]interface{}{
				"FOO": map[string]interface{}{"value": "old", "isSecret": false},
			},
		}}, nil,
	)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--from-json", `{"secret":true}`})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret=true via --from-json requires a 'value'")
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_PromptValue_DisabledPrompt(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test() // CanPrompt false by default
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:   types.ToPtr(123),
			Name: types.ToPtr("group"),
			Variables: &map[string]interface{}{
				"SECRET": map[string]interface{}{"value": nil, "isSecret": true},
			},
		}}, nil,
	)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "secret", "--prompt-value"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompting for secret value is not supported")
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_FromJSON_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil).AnyTimes()
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(_ context.Context, args taskagent.GetVariableGroupsByIdArgs) (*[]taskagent.VariableGroup, error) {
			if args.GroupIds != nil && len(*args.GroupIds) > 0 && (*args.GroupIds)[0] == 123 {
				return &[]taskagent.VariableGroup{{Id: types.ToPtr(123), Name: types.ToPtr("group")}}, nil
			}
			return nil, errors.New("unexpected call")
		},
	)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--from-json", `not-json`})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_SecretValueFlag_RedactsJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, outBuf, errBuf := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:   types.ToPtr(123),
			Name: types.ToPtr("group"),
			Variables: &map[string]interface{}{
				"SECRET": map[string]interface{}{"value": nil, "isSecret": true, "isReadOnly": false},
			},
		}}, nil,
	)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args taskagent.UpdateVariableGroupArgs) (*taskagent.VariableGroup, error) {
			valMap := types.GetValue(args.VariableGroupParameters.Variables, map[string]interface{}{})
			val, _ := valMap["SECRET"].(map[string]interface{})
			assert.Equal(t, "rotated", val["value"])
			assert.Equal(t, true, val["isSecret"])
			assert.Equal(t, false, val["isReadOnly"])
			return &taskagent.VariableGroup{}, nil
		},
	)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "secret", "--value", "rotated", "--secret", "--json"})

	_, err := cmd.ExecuteC()
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"SECRET","secret":true,"readOnly":false}`, outBuf.String())
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_ClearValue_PromptCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)
	prompter := mocks.NewMockPrompter(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	cmdCtx.EXPECT().Prompter().Return(prompter, nil)

	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:   types.ToPtr(123),
			Name: types.ToPtr("group"),
			Variables: &map[string]interface{}{
				"FOO": map[string]interface{}{"value": "keep", "isSecret": false},
			},
		}}, nil,
	)
	prompter.EXPECT().Confirm(gomock.Any(), false).Return(false, nil)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--clear-value"})

	_, err := cmd.ExecuteC()
	require.ErrorIs(t, err, util.ErrCancel)
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_VariableNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()

	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:        types.ToPtr(123),
			Name:      types.ToPtr("group"),
			Variables: &map[string]interface{}{"BAR": map[string]interface{}{"value": "x"}},
		}}, nil,
	)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--value", "v"})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "variable \"foo\" not found")
	assert.Equal(t, "", errBuf.String())
}

func TestUpdateVariable_FromJSON_DisallowedNameKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, _, errBuf := iostreams.Test()
	cmdCtx := mocks.NewMockCmdContext(ctrl)
	clientFactory := mocks.NewMockClientFactory(ctrl)
	taskClient := mocks.NewMockTaskAgentClient(ctrl)

	cmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	cmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
	cmdCtx.EXPECT().ClientFactory().Return(clientFactory).AnyTimes()
	clientFactory.EXPECT().TaskAgent(gomock.Any(), "org").Return(taskClient, nil)
	taskClient.EXPECT().GetVariableGroupsById(gomock.Any(), gomock.Any()).Return(
		&[]taskagent.VariableGroup{{
			Id:        types.ToPtr(123),
			Name:      types.ToPtr("group"),
			Variables: &map[string]interface{}{"FOO": map[string]interface{}{"value": "x"}},
		}}, nil,
	)
	taskClient.EXPECT().UpdateVariableGroup(gomock.Any(), gomock.Any()).Times(0)

	cmd := updatecmd.NewCmd(cmdCtx)
	cmd.SetArgs([]string{"org/project/123", "--name", "foo", "--from-json", `{"name":"bar","value":"v"}`})

	_, err := cmd.ExecuteC()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payload must not include 'name'")
	assert.Equal(t, "", errBuf.String())
}
