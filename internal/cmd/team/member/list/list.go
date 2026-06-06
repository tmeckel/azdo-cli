package list

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type listOptions struct {
	targetArg string
	top       int
	skip      int
	maxItems  int
	exporter  util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT/TEAM",
		Short: "List members of a team.",
		Long: heredoc.Doc(`
			List members of a team. The TEAM argument accepts the ID (GUID)
			or name of the team. Supports server-side paging via --top and
			--skip.
		`),
		Example: heredoc.Doc(`
			# List members of a team
			azdo team member list Fabrikam/"Fabrikam Engineering"

			# List the first 10 members in a specific organization
			azdo team member list MyOrg/Fabrikam/MyTeam --top 10
		`),
		Aliases: []string{"members", "ls", "l"},
		Args:    util.ExactArgs(1, "team argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runList(ctx, opts)
		},
	}

	cmd.Flags().IntVar(&opts.top, "top", 0, "Maximum number of members to return per page (server-side; 0 = server default)")
	cmd.Flags().IntVar(&opts.skip, "skip", 0, "Number of members to skip (server-side)")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Maximum number of members to return across all pages (client-side; 0 = unlimited)")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"identity", "isTeamAdmin"})

	return cmd
}

func runList(ctx util.CmdContext, opts *listOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectTargetWithDefaultOrganization(ctx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	client, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Core client: %w", err)
	}

	teamID := scope.Targets[0]

	members, err := fetchTeamMembers(ctx, client, scope.Project, teamID, opts)
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, &members)
	}

	return renderMembersTable(ctx, members)
}

func fetchTeamMembers(ctx util.CmdContext, client core.Client, project, teamID string, opts *listOptions) ([]webapi.TeamMember, error) {
	if opts.maxItems < 0 {
		return nil, util.FlagErrorf("--max-items must be >= 0")
	}

	out := make([]webapi.TeamMember, 0)
	skip := opts.skip

	for {
		args := core.GetTeamMembersWithExtendedPropertiesArgs{
			ProjectId: &project,
			TeamId:    &teamID,
		}
		if opts.top > 0 {
			top := opts.top
			args.Top = &top
		}
		if skip > 0 {
			s := skip
			args.Skip = &s
		}

		resp, err := client.GetTeamMembersWithExtendedProperties(ctx.Context(), args)
		if err != nil {
			return nil, fmt.Errorf("failed to list team members: %w", err)
		}
		if resp == nil || len(*resp) == 0 {
			return out, nil
		}

		for _, m := range *resp {
			out = append(out, m)
			if opts.maxItems > 0 && len(out) >= opts.maxItems {
				return out, nil
			}
		}

		if opts.top > 0 && len(*resp) < opts.top {
			return out, nil
		}

		skip += opts.top
		if opts.top == 0 {
			return out, nil
		}
	}
}

func renderMembersTable(ctx util.CmdContext, members []webapi.TeamMember) error {
	sort.Slice(members, func(i, j int) bool {
		li := strings.ToLower(fieldIdentityDisplay(members[i].Identity))
		lj := strings.ToLower(fieldIdentityDisplay(members[j].Identity))
		return li < lj
	})

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "DISPLAY NAME", "UNIQUE NAME", "IS TEAM ADMIN")
	tp.EndRow()

	for _, m := range members {
		identity := m.Identity
		var id, display, unique string
		if identity != nil {
			id = types.GetValue(identity.Id, "")
			display = types.GetValue(identity.DisplayName, "")
			unique = types.GetValue(identity.UniqueName, "")
		}
		tp.AddField(id)
		tp.AddField(display)
		tp.AddField(unique)
		tp.AddField(strconv.FormatBool(types.GetValue(m.IsTeamAdmin, false)))
		tp.EndRow()
	}

	return tp.Render()
}

func fieldIdentityDisplay(id *webapi.IdentityRef) string {
	if id == nil {
		return ""
	}
	if d := types.GetValue(id.DisplayName, ""); d != "" {
		return d
	}
	return types.GetValue(id.UniqueName, "")
}
