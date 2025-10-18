package list

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

type opts struct {
	scope        string
	exporter     util.Exporter
	relationship string
}

type membershipListResult struct {
	Descriptor       *string `json:"descriptor,omitempty"`
	DisplayName      *string `json:"displayName,omitempty"`
	URL              *string `json:"url,omitempty"`
	LegacyDescriptor *string `json:"legacyDescriptor,omitempty"`
	Origin           *string `json:"origin,omitempty"`
	OriginID         *string `json:"originId,omitempty"`
	SubjectKind      *string `json:"subjectKind,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}
	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP",
		Short: "List the members of an Azure DevOps security group.",
		Args:  cobra.ExactArgs(1),
		Aliases: []string{
			"ls",
			"l",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.scope = args[0]
			return runList(ctx, o)
		},
	}
	util.AddJSONFlags(cmd, &o.exporter, []string{"descriptor", "displayName", "url", "legacyDescriptor", "origin", "originId", "subjectKind"})
	util.StringEnumFlag(cmd, &o.relationship, "relationship", "r", "members", []string{"members", "memberof"}, "Relationship type: members or memberof")
	return cmd
}

func runList(ctx util.CmdContext, o *opts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	var organization, project, group string
	parts := strings.Split(o.scope, "/")
	switch len(parts) {
	case 2:
		organization, group = parts[0], parts[1]
	case 3:
		organization, project, group = parts[0], parts[1], parts[2]
	default:
		return util.FlagErrorf("invalid scope format: %q", o.scope)
	}

	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), organization)
	if err != nil {
		return err
	}

	var projectDescriptor *string
	var projectID *string

	if project != "" {
		zap.L().Sugar().Debug("Fetching Core client for project scope")
		coreClient, err := ctx.ClientFactory().Core(ctx.Context(), organization)
		if err != nil {
			return fmt.Errorf("failed to get core client: %w", err)
		}
		p, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
			ProjectId: &project,
		})
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
		}
		projectID = types.ToPtr(p.Id.String())

		zap.L().Sugar().Debugf("Fetched project: %q (%s)", *p.Name, p.Id.String())

		descriptor, err := graphClient.GetDescriptor(ctx.Context(), graph.GetDescriptorArgs{
			StorageKey: p.Id,
		})
		if err != nil {
			return fmt.Errorf("failed to get project descriptor: %w", err)
		}
		zap.L().Sugar().Debugf("Project descriptor: %s", types.GetValue(descriptor.Value, ""))

		projectDescriptor = descriptor.Value
	}

	zap.L().Sugar().Debug("projectDescriptor: ", types.GetValue(projectDescriptor, "nil"))
	zap.L().Sugar().Debugf("projectID: ", types.GetValue(projectID, "nil"))

	identityClient, err := ctx.ClientFactory().Identity(ctx.Context(), organization)
	if err != nil {
		return err
	}

	result, err := identityClient.ReadIdentities(ctx.Context(), identity.ReadIdentitiesArgs{
		SearchFilter:    types.ToPtr("LocalGroupName"),
		FilterValue:     &group,
		QueryMembership: &identity.QueryMembershipValues.None,
	})
	if err != nil {
		return err
	}
	if result == nil || len(*result) == 0 {
		return fmt.Errorf("group %q not found", group)
	}

	var groupDescriptor *string
	switch len(*result) {
	case 1:
		groupDescriptor = (*result)[0].SubjectDescriptor
	default:
		if projectID != nil {
			for _, g := range *result {
				props := g.Properties.(map[string]any)
				propVal := props["LocalScopeId"].(map[string]any)
				scope := propVal["$value"].(string)
				zap.L().Sugar().Debugf("Found group %q with scope %q", types.GetValue(g.ProviderDisplayName, ""), scope)
				if strings.EqualFold(scope, *projectID) {
					groupDescriptor = g.SubjectDescriptor
					break
				}
			}
		}
		if groupDescriptor == nil {
			return fmt.Errorf("multiple groups found with name %q", group)
		}
	}

	dir := graph.GraphTraversalDirectionValues.Down
	if o.relationship == "memberof" {
		dir = graph.GraphTraversalDirectionValues.Up
	}
	memberships, err := graphClient.ListMemberships(ctx.Context(), graph.ListMembershipsArgs{
		SubjectDescriptor: groupDescriptor,
		Direction:         &dir,
	})
	if err != nil {
		return err
	}
	if memberships == nil || len(*memberships) == 0 {
		ios.StopProgressIndicator()

		fmt.Fprintf(ios.Out, "No members found for group %q\n", o.scope)
		return nil
	}

	var keys []graph.GraphSubjectLookupKey
	for _, m := range *memberships {
		if o.relationship == "memberof" {
			keys = append(keys, graph.GraphSubjectLookupKey{Descriptor: m.ContainerDescriptor})
		} else {
			keys = append(keys, graph.GraphSubjectLookupKey{Descriptor: m.MemberDescriptor})
		}
	}
	lookup := graph.GraphSubjectLookup{LookupKeys: &keys}
	subjects, err := graphClient.LookupSubjects(ctx.Context(), graph.LookupSubjectsArgs{SubjectLookup: &lookup})
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if o.exporter != nil {
		results := []membershipListResult{}
		for _, s := range *subjects {
			results = append(results, membershipListResult{
				Descriptor:       s.Descriptor,
				DisplayName:      s.DisplayName,
				URL:              s.Url,
				LegacyDescriptor: s.LegacyDescriptor,
				Origin:           s.Origin,
				OriginID:         s.OriginId,
				SubjectKind:      s.SubjectKind,
			})
		}
		return o.exporter.Write(ios, results)
	}

	tp, err := ctx.Printer("table")
	if err != nil {
		return err
	}
	tp.AddColumns("DiplayName", "Descriptor", "SubjectType")

	for _, s := range *subjects {
		tp.AddField(types.GetValue(s.DisplayName, ""))
		tp.AddField(types.GetValue(s.Descriptor, ""))
		tp.AddField(types.GetValue(s.SubjectKind, ""))
		tp.EndRow()
	}
	return tp.Render()
}
