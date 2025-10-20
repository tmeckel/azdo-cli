package list

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	groupShared "github.com/tmeckel/azdo-cli/internal/cmd/security/group/shared"
	permissionShared "github.com/tmeckel/azdo-cli/internal/cmd/security/permission/namespace/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	rawTarget   string
	namespaceID string
	token       string
	recurse     bool
	exporter    util.Exporter
}

type permissionEntry struct {
	Token              *string `json:"token,omitempty"`
	Descriptor         *string `json:"descriptor,omitempty"`
	InheritPermissions *bool   `json:"inheritPermissions,omitempty"`
	Allow              *string `json:"allow,omitempty"`
	Deny               *string `json:"deny,omitempty"`
	EffectiveAllow     *string `json:"effectiveAllow,omitempty"`
	EffectiveDeny      *string `json:"effectiveDeny,omitempty"`
	InheritedAllow     *string `json:"inheritedAllow,omitempty"`
	InheritedDeny      *string `json:"inheritedDeny,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "list [TARGET]",
		Short: "List security ACLs for a namespace, optionally filtered by subject.",
		Long: heredoc.Doc(`
			List security access control entries (ACEs) for an Azure DevOps security namespace.

			Accepted TARGET formats:
			  - (empty)                        → use the default organization
			  - ORGANIZATION                   → list all ACLs for the namespace in the organization
			  - ORGANIZATION/SUBJECT           → list ACLs for the specified subject
			  - ORGANIZATION/PROJECT/SUBJECT   → list ACLs for the subject scoped to the project
		`),
		Example: heredoc.Doc(`
			# List all ACLs for a namespace using the default organization
			azdo security permission list --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866

			# List all ACLs for a namespace in an explicit organization
			azdo security permission list fabrikam --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866

			# List all tokens for a specific user
			azdo security permission list fabrikam/contoso@example.com --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866

			# List ACLs for a project-scoped group
			azdo security permission list fabrikam/ProjectAlpha/vssgp.Uy0xLTktMTIzNDU2 --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866 --recurse
		`),
		Aliases: []string{
			"ls",
			"l",
		},
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.rawTarget = args[0]
			}
			return runCommand(ctx, o)
		},
	}

	cmd.Flags().StringVarP(&o.namespaceID, "namespace-id", "n", "", "ID of the security namespace to query (required).")
	cmd.Flags().StringVar(&o.token, "token", "", "Security token to filter the results.")
	cmd.Flags().BoolVar(&o.recurse, "recurse", false, "Include child ACLs for the specified token when supported.")
	util.AddJSONFlags(cmd, &o.exporter, []string{
		"token",
		"descriptor",
		"inheritPermissions",
		"allow",
		"deny",
		"effectiveAllow",
		"effectiveDeny",
		"inheritedAllow",
		"inheritedDeny",
	})

	return cmd
}

func runCommand(ctx util.CmdContext, o *opts) error {
	if strings.TrimSpace(o.namespaceID) == "" {
		return util.FlagErrorf("--namespace-id is required")
	}

	namespaceUUID, err := uuid.Parse(strings.TrimSpace(o.namespaceID))
	if err != nil {
		return util.FlagErrorf("invalid namespace id %q: %v", o.namespaceID, err)
	}

	scope, subject, hasSubject, err := parseSubjectTarget(ctx, o.rawTarget)
	if err != nil {
		return err
	}

	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	zap.L().Sugar().Debugf("Resolved scope organization=%q project=%q subject=%q", scope.Organization, scope.Project, subject)

	if hasSubject && scope.Project != "" {
		if _, _, err := util.ResolveScopeDescriptor(ctx, scope.Organization, scope.Project); err != nil {
			return err
		}
	}

	securityClient, err := ctx.ClientFactory().Security(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create security client: %w", err)
	}

	namespaceDetails, err := securityClient.QuerySecurityNamespaces(ctx.Context(), security.QuerySecurityNamespacesArgs{
		SecurityNamespaceId: &namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to load namespace actions: %w", err)
	}
	actionDefinitions := permissionShared.ExtractNamespaceActions(namespaceDetails)

	requestArgs := security.QueryAccessControlListsArgs{
		SecurityNamespaceId: &namespaceUUID,
		IncludeExtendedInfo: types.ToPtr(true),
	}

	var descriptor string
	if hasSubject {
		member, err := groupShared.ResolveMemberDescriptor(ctx, scope.Organization, subject)
		if err != nil {
			return fmt.Errorf("failed to resolve subject %q: %w", subject, err)
		}
		descriptor = strings.TrimSpace(types.GetValue(member.Descriptor, ""))
		if descriptor == "" {
			return fmt.Errorf("resolved subject descriptor is empty")
		}
		zap.L().Sugar().Debugf("Resolved subject descriptor=%q", descriptor)
		requestArgs.Descriptors = types.ToPtr(descriptor)
	}

	if strings.TrimSpace(o.token) != "" {
		token := strings.TrimSpace(o.token)
		requestArgs.Token = &token
	}
	if o.recurse {
		requestArgs.Recurse = types.ToPtr(true)
	}

	zap.L().Sugar().Debugf("Querying ACLs (token=%q recurse=%v subjectFilter=%v)", o.token, o.recurse, hasSubject)

	response, err := securityClient.QueryAccessControlLists(ctx.Context(), requestArgs)
	if err != nil {
		return fmt.Errorf("failed to query access control lists: %w", err)
	}

	entries := transformResponse(response, requestArgs.Descriptors, actionDefinitions)

	ios.StopProgressIndicator()

	if o.exporter != nil {
		return o.exporter.Write(ios, entries)
	}

	if len(entries) == 0 {
		fmt.Fprintln(ios.Out, "No permissions found.")
		return nil
	}

	table, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	if requestArgs.Descriptors != nil {
		table.AddColumns("Token", "Allow", "Deny", "Effective Allow", "Effective Deny", "Inherits")
	} else {
		table.AddColumns("Token", "Descriptor", "Allow", "Deny", "Effective Allow", "Effective Deny", "Inherits")
	}
	table.EndRow()

	for _, entry := range entries {
		table.AddField(types.GetValue(entry.Token, ""))
		if requestArgs.Descriptors == nil {
			table.AddField(types.GetValue(entry.Descriptor, ""))
		}
		table.AddField(types.GetValue(entry.Allow, ""))
		table.AddField(types.GetValue(entry.Deny, ""))
		table.AddField(types.GetValue(entry.EffectiveAllow, ""))
		table.AddField(types.GetValue(entry.EffectiveDeny, ""))
		if types.GetValue(entry.InheritPermissions, false) {
			table.AddField("Yes")
		} else {
			table.AddField("No")
		}
		table.EndRow()
	}

	return table.Render()
}

func parseSubjectTarget(ctx util.CmdContext, input string) (*util.Scope, string, bool, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		scope, err := util.ParseScope(ctx, "")
		if err != nil {
			return nil, "", false, err
		}
		return scope, "", false, nil
	}

	segments := strings.Split(trimmed, "/")
	if len(segments) == 0 || len(segments) > 3 {
		return nil, "", false, util.FlagErrorf("invalid target %q", input)
	}

	orgPart := strings.TrimSpace(segments[0])
	if orgPart == "" {
		return nil, "", false, util.FlagErrorf("organization must not be empty")
	}

	switch len(segments) {
	case 1:
		scope, err := util.ParseScope(ctx, orgPart)
		if err != nil {
			return nil, "", false, err
		}
		return scope, "", false, nil
	case 2:
		subject := strings.TrimSpace(segments[1])
		if subject == "" {
			return nil, "", false, util.FlagErrorf("subject must not be empty")
		}
		scope, err := util.ParseScope(ctx, orgPart)
		if err != nil {
			return nil, "", false, err
		}
		return scope, subject, true, nil
	case 3:
		project := strings.TrimSpace(segments[1])
		subject := strings.TrimSpace(segments[2])
		if project == "" {
			return nil, "", false, util.FlagErrorf("project must not be empty")
		}
		if subject == "" {
			return nil, "", false, util.FlagErrorf("subject must not be empty")
		}
		scopeInput := fmt.Sprintf("%s/%s", orgPart, project)
		scope, err := util.ParseScope(ctx, scopeInput)
		if err != nil {
			return nil, "", false, err
		}
		return scope, subject, true, nil
	default:
		return nil, "", false, util.FlagErrorf("invalid target %q", input)
	}
}

func transformResponse(response *[]security.AccessControlList, descriptor *string, actions []security.ActionDefinition) []permissionEntry {
	if response == nil {
		return nil
	}

	results := make([]permissionEntry, 0)
	for _, acl := range *response {
		if acl.AcesDictionary == nil {
			continue
		}

		if descriptor != nil && strings.TrimSpace(*descriptor) != "" {
			ace, ok := (*acl.AcesDictionary)[*descriptor]
			if !ok {
				zap.L().Sugar().Debugf("Skipping ACL for token=%q without matching descriptor", types.GetValue(acl.Token, ""))
				continue
			}
			entry := buildPermissionEntry(acl, ace, descriptor, actions)
			results = append(results, entry)
			continue
		}

		for key, ace := range *acl.AcesDictionary {
			desc := key
			if ace.Descriptor != nil && strings.TrimSpace(*ace.Descriptor) != "" {
				desc = strings.TrimSpace(*ace.Descriptor)
			}
			localDescriptor := desc
			entry := buildPermissionEntry(acl, ace, &localDescriptor, actions)
			results = append(results, entry)
		}
	}

	return results
}

func buildPermissionEntry(acl security.AccessControlList, ace security.AccessControlEntry, descriptor *string, actions []security.ActionDefinition) permissionEntry {
	entry := permissionEntry{
		Token:              acl.Token,
		Descriptor:         descriptor,
		InheritPermissions: acl.InheritPermissions,
		Allow:              permissionShared.DescribeBitmask(actions, ace.Allow),
		Deny:               permissionShared.DescribeBitmask(actions, ace.Deny),
		EffectiveAllow:     permissionShared.DescribeBitmask(actions, extendedValue(ace.ExtendedInfo, func(info *security.AceExtendedInformation) *int { return info.EffectiveAllow })),
		EffectiveDeny:      permissionShared.DescribeBitmask(actions, extendedValue(ace.ExtendedInfo, func(info *security.AceExtendedInformation) *int { return info.EffectiveDeny })),
		InheritedAllow:     permissionShared.DescribeBitmask(actions, extendedValue(ace.ExtendedInfo, func(info *security.AceExtendedInformation) *int { return info.InheritedAllow })),
		InheritedDeny:      permissionShared.DescribeBitmask(actions, extendedValue(ace.ExtendedInfo, func(info *security.AceExtendedInformation) *int { return info.InheritedDeny })),
	}
	return entry
}

func extendedValue(info *security.AceExtendedInformation, accessor func(*security.AceExtendedInformation) *int) *int {
	if info == nil {
		return nil
	}
	return accessor(info)
}
