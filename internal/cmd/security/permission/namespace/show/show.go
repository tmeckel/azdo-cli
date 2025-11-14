package show

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	rawTarget string
	exporter  util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]NAMESPACE",
		Short: "Show details for a security permission namespace.",
		Long: heredoc.Doc(`
			Show the full details of a security permission namespace, including the actions it defines.

			The namespace can be specified by its GUID or by its name. When using a name, the command performs
			a case-insensitive match against both the namespace's name and display name.
		`),
		Example: heredoc.Doc(`
			# Show a namespace by ID using the default organization
			azdo security permission namespace show 52d39943-cb85-4d7f-8fa8-c6baac873819

			# Show a namespace by name using an explicit organization
			azdo security permission namespace show myorg/Project Collection

			# Display selected fields from the namespace as JSON
			azdo security permission namespace show myorg/Build --json namespaceId,name,actions
		`),
		Args: cobra.ExactArgs(1),
		Aliases: []string{
			"s",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.rawTarget = args[0]
			return runCommand(ctx, o)
		},
	}

	util.AddJSONFlags(cmd, &o.exporter, []string{
		"namespaceId",
		"name",
		"displayName",
		"dataspaceCategory",
		"isRemotable",
		"extensionType",
		"elementLength",
		"separatorValue",
		"writePermission",
		"readPermission",
		"useTokenTranslator",
		"systemBitMask",
		"structureValue",
		"actionsCount",
		"actions",
	})

	return cmd
}

func runCommand(ctx util.CmdContext, o *opts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	scope, identifier, err := parseNamespaceTarget(ctx, o.rawTarget)
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	securityClient, err := ctx.ClientFactory().Security(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create security client: %w", err)
	}

	var resolved *security.SecurityNamespaceDescription
	if namespaceID, parseErr := uuid.Parse(identifier); parseErr == nil {
		zap.L().Sugar().Debugf("Retrieving namespace %q by ID for organization %q", namespaceID, scope.Organization)
		args := security.QuerySecurityNamespacesArgs{
			SecurityNamespaceId: types.ToPtr(namespaceID),
		}
		response, err := securityClient.QuerySecurityNamespaces(ctx.Context(), args)
		if err != nil {
			return fmt.Errorf("failed to query security namespace: %w", err)
		}
		if response != nil && len(*response) > 0 {
			entry := (*response)[0]
			resolved = &entry
		}
	} else {
		zap.L().Sugar().Debugf("Resolving namespace %q by name for organization %q", identifier, scope.Organization)
		response, err := securityClient.QuerySecurityNamespaces(ctx.Context(), security.QuerySecurityNamespacesArgs{})
		if err != nil {
			return fmt.Errorf("failed to query security namespaces: %w", err)
		}
		if response != nil {
			matches := make([]security.SecurityNamespaceDescription, 0)
			for _, ns := range *response {
				if namespaceMatches(ns, identifier) {
					matches = append(matches, ns)
				}
			}
			switch len(matches) {
			case 0:
				resolved = nil
			case 1:
				resolved = &matches[0]
			default:
				return util.FlagErrorf("multiple namespaces matched %q; specify the namespace ID instead", identifier)
			}
		}
	}

	if resolved == nil {
		ios.StopProgressIndicator()
		fmt.Fprintln(ios.Out, "Namespace not found.")
		return nil
	}

	ios.StopProgressIndicator()

	entry := shared.TransformNamespace(*resolved)

	if o.exporter != nil {
		return o.exporter.Write(ios, entry)
	}

	if err := renderNamespaceSummary(ctx, entry); err != nil {
		return err
	}

	fmt.Fprintln(ios.Out)

	if len(entry.Actions) == 0 {
		fmt.Fprintln(ios.Out, "No actions are defined for this namespace.")
		return nil
	}

	actionPrinter, err := ctx.Printer("list")
	if err != nil {
		return err
	}
	actionPrinter.AddColumns("Bit", "Hex", "Name", "Display Name")
	actionPrinter.EndRow()
	for _, action := range entry.Actions {
		if action.Bit != nil {
			actionPrinter.AddField(fmt.Sprintf("%d", *action.Bit))
		} else {
			actionPrinter.AddField("-")
		}
		actionPrinter.AddField(valueOrPlaceholder(action.BitHex))
		actionPrinter.AddField(valueOrPlaceholder(action.Name))
		actionPrinter.AddField(valueOrPlaceholder(action.DisplayName))
		actionPrinter.EndRow()
	}
	return actionPrinter.Render()
}

func parseNamespaceTarget(ctx util.CmdContext, input string) (*util.Scope, string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, "", util.FlagErrorf("namespace identifier is required")
	}

	if strings.Contains(trimmed, "/") {
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, "", util.FlagErrorf("invalid namespace target: %s", input)
		}

		scope, err := util.ParseScope(ctx, strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, "", err
		}
		return scope, strings.TrimSpace(parts[1]), nil
	}

	organization, err := util.ParseOrganizationArg(ctx, "")
	if err != nil {
		return nil, "", err
	}
	return &util.Scope{Organization: organization}, trimmed, nil
}

func namespaceMatches(ns security.SecurityNamespaceDescription, identifier string) bool {
	if ns.Name != nil && strings.EqualFold(*ns.Name, identifier) {
		return true
	}
	if ns.DisplayName != nil && strings.EqualFold(*ns.DisplayName, identifier) {
		return true
	}
	return false
}

func renderNamespaceSummary(ctx util.CmdContext, entry shared.NamespaceEntry) error {
	printer, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	printer.AddColumns(
		"Namespace ID",
		"Name",
		"Display Name",
		"Dataspace",
		"Remotable",
		"System Bit Mask",
		"Write Permission",
		"Read Permission",
		"Structure",
		"Use Translator",
	)
	printer.EndRow()
	printer.AddField(valueOrPlaceholder(entry.NamespaceID))
	printer.AddField(valueOrPlaceholder(entry.Name))
	printer.AddField(valueOrPlaceholder(entry.DisplayName))
	printer.AddField(valueOrPlaceholder(entry.DataspaceCategory))
	printer.AddField(boolToYesNo(entry.IsRemotable))
	printer.AddField(valueOrPlaceholder(entry.SystemBitMask))
	printer.AddField(valueOrPlaceholder(entry.WritePermission))
	printer.AddField(valueOrPlaceholder(entry.ReadPermission))
	if entry.StructureValue != nil {
		printer.AddField(fmt.Sprintf("%d", *entry.StructureValue))
	} else {
		printer.AddField("-")
	}
	printer.AddField(boolToYesNo(entry.UseTranslator))
	printer.EndRow()

	return printer.Render()
}

func valueOrPlaceholder(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func boolToYesNo(value *bool) string {
	if value == nil {
		return "-"
	}
	if *value {
		return "Yes"
	}
	return "No"
}
