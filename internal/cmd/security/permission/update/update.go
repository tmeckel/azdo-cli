package update

import (
	"fmt"
	"strconv"
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
	allowBits   []string
	denyBits    []string
	merge       bool
	yes         bool
}

type AccessControlEntryUpdate struct {
	Token                string                        `json:"token"`
	Merge                bool                          `json:"merge"`
	AccessControlEntries []security.AccessControlEntry `json:"accessControlEntries"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "update <TARGET>",
		Short: "Update or create permissions for a user or group.",
		Long: heredoc.Doc(`
			Update the permissions for a user or group on a specific securable resource (identified by a token) by assigning "allow" or "deny" permission bits.

			The --allow-bit and --deny-bit flags accept one or more permission values. Each value may be provided as:
			  - a hexadecimal bitmask (e.g. 0x4),
			  - a decimal bit value (e.g. 4), or
			  - a textual action name matching the namespace action (e.g. "Read", "Edit").

			To discover the available actions (and their textual names) for a security namespace, use:
			  azdo security permission namespace show --namespace-id <namespace-uuid>

			Accepted TARGET formats:
		  - ORGANIZATION/SUBJECT           → target subject in org
		  - ORGANIZATION/PROJECT/SUBJECT   → subject scoped to project

		Token hierarchy (Git repo namespace example):
		  - repoV2 → all repos across org
		  - repoV2/{projectId} → all repos in project
		  - repoV2/{projectId}/{repoId} → single repo in project


			  - ORGANIZATION/SUBJECT           → target subject in org
			  - ORGANIZATION/PROJECT/SUBJECT   → subject scoped to project
		`),
		Example: heredoc.Doc(`
			# Allow the Read action (textual) for a user on a token
			azdo security permission update fabrikam/contoso@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f' --allow-bit Read

			# Allow multiple actions by specifying --allow-bit multiple times (textual and numeric)
			azdo security permission update fabrikam/contoso@example.com --namespace-id bf7bfa03-b2b7-47db-8113-fa2e002cc5b1 --token vstfs:///Classification/Node/18c76992-93fa-4eb2-aac0-0abc0be212d6 --allow-bit Read --allow-bit Contribute --allow-bit 0x4

			# Allow multiple actions using a single comma-separated value (shells may need quoting)
			azdo security permission update fabrikam/contoso@example.com --namespace-id 302acaca-b667-436d-a946-87133492041c --token BuildPrivileges --allow-bit "Read,Contribute,0x4"

			# Deny a numeric bit and merge with existing ACEs (merge will OR incoming bits with existing ACE)
			azdo security permission update fabrikam/contoso@example.com --namespace-id 33344d9c-fc72-4d6f-aba5-fa317101a7e9 --token '696416ee-f7ff-4ee3-934a-979b00dce74f/237' --deny-bit 8 --merge

			# Use --yes to skip confirmation prompts
			azdo security permission update fabrikam/contoso@example.com --namespace-id 8adf73b7-389a-4276-b638-fe1653f7efc7 --token '$/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd/00000000-0000-0000-0000-000000000000' --allow-bit Read --yes
		`),
		Aliases: []string{
			"create",
			"u",
			"new",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.rawTarget = args[0]
			return runCommand(ctx, o)
		},
	}

	cmd.Flags().StringVarP(&o.namespaceID, "namespace-id", "n", "", "ID of the security namespace to modify (required).")
	cmd.Flags().StringVar(&o.token, "token", "", "Security token for the resource (required).")
	cmd.Flags().StringSliceVar(&o.allowBits, "allow-bit", []string{}, "Permission bit or comma-separated bits to allow.")
	cmd.Flags().StringSliceVar(&o.denyBits, "deny-bit", []string{}, "Permission bit or comma-separated bits to deny.")
	cmd.Flags().BoolVarP(&o.yes, "yes", "y", false, "Do not prompt for confirmation.")
	cmd.Flags().BoolVar(&o.merge, "merge", false, "Merge incoming ACEs with existing entries or replace the permissions. If provided without value true is implied.")
	cmd.Flags().Lookup("merge").NoOptDefVal = "true"

	return cmd
}

func runCommand(ctx util.CmdContext, o *opts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if strings.TrimSpace(o.namespaceID) == "" {
		return util.FlagErrorf("--namespace-id is required")
	}
	if strings.TrimSpace(o.token) == "" {
		return util.FlagErrorf("--token is required")
	}

	namespaceUUID, err := uuid.Parse(strings.TrimSpace(o.namespaceID))
	if err != nil {
		return util.FlagErrorf("invalid namespace id %q: %v", o.namespaceID, err)
	}

	scope, err := shared.ParseSubjectTarget(ctx, o.rawTarget)
	if err != nil {
		return err
	}

	hasSubject := scope.Subject != ""
	if !hasSubject {
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
	if member.Descriptor == nil {
		return fmt.Errorf("identity %q does not have a descriptor", member.Id.String())
	}
	securityClient, err := ctx.ClientFactory().Security(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create security client: %w", err)
	}

	// load namespace action definitions so we can accept textual action names
	nsDetails, err := securityClient.QuerySecurityNamespaces(ctx.Context(), security.QuerySecurityNamespacesArgs{
		SecurityNamespaceId: &namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to load namespace actions: %w", err)
	}
	actions := shared.ExtractNamespaceActions(nsDetails)

	// require at least one of the flags to be provided
	if len(o.allowBits) == 0 && len(o.denyBits) == 0 {
		return util.FlagErrorf("at least one of --allow-bit or --deny-bit must be provided")
	}

	var allowVal *int
	var denyVal *int
	if len(o.allowBits) > 0 {
		v, err := parseBits(actions, o.allowBits)
		if err != nil {
			return err
		}
		allowVal = &v
	}
	if len(o.denyBits) > 0 {
		v, err := parseBits(actions, o.denyBits)
		if err != nil {
			return err
		}
		denyVal = &v
	}

	ace := security.AccessControlEntry{
		Descriptor: member.Descriptor,
		Allow:      allowVal,
		Deny:       denyVal,
	}

	container := AccessControlEntryUpdate{
		Token:                strings.TrimSpace(o.token),
		Merge:                o.merge,
		AccessControlEntries: []security.AccessControlEntry{ace},
	}

	zap.L().Sugar().Debugf("Setting ACE token=%q descriptor=%q allow=%v deny=%v merge=%v", o.token, *member.Descriptor, allowVal, denyVal, o.merge)

	_, err = securityClient.SetAccessControlEntries(ctx.Context(), security.SetAccessControlEntriesArgs{
		Container:           container,
		SecurityNamespaceId: &namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to set access control entries: %w", err)
	}

	ios.StopProgressIndicator()
	fmt.Fprintln(ios.Out, "Permissions updated.")
	return nil
}

func parseBits(actions []security.ActionDefinition, parts []string) (int, error) {
	var val int

	// Build a map for quick name->bit lookup (case-insensitive) and track all allowed bits.
	nameMap := make(map[string]int)
	var allowedMask int
	for _, a := range actions {
		bit := types.GetValue(a.Bit, 0)
		if bit == 0 {
			continue
		}

		allowedMask |= bit

		if n := strings.TrimSpace(types.GetValue(a.Name, "")); n != "" {
			nameMap[strings.ToLower(n)] = bit
		}
		if dn := strings.TrimSpace(types.GetValue(a.DisplayName, "")); dn != "" {
			nameMap[strings.ToLower(dn)] = bit
		}
		nameMap[strings.ToLower(fmt.Sprintf("bit %d", bit))] = bit
	}

	checkAllowed := func(bitVal int) error {
		if bitVal == 0 {
			return fmt.Errorf("permission bit value cannot be zero")
		}
		if allowedMask != 0 && bitVal&^allowedMask != 0 {
			return fmt.Errorf("permission bit value %d is not defined for this namespace", bitVal)
		}
		return nil
	}

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// numeric hex: 0x prefix
		if strings.HasPrefix(p, "0x") || strings.HasPrefix(p, "0X") {
			v, err := strconv.ParseInt(p[2:], 16, 32)
			if err != nil {
				return 0, fmt.Errorf("invalid bit value %q: %w", p, err)
			}
			candidate := int(v)
			if err := checkAllowed(candidate); err != nil {
				return 0, err
			}
			val |= candidate
			continue
		}

		// numeric decimal
		if d, err := strconv.ParseInt(p, 10, 32); err == nil {
			candidate := int(d)
			if err := checkAllowed(candidate); err != nil {
				return 0, err
			}
			val |= candidate
			continue
		}

		// textual name match (case-insensitive)
		l := strings.ToLower(p)
		if bit, ok := nameMap[l]; ok {
			val |= bit
			continue
		}

		return 0, fmt.Errorf("unrecognized permission token %q", p)
	}

	return val, nil
}
