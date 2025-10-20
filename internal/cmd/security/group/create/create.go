package create

import (
	"fmt"
	"slices"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type createOpts struct {
	name        string
	description string
	email       string
	originID    string
	groups      []string
	scope       string
	exporter    util.Exporter
}

type groupCreateResult struct {
	Descriptor    *string `json:"descriptor,omitempty"`
	PrincipalName *string `json:"principalName,omitempty"`
	DisplayName   *string `json:"displayName,omitempty"`
	Description   *string `json:"description,omitempty"`
	MailAddress   *string `json:"mailAddress,omitempty"`
	Origin        *string `json:"origin,omitempty"`
	OriginID      *string `json:"originId,omitempty"`
	URL           *string `json:"url,omitempty"`
	Domain        *string `json:"domain,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &createOpts{}

	cmd := &cobra.Command{
		Use:   "create [ORGANIZATION|ORGANIZATION/PROJECT]",
		Short: "Create a security group",
		Long: heredoc.Doc(`
			Create a security group in an Azure DevOps organization or project.

			Security groups can be created by name, email, or origin ID. Exactly one of these must be specified.
		`),
		Args: cobra.MaximumNArgs(1),
		Aliases: []string{
			"add",
			"new",
			"c",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.scope = args[0]
			}
			return runCreate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Name of the new security group.")
	cmd.Flags().StringVar(&opts.description, "description", "", "Description of the new security group.")
	cmd.Flags().StringVar(&opts.email, "email", "", "Create a security group using an existing AAD group's email address.")
	cmd.Flags().StringVar(&opts.originID, "origin-id", "", "Create a security group using an existing AAD group's origin ID.")
	cmd.Flags().StringSliceVar(&opts.groups, "groups", nil, "A comma-separated list of group descriptors to add the new group to.")
	util.AddJSONFlags(cmd, &opts.exporter, []string{"descriptor", "principalName", "displayName", "description", "mailAddress", "origin", "originId", "url", "domain"})

	return cmd
}

func runCreate(ctx util.CmdContext, opts *createOpts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	cfg, err := ctx.Config()
	if err != nil {
		return err
	}

	scope, err := util.ParseScope(ctx, opts.scope)
	if err != nil {
		return err
	}
	organization := scope.Organization
	project := scope.Project

	if opts.scope != "" && !slices.Contains(cfg.Authentication().GetOrganizations(), organization) {
		return util.FlagErrorf("organization %q not found", organization)
	}

	cf := ctx.ClientFactory()

	graphClient, err := cf.Graph(ctx.Context(), organization)
	if err != nil {
		return err
	}

	scopeDescriptor, _, err := util.ResolveScopeDescriptor(ctx, organization, project)
	if err != nil {
		return err
	}

	var groupDescriptors *[]string
	if len(opts.groups) > 0 {
		groupDescriptors = &opts.groups
	}

	var createdGroup *graph.GraphGroup

	switch {
	case opts.name != "":
		args := graph.CreateGroupVstsArgs{
			CreationContext: &graph.GraphGroupVstsCreationContext{
				DisplayName: &opts.name,
				Description: &opts.description,
			},
			ScopeDescriptor:  scopeDescriptor,
			GroupDescriptors: groupDescriptors,
		}
		createdGroup, err = graphClient.CreateGroupVsts(ctx.Context(), args)
	case opts.email != "":
		args := graph.CreateGroupMailAddressArgs{
			CreationContext: &graph.GraphGroupMailAddressCreationContext{
				MailAddress: &opts.email,
			},
			ScopeDescriptor:  scopeDescriptor,
			GroupDescriptors: groupDescriptors,
		}
		createdGroup, err = graphClient.CreateGroupMailAddress(ctx.Context(), args)
	case opts.originID != "":
		args := graph.CreateGroupOriginIdArgs{
			CreationContext: &graph.GraphGroupOriginIdCreationContext{
				OriginId: &opts.originID,
			},
			ScopeDescriptor:  scopeDescriptor,
			GroupDescriptors: groupDescriptors,
		}
		createdGroup, err = graphClient.CreateGroupOriginId(ctx.Context(), args)
	default:
		return fmt.Errorf("exactly one of --name, --email, or --origin-id must be specified")
	}

	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		result := groupCreateResult{
			Descriptor:    createdGroup.Descriptor,
			PrincipalName: createdGroup.PrincipalName,
			DisplayName:   createdGroup.DisplayName,
			Description:   createdGroup.Description,
			MailAddress:   createdGroup.MailAddress,
			Origin:        createdGroup.Origin,
			OriginID:      createdGroup.OriginId,
			URL:           createdGroup.Url,
			Domain:        createdGroup.Domain,
		}
		return opts.exporter.Write(ios, result)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("Descriptor", "PrincipalName", "DisplayName", "Description")
	tp.EndRow()
	tp.AddField(*createdGroup.Descriptor)
	tp.AddField(*createdGroup.PrincipalName)
	tp.AddField(types.GetValue(createdGroup.DisplayName, ""))
	tp.AddField(types.GetValue(createdGroup.Description, ""))

	tp.EndRow()
	return tp.Render()
}
