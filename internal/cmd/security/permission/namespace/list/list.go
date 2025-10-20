package list

import (
	"fmt"
	"slices"
	"strings"

	"go.uber.org/zap"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/permission/namespace/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	target    string
	localOnly bool
	exporter  util.Exporter
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
		results := make([]shared.NamespaceEntry, 0, len(namespaces))
		for _, ns := range namespaces {
			results = append(results, shared.TransformNamespace(ns))
		}
		return o.exporter.Write(ios, results)
	}

	if len(namespaces) == 0 {
		fmt.Fprintln(ios.Out, "No security namespaces found.")
		return nil
	}

	table, err := ctx.Printer("table")
	if err != nil {
		return err
	}

	table.AddColumns("Namespace ID", "Name", "Display Name", "Dataspace", "Remotable")
	table.EndRow()

	slices.SortFunc(namespaces, func(a, b security.SecurityNamespaceDescription) int {
		return strings.Compare(strings.ToLower(*a.Name), strings.ToLower(*b.Name))
	})

	for _, ns := range namespaces {
		entry := shared.TransformNamespace(ns)
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
