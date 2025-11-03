package list

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
	recurse     bool
	exporter    util.Exporter
}

type permissionEntry struct {
	Token              *string `json:"token,omitempty"`
	Descriptor         *string `json:"descriptor,omitempty"`
	InheritPermissions *bool   `json:"inheritPermissions,omitempty"`
	Allow              *int    `json:"allow,omitempty"`
	Deny               *int    `json:"deny,omitempty"`
	EffectiveAllow     *int    `json:"effectiveAllow,omitempty"`
	EffectiveDeny      *int    `json:"effectiveDeny,omitempty"`
	InheritedAllow     *int    `json:"inheritedAllow,omitempty"`
	InheritedDeny      *int    `json:"inheritedDeny,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "list [TARGET]",
		Short: "List security ACEs for a namespace, optionally filtered by subject.",
		Long: heredoc.Doc(`
			List security access control entries (ACEs) for an Azure DevOps security namespace.

			Accepted TARGET formats:
			  - (empty)                        → use the default organization
			  - ORGANIZATION                   → list all ACEs for the namespace in the organization
			  - ORGANIZATION/SUBJECT           → list ACEs for the specified subject
			  - ORGANIZATION/PROJECT/SUBJECT   → list ACEs for the subject scoped to the project
		`),
		Example: heredoc.Doc(`
			# List all ACEs for a namespace using the default organization
			azdo security permission list --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866

			# List all ACEs for a namespace in an explicit organization
			azdo security permission list fabrikam --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866

			# List all tokens for a specific user
			azdo security permission list fabrikam/contoso@example.com --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866

			# List ACEs for a project-scoped group
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
	cmd.Flags().BoolVar(&o.recurse, "recurse", false, "Include child ACEs for the specified token when supported.")
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
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if strings.TrimSpace(o.namespaceID) == "" {
		return util.FlagErrorf("--namespace-id is required")
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

	zap.L().Sugar().Debugf("Resolved scope organization=%q project=%q subject=%q", scope.Organization, scope.Project, scope.Subject)

	if hasSubject && scope.Project != "" {
		if _, _, err := util.ResolveScopeDescriptor(ctx, scope.Organization, scope.Project); err != nil {
			return err
		}
	}

	securityClient, err := ctx.ClientFactory().Security(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create security client: %w", err)
	}

	requestArgs := security.QueryAccessControlListsArgs{
		SecurityNamespaceId: &namespaceUUID,
		IncludeExtendedInfo: types.ToPtr(true),
	}

	if hasSubject {
		extensionsClient, err := ctx.ClientFactory().Extensions(ctx.Context(), scope.Organization)
		if err != nil {
			return err
		}

		member, err := extensionsClient.ResolveIdentity(ctx.Context(), scope.Subject)
		if err != nil {
			return fmt.Errorf("failed to resolve subject %q: %w", scope.Subject, err)
		}

		subj := strings.TrimSpace(types.GetValue(member.Descriptor, ""))
		if subj == "" {
			return fmt.Errorf("resolved subject descriptor is empty")
		}

		requestArgs.Descriptors = &subj
	}

	if strings.TrimSpace(o.token) != "" {
		token := strings.TrimSpace(o.token)
		requestArgs.Token = &token
	}
	if o.recurse {
		requestArgs.Recurse = types.ToPtr(true)
	}

	zap.L().Sugar().Debugf("Querying ACEs (token=%q recurse=%v subjectFilter=%q)", o.token, o.recurse, scope.Subject)

	response, err := securityClient.QueryAccessControlLists(ctx.Context(), requestArgs)
	if err != nil {
		return fmt.Errorf("failed to query access control lists: %w", err)
	}

	entries := transformResponse(response)

	ios.StopProgressIndicator()

	if o.exporter != nil {
		return o.exporter.Write(ios, entries)
	}

	if len(entries) == 0 {
		fmt.Fprintln(ios.Out, "No permissions found.")
		return nil
	}

	table, err := ctx.Printer("table")
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
		table.AddField(formatPermissionValue(entry.Allow))
		table.AddField(formatPermissionValue(entry.Deny))
		table.AddField(formatPermissionValue(entry.EffectiveAllow))
		table.AddField(formatPermissionValue(entry.EffectiveDeny))
		if types.GetValue(entry.InheritPermissions, false) {
			table.AddField("Yes")
		} else {
			table.AddField("No")
		}
		table.EndRow()
	}

	return table.Render()
}

func transformResponse(response *[]security.AccessControlList) []permissionEntry {
	if response == nil {
		return nil
	}

	results := make([]permissionEntry, 0)
	for _, acl := range *response {
		if acl.AcesDictionary == nil {
			continue
		}

		for key, ace := range *acl.AcesDictionary {
			desc := key
			if ace.Descriptor != nil && strings.TrimSpace(*ace.Descriptor) != "" {
				desc = strings.TrimSpace(*ace.Descriptor)
			}
			entry := buildPermissionEntry(acl, ace, &desc)
			results = append(results, entry)
		}
	}

	return results
}

func buildPermissionEntry(acl security.AccessControlList, ace security.AccessControlEntry, descriptor *string) permissionEntry {
	entry := permissionEntry{
		Token:              acl.Token,
		Descriptor:         descriptor,
		InheritPermissions: acl.InheritPermissions,
		Allow:              ace.Allow,
		Deny:               ace.Deny,
		EffectiveAllow:     ace.ExtendedInfo.EffectiveAllow,
		EffectiveDeny:      ace.ExtendedInfo.EffectiveDeny,
		InheritedAllow:     ace.ExtendedInfo.InheritedAllow,
		InheritedDeny:      ace.ExtendedInfo.InheritedDeny,
	}
	return entry
}

func formatPermissionValue(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("0x%X", *value)
}
