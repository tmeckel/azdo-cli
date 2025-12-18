package create

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	targetArg   string
	name        string
	value       string
	secret      bool
	readOnly    bool
	promptValue bool
	exporter    util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "create [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME --name VARIABLE_NAME",
		Short: "Create a variable in a variable group",
		Long: heredoc.Doc(`
				Add a variable to an existing variable group. Secret values are write-only and will be redacted in
				human output and omitted from JSON output.
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.targetArg = args[0]
			return run(ctx, o)
		},
	}

	cmd.Flags().StringVar(&o.name, "name", "", "Variable name to add (case-insensitive)")
	cmd.Flags().StringVar(&o.value, "value", "", "Literal value for the variable")
	cmd.Flags().BoolVar(&o.secret, "secret", false, "Mark the variable as secret (write-only)")
	cmd.Flags().BoolVar(&o.readOnly, "read-only", false, "Set the variable read-only")
	cmd.Flags().BoolVar(&o.promptValue, "prompt-value", false, "Prompt securely for a secret value (only valid with --secret)")
	_ = cmd.MarkFlagRequired("name")

	// JSON flags expose only the created variable fields
	util.AddJSONFlags(cmd, &o.exporter, []string{"name", "secret", "value", "readOnly"})

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
	if group.Type != nil && strings.EqualFold(types.GetValue(group.Type, ""), "AzureKeyVault") {
		return util.FlagErrorf("cannot add variables to an Azure Key Vault-backed variable group")
	}

	// Check duplicate (case-insensitive)
	if group.Variables != nil {
		for k := range *group.Variables {
			if strings.EqualFold(k, opts.name) {
				return util.FlagErrorf("variable %q already exists in group %q", opts.name, types.GetValue(group.Name, scope.Target))
			}
		}
	}

	// Resolve value
	var finalValue string
	if opts.secret {
		if opts.value != "" {
			finalValue = opts.value
		} else if opts.promptValue {
			// env lookup
			envKey := strings.ToUpper("AZDO_PIPELINES_SECRET_" + opts.name)
			if v, ok := os.LookupEnv(envKey); ok {
				finalValue = v
			} else {
				if !ios.CanPrompt() {
					return util.FlagErrorf("no value provided for secret %q and prompting is disabled", opts.name)
				}
				ios.StopProgressIndicator()
				prompter, err := cmdCtx.Prompter()
				if err != nil {
					return err
				}
				secret, err := prompter.Secret(fmt.Sprintf("Value for secret %q:", opts.name))
				ios.StartProgressIndicator()
				if err != nil {
					return err
				}
				finalValue = secret
			}
		} else {
			return util.FlagErrorf("secret value required; provide --value or --prompt-value")
		}
	} else {
		if opts.value == "" {
			return util.FlagErrorf("--value is required for non-secret variables")
		}
		finalValue = opts.value
	}

	// Insert into variables map
	newVars := map[string]interface{}{}
	if group.Variables != nil {
		for k, v := range *group.Variables {
			newVars[k] = v
		}
	}
	newVars[opts.name] = taskagent.VariableValue{
		Value:      types.ToPtr(finalValue),
		IsSecret:   types.ToPtr(opts.secret),
		IsReadOnly: types.ToPtr(opts.readOnly),
	}

	groupID := 0
	if group.Id != nil {
		groupID = types.GetValue(group.Id, 0)
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

	// JSON output: emit only the created variable
	if opts.exporter != nil {
		type createdVarView struct {
			Name     string  `json:"name"`
			Secret   *bool   `json:"secret,omitempty"`
			Value    *string `json:"value,omitempty"`
			ReadOnly *bool   `json:"readOnly,omitempty"`
		}
		var valuePtr *string
		if !opts.secret {
			valuePtr = types.ToPtr(finalValue)
		}
		view := createdVarView{
			Name:     opts.name,
			Secret:   types.ToPtr(opts.secret),
			Value:    valuePtr,
			ReadOnly: types.ToPtr(opts.readOnly),
		}
		return opts.exporter.Write(ios, view)
	}

	tp, err := cmdCtx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("NAME", "VALUE", "IS SECRET", "IS READONLY")
	tp.EndRow()
	valueDisplayed := fmt.Sprintf("%v", finalValue)
	if opts.secret {
		valueDisplayed = "***"
	}
	tp.AddField(opts.name)
	tp.AddField(valueDisplayed)
	tp.AddField(strconv.FormatBool(opts.secret))
	tp.AddField(strconv.FormatBool(opts.readOnly))
	tp.EndRow()
	return tp.Render()
}
