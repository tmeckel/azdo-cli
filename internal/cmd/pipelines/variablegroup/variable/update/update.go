package update

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

type opts struct {
	targetArg   string
	name        string
	newName     string
	value       string
	secret      bool
	readOnly    bool
	promptValue bool
	clearValue  bool
	fromJSON    string
	yes         bool
	exporter    util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "update [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME --name VARIABLE_NAME",
		Short: "Update a variable in a variable group",
		Long: heredoc.Doc(`
				Update an existing variable in a variable group. Supports renaming, value changes,
				toggling secret/read-only flags, prompting for secret values, and applying changes
				from JSON. Secret values are write-only and will be redacted in human output and
				omitted from JSON output.
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.targetArg = args[0]
			return run(ctx, cmd, o)
		},
	}

	cmd.Flags().StringVar(&o.name, "name", "", "Variable name to update (case-insensitive)")
	cmd.Flags().StringVar(&o.newName, "new-name", "", "Rename the variable (case-insensitive)")
	cmd.Flags().StringVar(&o.value, "value", "", "Replace the variable value")
	cmd.Flags().BoolVar(&o.secret, "secret", false, "Set variable as secret (tri-state: only when explicitly set)")
	cmd.Flags().BoolVar(&o.readOnly, "read-only", false, "Set variable read-only (tri-state: only when explicitly set)")
	cmd.Flags().BoolVar(&o.promptValue, "prompt-value", false, "Prompt securely for a secret value (write-only)")
	cmd.Flags().BoolVar(&o.clearValue, "clear-value", false, "Clear the stored value for a non-secret variable (destructive)")
	cmd.Flags().StringVar(&o.fromJSON, "from-json", "", "Apply updates from JSON (file path, '-', or inline JSON)")
	cmd.Flags().BoolVar(&o.yes, "yes", false, "Skip confirmation prompts for destructive operations")
	_ = cmd.MarkFlagRequired("name")

	// JSON output exposes only the updated variable fields (matching create behavior)
	util.AddJSONFlags(cmd, &o.exporter, []string{"name", "secret", "value", "readOnly"})

	return cmd
}

func run(cmdCtx util.CmdContext, cmd *cobra.Command, opts *opts) error {
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
		return util.FlagErrorf("variable group %q not found", scope.Target)
	}

	// Load --from-json if provided
	var fromPayload struct {
		NewName    *string `json:"newName"`
		Value      *string `json:"value"`
		Secret     *bool   `json:"secret"`
		ReadOnly   *bool   `json:"readOnly"`
		ClearValue *bool   `json:"clearValue"`
	}
	fromJSONSet := false
	if cmd.Flags().Changed("from-json") {
		fromJSONSet = true
		var raw []byte
		var err error
		if opts.fromJSON == "-" {
			raw, err = io.ReadAll(ios.In)
			if err != nil {
				return util.FlagErrorWrap(err)
			}
		} else {
			if st, err := os.Stat(opts.fromJSON); err == nil && !st.IsDir() {
				raw, err = os.ReadFile(opts.fromJSON)
				if err != nil {
					return util.FlagErrorWrap(err)
				}
			} else {
				raw = []byte(opts.fromJSON)
			}
		}
		// Ensure payload does not include "name"
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keys); err != nil {
			return util.FlagErrorWrap(err)
		}
		if _, ok := keys["name"]; ok {
			return util.FlagErrorf("--from-json payload must not include 'name'; pass the variable name via --name")
		}
		if err := json.Unmarshal(raw, &fromPayload); err != nil {
			return util.FlagErrorWrap(err)
		}
	}

	// Validate mutual exclusivity when --from-json is set
	if fromJSONSet {
		// disallow mixing with other change flags
		disallowed := []string{"new-name", "value", "secret", "read-only", "prompt-value", "clear-value"}
		for _, f := range disallowed {
			if cmd.Flags().Changed(f) {
				return util.FlagErrorf("--from-json cannot be combined with --%s", f)
			}
		}
	}

	// Ensure at least one change requested
	changeRequested := false
	if fromJSONSet {
		if fromPayload.NewName != nil || fromPayload.Value != nil || fromPayload.Secret != nil || fromPayload.ReadOnly != nil || fromPayload.ClearValue != nil {
			changeRequested = true
		}
	} else {
		if cmd.Flags().Changed("new-name") || cmd.Flags().Changed("value") || cmd.Flags().Changed("secret") || cmd.Flags().Changed("read-only") || opts.promptValue || opts.clearValue {
			changeRequested = true
		}
	}
	if !changeRequested {
		return util.FlagErrorf("no changes requested; provide one of --new-name, --value, --secret, --read-only, --prompt-value, --clear-value, or --from-json")
	}

	// Ensure variable exists (case-insensitive lookup)
	var origKey string
	var origVal any
	if group.Variables != nil {
		for k, v := range *group.Variables {
			if strings.EqualFold(k, opts.name) {
				origKey = k
				origVal = v
				break
			}
		}
	}
	if origKey == "" {
		return util.FlagErrorf("variable %q not found in group %q", opts.name, types.GetValue(group.Name, scope.Target))
	}

	// Detect if this is an Azure Key Vault-backed group
	if group.Type != nil && strings.EqualFold(types.GetValue(group.Type, ""), "AzureKeyVault") {
		// Disallow any value modifications
		if (fromJSONSet && fromPayload.Value != nil) || cmd.Flags().Changed("value") || opts.promptValue || opts.clearValue {
			return util.FlagErrorf("cannot modify variable values for an Azure Key Vault-backed variable group")
		}
	}

	// Extract current variable info
	var isSecret bool
	var isReadOnly bool
	var currentValue any
	if m, ok := origVal.(map[string]interface{}); ok {
		if s, ok := m["isSecret"].(bool); ok {
			isSecret = s
		}
		if r, ok := m["isReadOnly"].(bool); ok {
			isReadOnly = r
		}
		if v, ok := m["value"]; ok {
			currentValue = v
		}
	}

	// Validate clear-value only for non-secret
	if (fromJSONSet && fromPayload.ClearValue != nil && *fromPayload.ClearValue) || opts.clearValue {
		if isSecret {
			return util.FlagErrorf("cannot clear value of a secret variable")
		}
	}

	// If from-json requests secret=true but no value present, error
	if fromJSONSet && fromPayload.Secret != nil && *fromPayload.Secret {
		if fromPayload.Value == nil {
			return util.FlagErrorf("setting secret=true via --from-json requires a 'value' in the payload")
		}
	}

	// Prepare mutated variables map
	newVars := map[string]interface{}{}
	if group.Variables != nil {
		for k, v := range *group.Variables {
			newVars[k] = v
		}
	}

	// Determine new key (handle rename)
	finalKey := origKey
	if fromJSONSet && fromPayload.NewName != nil {
		// check collision
		for k := range newVars {
			if strings.EqualFold(k, *fromPayload.NewName) && !strings.EqualFold(k, origKey) {
				return util.FlagErrorf("variable %q already exists in group %q", *fromPayload.NewName, types.GetValue(group.Name, scope.Target))
			}
		}
		finalKey = *fromPayload.NewName
	} else if cmd.Flags().Changed("new-name") {
		// check collision
		for k := range newVars {
			if strings.EqualFold(k, opts.newName) && !strings.EqualFold(k, origKey) {
				return util.FlagErrorf("variable %q already exists in group %q", opts.newName, types.GetValue(group.Name, scope.Target))
			}
		}
		finalKey = opts.newName
	}

	// Build the new VariableValue
	// Start by copying existing map for the origKey (if present)
	var finalValue any
	finalIsSecret := isSecret
	finalIsReadOnly := isReadOnly

	// Apply tri-state flags and value changes
	if fromJSONSet {
		if fromPayload.Value != nil {
			finalValue = *fromPayload.Value
		}
		if fromPayload.Secret != nil {
			finalIsSecret = *fromPayload.Secret
		}
		if fromPayload.ReadOnly != nil {
			finalIsReadOnly = *fromPayload.ReadOnly
		}
		if fromPayload.ClearValue != nil && *fromPayload.ClearValue {
			finalValue = nil
		}
	} else {
		if cmd.Flags().Changed("value") {
			finalValue = opts.value
		}
		if cmd.Flags().Changed("secret") {
			finalIsSecret = opts.secret
		}
		if cmd.Flags().Changed("read-only") {
			finalIsReadOnly = opts.readOnly
		}
		if opts.clearValue {
			finalValue = nil
		}
		if opts.promptValue {
			// Only valid for prompting for secrets
			if !ios.CanPrompt() {
				return util.FlagErrorf("prompting for secret value is not supported in this environment")
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
			finalIsSecret = true
		}
	}

	// If finalValue is unset, try to preserve currentValue unless clear was requested
	if finalValue == nil && !((fromJSONSet && fromPayload.ClearValue != nil && *fromPayload.ClearValue) || opts.clearValue) {
		finalValue = currentValue
	}

	// Apply rename: remove old key if different
	if !strings.EqualFold(finalKey, origKey) {
		delete(newVars, origKey)
	}

	// Insert updated entry
	newVars[finalKey] = map[string]interface{}{
		"value":      finalValue,
		"isSecret":   finalIsSecret,
		"isReadOnly": finalIsReadOnly,
	}

	// If clear-value was requested and not suppressed by --yes, prompt for confirmation
	if (cmd.Flags().Changed("clear-value") && opts.clearValue) || (fromJSONSet && fromPayload.ClearValue != nil && *fromPayload.ClearValue) {
		if !opts.yes {
			ios.StopProgressIndicator()
			prompter, err := cmdCtx.Prompter()
			if err != nil {
				return err
			}
			ok, err := prompter.Confirm(fmt.Sprintf("Clear value of variable '%s' in group '%s'?", opts.name, types.GetValue(group.Name, scope.Target)), false)
			ios.StartProgressIndicator()
			if err != nil {
				return err
			}
			if !ok {
				return util.ErrCancel
			}
		}
	}

	// Persist
	groupID := 0
	if group.Id != nil {
		groupID = types.GetValue(group.Id, 0)
	}

	params := taskagent.VariableGroupParameters{
		Name:      group.Name,
		Variables: &newVars,
	}

	zap.L().Debug("Updating variable group", zap.Int("groupID", groupID), zap.String("project", scope.Project))
	_, err = taskClient.UpdateVariableGroup(cmdCtx.Context(), taskagent.UpdateVariableGroupArgs{
		VariableGroupParameters: &params,
		GroupId:                 &groupID,
	})
	if err != nil {
		return fmt.Errorf("failed to update variable group %d: %w", groupID, err)
	}

	ios.StopProgressIndicator()

	// JSON output: emit only the updated variable (do not emit entire variable group)
	if opts.exporter != nil {
		type updatedVarView struct {
			Name     string  `json:"name"`
			Secret   *bool   `json:"secret,omitempty"`
			Value    *string `json:"value,omitempty"`
			ReadOnly *bool   `json:"readOnly,omitempty"`
		}
		var valuePtr *string
		if !finalIsSecret && finalValue != nil {
			s := fmt.Sprintf("%v", finalValue)
			valuePtr = &s
		}
		view := updatedVarView{
			Name:     finalKey,
			Secret:   types.ToPtr(finalIsSecret),
			Value:    valuePtr,
			ReadOnly: types.ToPtr(finalIsReadOnly),
		}
		return opts.exporter.Write(ios, view)
	}

	// Human-friendly table output
	tp, err := cmdCtx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("NAME", "VALUE", "IS SECRET", "IS READONLY")
	tp.EndRow()
	displayValue := ""
	if finalValue != nil {
		displayValue = fmt.Sprintf("%v", finalValue)
	}
	if finalIsSecret {
		displayValue = "***"
	}
	tp.AddField(finalKey)
	tp.AddField(displayValue)
	tp.AddField(strconv.FormatBool(finalIsSecret))
	tp.AddField(strconv.FormatBool(finalIsReadOnly))
	tp.EndRow()
	return tp.Render()
}
