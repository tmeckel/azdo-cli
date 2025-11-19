package show

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

type opts struct {
	target   string
	exporter util.Exporter
}

type groupDetails struct {
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
		Use:   "show ORGANIZATION/GROUP | ORGANIZATION/PROJECT/GROUP",
		Short: "Show details of an Azure DevOps security group",
		Long: heredoc.Doc(`
			Display the details of an Azure DevOps security group within an organization or project scope.

			The organization segment is required. Provide an optional project segment to narrow the search scope.
		`),
		Example: heredoc.Doc(`
			# Show an organization-level security group
			azdo security group show MyOrg/Project Collection Administrators

			# Show a project-level security group
			azdo security group show MyOrg/MyProject/Contributors

			# Show details as JSON
			azdo security group show MyOrg/Contributors --json
		`),
		Args: cobra.ExactArgs(1),
		Aliases: []string{
			"s",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			commandOpts.target = args[0]
			return runCommand(ctx, commandOpts)
		},
	}

	util.AddJSONFlags(cmd, &commandOpts.exporter, []string{"descriptor", "name", "description", "principalName", "origin", "originId", "domain", "mailAddress", "url"})

	return cmd
}

func runCommand(ctx util.CmdContext, o *opts) error {
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

	zap.L().Sugar().Debugw("Resolved target for show command", "organization", target.Organization, "project", target.Project, "group", target.Target)

	groupDetailsResult, err := shared.FindGroupByName(ctx, target.Organization, target.Project, target.Target, "")
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if o.exporter != nil {
		return o.exporter.Write(ios, groupDetails{
			Descriptor:    groupDetailsResult.Descriptor,
			Name:          groupDetailsResult.DisplayName,
			Description:   groupDetailsResult.Description,
			PrincipalName: groupDetailsResult.PrincipalName,
			Origin:        groupDetailsResult.Origin,
			OriginID:      groupDetailsResult.OriginId,
			Domain:        groupDetailsResult.Domain,
			MailAddress:   groupDetailsResult.MailAddress,
			URL:           groupDetailsResult.Url,
		})
	}

	printer, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	printer.AddColumns("Descriptor", "Name", "Description", "Principal Name", "Origin", "Origin ID", "Domain", "Mail Address", "URL")
	printer.EndRow()
	printer.AddField(types.GetValue(groupDetailsResult.Descriptor, ""))
	printer.AddField(types.GetValue(groupDetailsResult.DisplayName, ""))
	printer.AddField(types.GetValue(groupDetailsResult.Description, ""))
	printer.AddField(types.GetValue(groupDetailsResult.PrincipalName, ""))
	printer.AddField(types.GetValue(groupDetailsResult.Origin, ""))
	printer.AddField(types.GetValue(groupDetailsResult.OriginId, ""))
	printer.AddField(types.GetValue(groupDetailsResult.Domain, ""))
	printer.AddField(types.GetValue(groupDetailsResult.MailAddress, ""))
	printer.AddField(types.GetValue(groupDetailsResult.Url, ""))
	printer.EndRow()

	return printer.Render()
}
