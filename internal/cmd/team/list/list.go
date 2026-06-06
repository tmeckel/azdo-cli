package list

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type listOptions struct {
	scopeArg string
	top      int
	skip     int
	mine     bool
	maxItems int
	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List teams in a project.",
		Long: heredoc.Doc(`
			List all teams in the specified project. Supports server-side paging via
			--top and --skip, --mine filtering, and JSON export.
		`),
		Example: heredoc.Doc(`
			# List all teams in the default organization
			azdo team list Fabrikam

			# List the first 10 teams in a specific organization
			azdo team list MyOrg/Fabrikam --top 10

			# List teams you are a member of
			azdo team list Fabrikam --mine
		`),
		Aliases: []string{"ls", "l"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runList(ctx, opts)
		},
	}

	cmd.Flags().IntVar(&opts.top, "top", 0, "Maximum number of teams to return per page (server-side; 0 = server default)")
	cmd.Flags().IntVar(&opts.skip, "skip", 0, "Number of teams to skip (server-side)")
	cmd.Flags().BoolVar(&opts.mine, "mine", false, "Return only teams the current user is a member of")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Maximum number of teams to return across all pages (client-side; 0 = unlimited)")

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "name", "description", "url",
		"identity", "identityUrl", "projectId", "projectName",
	})

	return cmd
}

func runList(ctx util.CmdContext, opts *listOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	client, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Core client: %w", err)
	}

	teams, err := fetchTeams(ctx, client, scope.Project, opts)
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, teams)
	}

	return renderTeamsTable(ctx, teams)
}

func fetchTeams(ctx util.CmdContext, client core.Client, project string, opts *listOptions) ([]core.WebApiTeam, error) {
	if opts.maxItems < 0 {
		return nil, util.FlagErrorf("--max-items must be >= 0")
	}

	out := make([]core.WebApiTeam, 0)
	skip := opts.skip

	for {
		args := core.GetTeamsArgs{ProjectId: &project}
		if opts.mine {
			mine := true
			args.Mine = &mine
		}
		if opts.top > 0 {
			top := opts.top
			args.Top = &top
		}
		if skip > 0 {
			s := skip
			args.Skip = &s
		}

		resp, err := client.GetTeams(ctx.Context(), args)
		if err != nil {
			return nil, fmt.Errorf("failed to list teams: %w", err)
		}
		if resp == nil || len(*resp) == 0 {
			return out, nil
		}

		for _, t := range *resp {
			out = append(out, t)
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

func renderTeamsTable(ctx util.CmdContext, teams []core.WebApiTeam) error {
	sort.Slice(teams, func(i, j int) bool {
		return strings.ToLower(types.GetValue(teams[i].Name, "")) < strings.ToLower(types.GetValue(teams[j].Name, ""))
	})

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "NAME", "DESCRIPTION", "PROJECT")
	tp.EndRow()

	for _, t := range teams {
		tp.AddField(types.GetValue(t.Id, uuid.UUID{}).String())
		tp.AddField(types.GetValue(t.Name, ""))
		tp.AddField(types.GetValue(t.Description, ""))
		tp.AddField(types.GetValue(t.ProjectName, ""))
		tp.EndRow()
	}

	return tp.Render()
}
