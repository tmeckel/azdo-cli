package reset

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	rawTarget   string
	namespaceID string
	token       string
	permission  []string
	yes         bool
	exporter    util.Exporter
}

type permissionResult struct {
	Bit         int     `json:"bit"`
	Name        *string `json:"name,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
	Effective   string  `json:"effective"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "reset <TARGET>",
		Short: "Reset explicit permission bits for a user or group.",
		Long: heredoc.Doc(`
			Reset the explicit allow/deny permission bits for a user or group on a securable resource (identified by a token).

			The --permission-bit flag accepts one or more permission values. Each value may be provided as:
			  - a hexadecimal bitmask (e.g. 0x4),
			  - a decimal bit value (e.g. 4), or
			  - a textual action name or display name matching the namespace action (e.g. "Read").

			Accepted TARGET formats:
			  - ORGANIZATION/SUBJECT
			  - ORGANIZATION/PROJECT/SUBJECT
		`),
		Example: heredoc.Doc(`
			# Reset the Read action (textual) for a user on a token
			azdo security permission reset fabrikam/user@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f' --permission-bit Read

			# Reset multiple actions by specifying --permission-bit multiple times
			azdo security permission reset fabrikam/user@example.com --namespace-id bf7bfa03-b2b7-47db-8113-fa2e002cc5b1 --token vstfs:///Classification/Node/18c76992-93fa-4eb2-aac0-0abc0be212d6 --permission-bit Read --permission-bit Contribute

			# Reset multiple actions using a single comma-separated value (shells may need quoting)
			azdo security permission reset fabrikam/user@example.com --namespace-id 302acaca-b667-436d-a946-87133492041c --token BuildPrivileges --permission-bit "Read,Contribute,0x4"

			# Use --yes to skip confirmation prompts
			azdo security permission reset fabrikam/user@example.com --namespace-id 8adf73b7-389a-4276-b638-fe1653f7efc7 --token '$/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd/00000000-0000-0000-0000-000000000000' --permission-bit Read --yes
		`),
		Aliases: []string{
			"r",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.rawTarget = args[0]
			return runCommand(ctx, o)
		},
	}

	cmd.Flags().StringVarP(&o.namespaceID, "namespace-id", "n", "", "ID of the security namespace to modify (required).")
	cmd.Flags().StringVar(&o.token, "token", "", "Security token for the resource (required).")
	cmd.Flags().StringSliceVar(&o.permission, "permission-bit", []string{}, "Permission bit or comma-separated bits to reset (required).")
	cmd.Flags().BoolVarP(&o.yes, "yes", "y", false, "Do not prompt for confirmation.")
	util.AddJSONFlags(cmd, &o.exporter, []string{
		"bit",
		"name",
		"displayName",
		"effective",
	})

	_ = cmd.MarkFlagRequired("namespace-id")
	_ = cmd.MarkFlagRequired("token")
	_ = cmd.MarkFlagRequired("permission-bit")

	return cmd
}

func runCommand(ctx util.CmdContext, o *opts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	namespaceID := strings.TrimSpace(o.namespaceID)
	if namespaceID == "" {
		return util.FlagErrorf("--namespace-id is required")
	}
	token := strings.TrimSpace(o.token)
	if token == "" {
		return util.FlagErrorf("--token is required")
	}
	if len(o.permission) == 0 {
		return util.FlagErrorf("--permission-bit is required")
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return util.FlagErrorf("invalid namespace id %q: %v", o.namespaceID, err)
	}

	scope, err := shared.ParseSubjectTarget(ctx, o.rawTarget)
	if err != nil {
		return err
	}

	zap.L().Debug("Parsed permission reset target",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("subject", scope.Subject))

	if strings.TrimSpace(scope.Subject) == "" {
		return util.FlagErrorf("a subject is required")
	}

	if scope.Project != "" {
		if _, _, err := util.ResolveScopeDescriptor(ctx, scope.Organization, scope.Project); err != nil {
			return err
		}
	}

	extensionsClient, err := ctx.ClientFactory().Extensions(ctx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	member, err := extensionsClient.ResolveIdentity(ctx.Context(), scope.Subject)
	if err != nil {
		return fmt.Errorf("failed to resolve identity %q: %w", scope.Subject, err)
	}

	descriptor := strings.TrimSpace(types.GetValue(member.Descriptor, ""))
	if descriptor == "" {
		return fmt.Errorf("resolved identity does not contain a descriptor for %q", scope.Subject)
	}

	zap.L().Debug("Resolved identity descriptor for permission reset",
		zap.String("descriptor", descriptor))

	securityClient, err := ctx.ClientFactory().Security(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create security client: %w", err)
	}

	nsDetails, err := securityClient.QuerySecurityNamespaces(ctx.Context(), security.QuerySecurityNamespacesArgs{
		SecurityNamespaceId: &namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to load namespace actions: %w", err)
	}
	actions := shared.ExtractNamespaceActions(nsDetails)

	bitmask, err := shared.ParsePermissionBits(actions, o.permission)
	if err != nil {
		return err
	}
	if bitmask == 0 {
		return util.FlagErrorf("at least one --permission-bit value must be provided")
	}

	zap.L().Debug("Computed permission bitmask for reset",
		zap.String("namespaceId", namespaceUUID.String()),
		zap.Int("bitmask", bitmask),
		zap.String("token", token))

	if !o.yes {
		ios.StopProgressIndicator()
		p, err := ctx.Prompter()
		if err != nil {
			return err
		}
		message := fmt.Sprintf("Reset permissions for %s on %s?", scope.Subject, token)
		confirmed, err := p.Confirm(message, false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
		ios.StartProgressIndicator()
	}

	permissions := bitmask
	tokenCopy := token
	_, err = securityClient.RemovePermission(ctx.Context(), security.RemovePermissionArgs{
		SecurityNamespaceId: &namespaceUUID,
		Descriptor:          &descriptor,
		Permissions:         &permissions,
		Token:               &tokenCopy,
	})
	if err != nil {
		return fmt.Errorf("failed to reset permissions: %w", err)
	}

	response, err := securityClient.QueryAccessControlLists(ctx.Context(), security.QueryAccessControlListsArgs{
		SecurityNamespaceId: &namespaceUUID,
		Token:               &tokenCopy,
		Descriptors:         &descriptor,
		IncludeExtendedInfo: types.ToPtr(true),
	})
	if err != nil {
		return fmt.Errorf("failed to query updated permissions: %w", err)
	}

	ios.StopProgressIndicator()

	ace := extractDescriptorEntry(response, descriptor)
	if ace == nil {
		fmt.Fprintln(ios.Out, "No permissions changed.")
		return nil
	}

	results := summarizePermissions(actions, bitmask, ace)
	if len(results) == 0 {
		fmt.Fprintln(ios.Out, "No permissions changed.")
		return nil
	}

	if o.exporter != nil {
		return o.exporter.Write(ios, results)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("Action", "Bit", "Effective")
	tp.EndRow()

	for _, entry := range results {
		actionName := strings.TrimSpace(types.GetValue(entry.DisplayName, ""))
		if actionName == "" {
			actionName = strings.TrimSpace(types.GetValue(entry.Name, ""))
		}
		if actionName == "" {
			actionName = fmt.Sprintf("Bit %d", entry.Bit)
		}

		tp.AddField(actionName)
		tp.AddField(fmt.Sprintf("%d (0x%X)", entry.Bit, entry.Bit))
		tp.AddField(entry.Effective)
		tp.EndRow()
	}

	return tp.Render()
}

func extractDescriptorEntry(response *[]security.AccessControlList, descriptor string) *security.AccessControlEntry {
	if response == nil || len(*response) == 0 {
		return nil
	}

	for _, acl := range *response {
		if acl.AcesDictionary == nil || len(*acl.AcesDictionary) == 0 {
			continue
		}

		if entry, ok := (*acl.AcesDictionary)[descriptor]; ok {
			return shared.CloneAccessControlEntry(&entry)
		}

		for key, entry := range *acl.AcesDictionary {
			if strings.EqualFold(strings.TrimSpace(key), descriptor) {
				return shared.CloneAccessControlEntry(&entry)
			}
		}
	}

	return nil
}

func summarizePermissions(actions []security.ActionDefinition, requestedBits int, ace *security.AccessControlEntry) []permissionResult {
	if ace == nil || requestedBits == 0 {
		return nil
	}

	results := make([]permissionResult, 0)

	allowBits := types.GetValue(ace.Allow, 0)
	denyBits := types.GetValue(ace.Deny, 0)

	effectiveAllow := allowBits
	effectiveDeny := denyBits
	inheritedAllow := 0
	inheritedDeny := 0

	if ace.ExtendedInfo != nil {
		effectiveAllow = types.GetValue(ace.ExtendedInfo.EffectiveAllow, effectiveAllow)
		effectiveDeny = types.GetValue(ace.ExtendedInfo.EffectiveDeny, effectiveDeny)
		inheritedAllow = effectiveAllow ^ allowBits
		inheritedDeny = effectiveDeny ^ denyBits
	}

	for _, action := range actions {
		bit := types.GetValue(action.Bit, 0)
		if bit == 0 || requestedBits&bit != bit {
			continue
		}

		name := strings.TrimSpace(types.GetValue(action.Name, ""))
		displayName := strings.TrimSpace(types.GetValue(action.DisplayName, ""))

		state := "Not set"
		if effectiveDeny&bit == bit {
			state = "Deny"
			if inheritedDeny&bit == bit {
				state = "Deny (inherited)"
			}
		} else if effectiveAllow&bit == bit {
			state = "Allow"
			if inheritedAllow&bit == bit {
				state = "Allow (inherited)"
			}
		}

		entry := permissionResult{
			Bit:       bit,
			Effective: state,
		}
		if name != "" {
			entry.Name = types.ToPtr(name)
		}
		if displayName != "" {
			entry.DisplayName = types.ToPtr(displayName)
		}
		results = append(results, entry)
	}

	return results
}
