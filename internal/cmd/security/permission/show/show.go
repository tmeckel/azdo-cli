package show

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/text"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	rawTarget   string
	namespaceID string
	token       string
	exporter    util.Exporter
}

type permissionEntry struct {
	Token              *string   `json:"token,omitempty"`
	Descriptor         *string   `json:"descriptor,omitempty"`
	InheritPermissions *bool     `json:"inheritPermissions,omitempty"`
	Allow              *[]string `json:"allow,omitempty"`
	Deny               *[]string `json:"deny,omitempty"`
	EffectiveAllow     *[]string `json:"effectiveAllow,omitempty"`
	EffectiveDeny      *[]string `json:"effectiveDeny,omitempty"`
	InheritedAllow     *[]string `json:"inheritedAllow,omitempty"`
	InheritedDeny      *[]string `json:"inheritedDeny,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "show <TARGET>",
		Short: "Show permissions for a user or group.",
		Long: heredoc.Doc(`
			Show the explicit and effective permissions for a user or group on a specific securable resource (identified by a token).

			Accepted TARGET formats:
			  - ORGANIZATION/SUBJECT           → show permissions for the specified subject
			  - ORGANIZATION/PROJECT/SUBJECT   → show permissions for the subject scoped to the project
		`),
		Example: heredoc.Doc(`
			# Show permissions for a user
			azdo security permission show fabrikam/contoso@example.com --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866 --token /projects/a6880f5a-60e1-4103-89f2-69533e4d139f

			# Show permissions for a project-scoped group
			azdo security permission show fabrikam/ProjectAlpha/vssgp.Uy0xLTktMTIzNDU2 --namespace-id 33344d9c-fc72-4d6f-aba5-fa317101a7e9 --token /
		`),
		Aliases: []string{
			"s",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.rawTarget = args[0]
			return runCommand(ctx, o)
		},
	}

	cmd.Flags().StringVarP(&o.namespaceID, "namespace-id", "n", "", "ID of the security namespace to query (required).")
	cmd.Flags().StringVar(&o.token, "token", "", "Security token to query (required).")
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

	namespaceUUID, err := uuid.Parse(strings.TrimSpace(o.namespaceID))
	if err != nil {
		return util.FlagErrorf("invalid namespace id %q: %v", o.namespaceID, err)
	}

	scope, err := shared.ParseSubjectTarget(ctx, o.rawTarget)
	if err != nil {
		return err
	}

	zap.L().Sugar().Debugf("Resolved scope organization=%q project=%q subject=%q", scope.Organization, scope.Project, scope.Subject)

	hasSubject := scope.Subject != ""

	if !hasSubject {
		return util.FlagErrorf("a subject is required")
	}

	if scope.Project != "" {
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
	actionDefinitions := shared.ExtractNamespaceActions(namespaceDetails)

	extensionsClient, err := ctx.ClientFactory().Extensions(ctx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	member, err := extensionsClient.ResolveMemberDescriptor(ctx.Context(), scope.Subject)
	if err != nil {
		return fmt.Errorf("failed to resolve subject %q: %w", scope.Subject, err)
	}

	// Resolve the identity descriptor form used by ACLs via the Identity API.
	identityClient, ierr := ctx.ClientFactory().Identity(ctx.Context(), scope.Organization)
	if ierr != nil {
		return fmt.Errorf("failed to create identity client: %w", ierr)
	}
	subj := strings.TrimSpace(types.GetValue(member.Descriptor, ""))
	if subj == "" {
		return fmt.Errorf("resolved subject descriptor is empty")
	}
	sd := subj
	idents, err := identityClient.ReadIdentities(ctx.Context(), identity.ReadIdentitiesArgs{
		SubjectDescriptors: &sd,
	})
	if err != nil {
		return fmt.Errorf("failed to read identity information for %q: %w", subj, err)
	}
	if idents == nil || len(*idents) == 0 {
		return fmt.Errorf("no identity returned for descriptor %q", subj)
	}
	descriptor := strings.TrimSpace(types.GetValue((*idents)[0].Descriptor, ""))
	if descriptor == "" {
		return fmt.Errorf("identity for %q did not contain a descriptor", subj)
	}
	zap.L().Sugar().Debugf("Resolved subject descriptor (acl)=%q", descriptor)

	requestArgs := security.QueryAccessControlListsArgs{
		SecurityNamespaceId: &namespaceUUID,
		Token:               types.ToPtr(strings.TrimSpace(o.token)),
		Descriptors:         types.ToPtr(descriptor),
		IncludeExtendedInfo: types.ToPtr(true),
		Recurse:             types.ToPtr(true),
	}

	zap.L().Sugar().Debugf("Querying ACLs (token=%q subjectFilter=%v)", o.token, descriptor)

	response, err := securityClient.QueryAccessControlLists(ctx.Context(), requestArgs)
	if err != nil {
		return fmt.Errorf("failed to query access control lists: %w", err)
	}

	entry := transformResponse(response, &descriptor, actionDefinitions)

	ios.StopProgressIndicator()

	if o.exporter != nil {
		return o.exporter.Write(ios, entry)
	}

	if entry == nil {
		fmt.Fprintln(ios.Out, "No permissions found.")
		return nil
	}

	table, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	table.AddColumns("Token", "Subject Descriptor", "Inherit Permissions", "Allow", "Deny", "Effective Allow", "Effective Deny", "Inherited Allow", "Inherited Deny")
	table.EndRow()

	table.AddField(types.GetValue(entry.Token, "-"))
	table.AddField(types.GetValue(entry.Descriptor, "-"))
	if types.GetValue(entry.InheritPermissions, false) {
		table.AddField("Yes")
	} else {
		table.AddField("No")
	}
	table.AddField(text.NewSliceFormatter(types.GetValue(entry.Allow, []string{"None"})).WithSort(true).String())
	table.AddField(text.NewSliceFormatter(types.GetValue(entry.Deny, []string{"None"})).WithSort(true).String())
	table.AddField(text.NewSliceFormatter(types.GetValue(entry.EffectiveAllow, []string{"None"})).WithSort(true).String())
	table.AddField(text.NewSliceFormatter(types.GetValue(entry.EffectiveDeny, []string{"None"})).WithSort(true).String())
	table.AddField(text.NewSliceFormatter(types.GetValue(entry.InheritedAllow, []string{"None"})).WithSort(true).String())
	table.AddField(text.NewSliceFormatter(types.GetValue(entry.InheritedDeny, []string{"None"})).WithSort(true).String())
	table.EndRow()
	return table.Render()
}

func transformResponse(response *[]security.AccessControlList, descriptor *string, actions []security.ActionDefinition) *permissionEntry {
	if response == nil || len(*response) == 0 {
		return nil
	}

	for _, acl := range *response {
		if acl.AcesDictionary == nil {
			continue
		}

		if ace, ok := (*acl.AcesDictionary)[*descriptor]; ok {
			entry := buildPermissionEntry(acl, ace, descriptor, actions)
			return &entry
		}
	}

	return nil
}

func buildPermissionEntry(acl security.AccessControlList, ace security.AccessControlEntry, descriptor *string, actions []security.ActionDefinition) permissionEntry {
	return permissionEntry{
		Token:              acl.Token,
		Descriptor:         descriptor,
		InheritPermissions: acl.InheritPermissions,
		Allow:              shared.DescribeBitmaskArray(actions, ace.Allow),
		Deny:               shared.DescribeBitmaskArray(actions, ace.Deny),
		EffectiveAllow:     shared.DescribeBitmaskArray(actions, extendedValue(ace.ExtendedInfo, func(info *security.AceExtendedInformation) *int { return info.EffectiveAllow })),
		EffectiveDeny:      shared.DescribeBitmaskArray(actions, extendedValue(ace.ExtendedInfo, func(info *security.AceExtendedInformation) *int { return info.EffectiveDeny })),
		InheritedAllow:     shared.DescribeBitmaskArray(actions, extendedValue(ace.ExtendedInfo, func(info *security.AceExtendedInformation) *int { return info.InheritedAllow })),
		InheritedDeny:      shared.DescribeBitmaskArray(actions, extendedValue(ace.ExtendedInfo, func(info *security.AceExtendedInformation) *int { return info.InheritedDeny })),
	}
}

func extendedValue(info *security.AceExtendedInformation, accessor func(*security.AceExtendedInformation) *int) *int {
	if info == nil {
		return nil
	}
	return accessor(info)
}
