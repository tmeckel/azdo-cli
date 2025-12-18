package delete

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	targetArg string
	name      string
	yes       bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "delete [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME --name VARIABLE_NAME",
		Short: "Delete a variable from a variable group",
		Long: heredoc.Doc(`
			Remove a variable from a variable group. The variable name lookup is case-insensitive.
		`),
		Example: heredoc.Doc(`
			# Delete variable 'PASSWORD' from variable group 123 in the default organization
			azdo pipelines variable-group variable delete MyProject/123 --name PASSWORD --yes
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.targetArg = args[0]
			return run(ctx, o)
		},
	}

	cmd.Flags().StringVar(&o.name, "name", "", "Name of the variable to delete (case-insensitive)")
	cmd.Flags().BoolVar(&o.yes, "yes", false, "Skip the confirmation prompt")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func run(cmdCtx util.CmdContext, opts *opts) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	taskClient, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create task agent client: %w", err)
	}

	group, err := shared.ResolveVariableGroup(cmdCtx, taskClient, scope.Project, scope.Target)
	if err != nil {
		return err
	}
	if group == nil {
		return fmt.Errorf("variable group %q not found", scope.Target)
	}
	if group.Id == nil {
		return fmt.Errorf("resolved variable group is missing an ID")
	}

	groupID := *group.Id
	groupName := types.GetValue(group.Name, scope.Target)

	// Find variable key case-insensitively
	var foundKey string
	var vars map[string]interface{}
	if group.Variables != nil {
		vars = *group.Variables
		for k := range vars {
			if strings.EqualFold(k, opts.name) {
				foundKey = k
				break
			}
		}
	}
	if foundKey == "" {
		return fmt.Errorf("variable %q not found in group %q", opts.name, groupName)
	}

	// Confirm unless --yes
	if !opts.yes {
		if !ios.CanPrompt() {
			return util.FlagErrorf("--yes required when not running interactively")
		}
		ios.StopProgressIndicator()
		prompter, err := cmdCtx.Prompter()
		if err != nil {
			return err
		}
		prompt := fmt.Sprintf("Delete variable '%s' from group '%s'?", opts.name, groupName)
		confirmed, err := prompter.Confirm(prompt, false)
		if err != nil {
			return err
		}
		if !confirmed {
			zap.L().Debug("variable deletion canceled by user", zap.String("group", groupName), zap.String("variable", opts.name))
			return util.ErrCancel
		}
		ios.StartProgressIndicator()
	}

	// Remove the variable and call UpdateVariableGroup
	newVars := make(map[string]interface{})
	for k, v := range vars {
		if k == foundKey {
			continue
		}
		newVars[k] = v
	}

	params := taskagent.VariableGroupParameters{
		Name:      group.Name,
		Variables: &newVars,
	}
	_, err = taskClient.UpdateVariableGroup(cmdCtx.Context(), taskagent.UpdateVariableGroupArgs{
		VariableGroupParameters: &params,
		GroupId:                 &groupID,
	})
	if err != nil {
		return fmt.Errorf("failed to update variable group %d: %w", groupID, err)
	}

	ios.StopProgressIndicator()
	// Only print on TTY
	if ios.IsStdoutTTY() {
		fmt.Fprintf(ios.Out, "Variable deleted.\n")
	}

	return nil
}
