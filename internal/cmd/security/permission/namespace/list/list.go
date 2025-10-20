package list

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	target    string
	localOnly bool
	exporter  util.Exporter
}

type namespaceEntry struct {
	NamespaceID       string `json:"namespaceId"`
	Name              string `json:"name,omitempty"`
	DisplayName       string `json:"displayName,omitempty"`
	DataspaceCategory string `json:"dataspaceCategory,omitempty"`
	IsRemotable       *bool  `json:"isRemotable,omitempty"`
	ExtensionType     string `json:"extensionType,omitempty"`
	ElementLength     *int   `json:"elementLength,omitempty"`
	SeparatorValue    string `json:"separatorValue,omitempty"`
	WritePermission   string `json:"writePermission,omitempty"`
	ReadPermission    string `json:"readPermission,omitempty"`
	UseTranslator     *bool  `json:"useTokenTranslator,omitempty"`
	SystemBitMask     string `json:"systemBitMask,omitempty"`
	StructureValue    *int   `json:"structureValue,omitempty"`
	ActionsCount      *int   `json:"actionsCount,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION]",
		Short: "List security permission namespaces.",
		Long: heredoc.Doc(`
			List all security permission namespaces available in an Azure DevOps organization.

			Namespaces define the scope and structure for security permissions on various resources.
		`),
		Args: cobra.MaximumNArgs(1),
		Aliases: []string{
			"ls",
			"l",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.target = args[0]
			}
			return runCommand(ctx, o)
		},
	}

	cmd.Flags().BoolVar(&o.localOnly, "local-only", false, "Only include namespaces defined locally within the organization.")
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

	scope, err := util.ParseScope(ctx, o.target)
	if err != nil {
		return err
	}
	if scope.Project != "" {
		return util.FlagErrorf("project scope is not supported for this command")
	}

	organization := scope.Organization
	zap.L().Sugar().Debugf("Listing security namespaces for organization %q (localOnly=%v)", organization, o.localOnly)

	securityClient, err := ctx.ClientFactory().Security(ctx.Context(), organization)
	if err != nil {
		return fmt.Errorf("failed to create security client: %w", err)
	}

	args := security.QuerySecurityNamespacesArgs{}
	if o.localOnly {
		args.LocalOnly = types.ToPtr(true)
	}

	response, err := securityClient.QuerySecurityNamespaces(ctx.Context(), args)
	if err != nil {
		return fmt.Errorf("failed to query security namespaces: %w", err)
	}

	var namespaces []security.SecurityNamespaceDescription
	if response != nil {
		namespaces = *response
	}
	zap.L().Sugar().Debugf("Fetched %d namespaces", len(namespaces))

	ios.StopProgressIndicator()

	if o.exporter != nil {
		results := make([]namespaceEntry, 0, len(namespaces))
		for _, ns := range namespaces {
			results = append(results, transformNamespace(ns))
		}
		return o.exporter.Write(ios, results)
	}

	if len(namespaces) == 0 {
		fmt.Fprintln(ios.Out, "No security namespaces found.")
		return nil
	}

	table, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	table.AddColumns("Namespace ID", "Name", "Display Name", "Dataspace", "Remotable")
	table.EndRow()

	for _, ns := range namespaces {
		entry := transformNamespace(ns)
		table.AddField(entry.NamespaceID)
		table.AddField(entry.Name)
		table.AddField(entry.DisplayName)
		table.AddField(entry.DataspaceCategory)
		if entry.IsRemotable != nil && *entry.IsRemotable {
			table.AddField("Yes")
		} else {
			table.AddField("No")
		}
		table.EndRow()
	}

	return table.Render()
}

func transformNamespace(ns security.SecurityNamespaceDescription) namespaceEntry {
	var namespaceID string
	if ns.NamespaceId != nil {
		namespaceID = ns.NamespaceId.String()
	}

	var systemBitMask string
	if ns.SystemBitMask != nil {
		systemBitMask = fmt.Sprintf("0x%X", *ns.SystemBitMask)
	}

	var writePermission string
	if ns.WritePermission != nil {
		writePermission = fmt.Sprintf("0x%X", *ns.WritePermission)
	}

	var readPermission string
	if ns.ReadPermission != nil {
		readPermission = fmt.Sprintf("0x%X", *ns.ReadPermission)
	}

	entry := namespaceEntry{
		NamespaceID:       namespaceID,
		Name:              types.GetValue(ns.Name, ""),
		DisplayName:       types.GetValue(ns.DisplayName, ""),
		DataspaceCategory: types.GetValue(ns.DataspaceCategory, ""),
		IsRemotable:       ns.IsRemotable,
		ExtensionType:     types.GetValue(ns.ExtensionType, ""),
		ElementLength:     ns.ElementLength,
		SeparatorValue:    types.GetValue(ns.SeparatorValue, ""),
		WritePermission:   writePermission,
		ReadPermission:    readPermission,
		UseTranslator:     ns.UseTokenTranslator,
		SystemBitMask:     systemBitMask,
		StructureValue:    ns.StructureValue,
	}

	if ns.Actions != nil {
		count := len(*ns.Actions)
		entry.ActionsCount = types.ToPtr(count)
	}

	return entry
}
