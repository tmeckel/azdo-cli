package delete

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
	yes         bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "delete <TARGET>",
		Short: "Delete permissions for a user or group.",
		Long: heredoc.Doc(`
			Delete every explicit permission entry (allow or deny) for a user or group on a securable resource.

			Accepted TARGET formats:
			  - ORGANIZATION/SUBJECT           → delete permissions in an organization scope
			  - ORGANIZATION/PROJECT/SUBJECT   → delete permissions scoped to a project
		`),
		Example: heredoc.Doc(`
			# Prompt before deleting permissions
			azdo security permission delete fabrikam/contoso@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f'

			# Delete permissions without confirmation
			azdo security permission delete fabrikam/contoso@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f' --yes

			# Delete project-scoped permissions
			azdo security permission delete fabrikam/ProjectAlpha/vssgp.Uy0xLTktMTIzNDU2 --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token 'repoV2/{projectId}/{repoId}'
		`),
		Aliases: []string{
			"d",
			"del",
			"rm",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.rawTarget = args[0]
			return runCommand(ctx, o)
		},
	}

	cmd.Flags().StringVarP(&o.namespaceID, "namespace-id", "n", "", "ID of the security namespace to modify (required).")
	cmd.Flags().StringVar(&o.token, "token", "", "Security token to delete (required).")
	cmd.Flags().BoolVarP(&o.yes, "yes", "y", false, "Do not prompt for confirmation.")

	_ = cmd.MarkFlagRequired("namespace-id")
	_ = cmd.MarkFlagRequired("token")

	return cmd
}

func runCommand(ctx util.CmdContext, o *opts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	namespaceValue := strings.TrimSpace(o.namespaceID)
	if namespaceValue == "" {
		return util.FlagErrorf("--namespace-id is required")
	}
	tokenValue := strings.TrimSpace(o.token)
	if tokenValue == "" {
		return util.FlagErrorf("--token is required")
	}

	namespaceUUID, err := uuid.Parse(namespaceValue)
	if err != nil {
		return util.FlagErrorf("invalid namespace id %q: %v", o.namespaceID, err)
	}

	scope, err := shared.ParseSubjectTarget(ctx, o.rawTarget)
	if err != nil {
		return err
	}

	if scope.Subject == "" {
		return util.FlagErrorf("a subject is required")
	}

	zap.L().Debug("Resolved target for permission delete",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("subject", scope.Subject),
	)

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
		return fmt.Errorf("identity %q does not have a descriptor", scope.Subject)
	}

	securityClient, err := ctx.ClientFactory().Security(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create security client: %w", err)
	}

	if !o.yes {
		ios.StopProgressIndicator()

		p, err := ctx.Prompter()
		if err != nil {
			return err
		}

		confirmed, err := p.Confirm(fmt.Sprintf("Delete permissions for %q on %q?", scope.Subject, tokenValue), false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}

		ios.StartProgressIndicator()
	}

	zap.L().Debug("Removing access control entries",
		zap.String("namespaceId", namespaceUUID.String()),
		zap.String("token", tokenValue),
		zap.String("descriptor", descriptor),
	)

	result, err := securityClient.RemoveAccessControlEntries(ctx.Context(), security.RemoveAccessControlEntriesArgs{
		SecurityNamespaceId: &namespaceUUID,
		Token:               &tokenValue,
		Descriptors:         &descriptor,
	})
	if err != nil {
		return fmt.Errorf("failed to delete permissions: %w", err)
	}
	if result == nil || !types.GetValue(result, false) {
		return fmt.Errorf("failed to delete permissions: service returned no confirmation")
	}

	zap.L().Debug("Verifying permissions are removed",
		zap.String("token", tokenValue),
		zap.String("descriptor", descriptor),
	)

	response, err := securityClient.QueryAccessControlLists(ctx.Context(), security.QueryAccessControlListsArgs{
		SecurityNamespaceId: &namespaceUUID,
		Token:               &tokenValue,
		Descriptors:         &descriptor,
		IncludeExtendedInfo: types.ToPtr(true),
	})
	if err != nil {
		return fmt.Errorf("failed to verify permissions deletion: %w", err)
	}

	if aclHasDescriptor(response, descriptor) {
		return fmt.Errorf("descriptor %q still has permissions on token %q", descriptor, tokenValue)
	}

	ios.StopProgressIndicator()

	fmt.Fprintln(ios.Out, "Permissions deleted.")
	return nil
}

func aclHasDescriptor(acls *[]security.AccessControlList, descriptor string) bool {
	if acls == nil {
		return false
	}

	for _, acl := range *acls {
		if acl.AcesDictionary == nil || len(*acl.AcesDictionary) == 0 {
			continue
		}

		for key, ace := range *acl.AcesDictionary {
			candidate := strings.TrimSpace(types.GetValue(ace.Descriptor, key))
			if candidate == descriptor {
				hasAllow := ace.Allow != nil && types.GetValue(ace.Allow, 0) != 0
				hasDeny := ace.Deny != nil && types.GetValue(ace.Deny, 0) != 0
				if hasAllow || hasDeny {
					return true
				}
				zap.L().Debug("Skipping empty ACL entry for descriptor",
					zap.String("descriptor", descriptor),
					zap.Int("allow", types.GetValue(ace.Allow, 0)),
					zap.Int("deny", types.GetValue(ace.Deny, 0)),
				)
			}
		}
	}

	return false
}
