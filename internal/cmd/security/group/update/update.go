package update

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	target      string
	name        string
	description string
	descriptor  string
	exporter    util.Exporter
}

type updateResult struct {
	Descriptor    *string `json:"descriptor,omitempty"`
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	PrincipalName *string `json:"principalName,omitempty"`
	Origin        *string `json:"origin,omitempty"`
	OriginID      *string `json:"originId,omitempty"`
	Domain        *string `json:"domain,omitempty"`
	MailAddress   *string `json:"mailAddress,omitempty"`
	URL           *string `json:"url,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	commandOpts := &opts{}

	cmd := &cobra.Command{
		Use:   "update ORGANIZATION/GROUP | ORGANIZATION/PROJECT/GROUP",
		Short: "Update an Azure DevOps security group",
		Long: heredoc.Doc(`
			Update the display name and/or description of an Azure DevOps security group.

			Provide the organization segment and optional project segment to scope the lookup. At least one of --name or --description must be specified.
		`),
		Example: heredoc.Doc(`
			# Update only the description of a project-level group
			azdo security group update MyOrg/MyProject/Developers --description "Updated description"

			# Update the name of an organization-level group
			azdo security group update MyOrg/Old Group Name --name "New Group Name"
		`),
		Args: cobra.ExactArgs(1),
		Aliases: []string{
			"u",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			commandOpts.target = args[0]
			return run(ctx, commandOpts)
		},
	}

	cmd.Flags().StringVar(&commandOpts.name, "name", "", "New display name for the security group.")
	cmd.Flags().StringVar(&commandOpts.description, "description", "", "New description for the security group.")
	cmd.Flags().StringVar(&commandOpts.descriptor, "descriptor", "", "Descriptor of the security group (required if multiple groups match the name).")
	util.AddJSONFlags(cmd, &commandOpts.exporter, []string{"descriptor", "name", "description", "principalName", "origin", "originId", "domain", "mailAddress", "url"})

	return cmd
}

func run(ctx util.CmdContext, o *opts) error {
	if o.name == "" && o.description == "" {
		return util.FlagErrorf("either --name or --description must be provided")
	}

	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	target, err := util.ParseTarget(o.target)
	if err != nil {
		return err
	}

	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), target.Organization)
	if err != nil {
		return err
	}

	group, err := shared.FindGroupByName(ctx, target.Organization, target.Project, target.Target, o.descriptor)
	if err != nil {
		return err
	}

	if group.Descriptor == nil || types.GetValue(group.Descriptor, "") == "" {
		return fmt.Errorf("resolved group descriptor is empty")
	}

	replace := webapi.OperationValues.Replace
	var patchDocument []webapi.JsonPatchOperation

	if o.name != "" {
		path := "/displayName"
		value := o.name
		patchDocument = append(patchDocument, webapi.JsonPatchOperation{
			Op:    &replace,
			Path:  &path,
			Value: value,
		})
	}

	if o.description != "" {
		path := "/description"
		value := o.description
		patchDocument = append(patchDocument, webapi.JsonPatchOperation{
			Op:    &replace,
			Path:  &path,
			Value: value,
		})
	}

	updatedGroup, err := graphClient.UpdateGroup(ctx.Context(), graph.UpdateGroupArgs{
		GroupDescriptor: group.Descriptor,
		PatchDocument:   &patchDocument,
	})
	if err != nil {
		return fmt.Errorf("failed to update security group: %w", err)
	}

	ios.StopProgressIndicator()

	if o.exporter != nil {
		return o.exporter.Write(ios, updateResult{
			Descriptor:    updatedGroup.Descriptor,
			Name:          updatedGroup.DisplayName,
			Description:   updatedGroup.Description,
			PrincipalName: updatedGroup.PrincipalName,
			Origin:        updatedGroup.Origin,
			OriginID:      updatedGroup.OriginId,
			Domain:        updatedGroup.Domain,
			MailAddress:   updatedGroup.MailAddress,
			URL:           updatedGroup.Url,
		})
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("Descriptor", "Name", "Description", "Principal Name")
	tp.EndRow()
	tp.AddField(types.GetValue(updatedGroup.Descriptor, ""))
	tp.AddField(types.GetValue(updatedGroup.DisplayName, ""))
	tp.AddField(types.GetValue(updatedGroup.Description, ""))
	tp.AddField(types.GetValue(updatedGroup.PrincipalName, ""))
	tp.EndRow()

	return tp.Render()
}
