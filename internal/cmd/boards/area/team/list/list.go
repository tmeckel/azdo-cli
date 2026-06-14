package list

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type listOptions struct {
	targetArg string
	exporter  util.Exporter
}

type teamFieldValueView struct {
	AreaPath        string `json:"areaPath"`
	IncludeChildren bool   `json:"includeChildren"`
	IsDefault       bool   `json:"isDefault"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT/TEAM",
		Short: "List area paths assigned to a team.",
		Long: heredoc.Doc(`
			List Azure Boards area paths assigned to a team. The TEAM argument accepts
			the ID (GUID) or name of the team. The argument accepts the form
			[ORGANIZATION/]PROJECT/TEAM. When the organization segment is omitted,
			the default organization from configuration is used.
		`),
		Example: heredoc.Doc(`
			# List area paths for a team using the default organization
			azdo boards area team list Fabrikam/"Fabrikam Engineering"

			# List area paths for a team in a specific organization
			azdo boards area team list MyOrg/Fabrikam/"My Team"
		`),
		Aliases: []string{"ls", "l"},
		Args:    util.ExactArgs(1, "team argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runList(ctx, opts)
		},
	}

	util.AddJSONFlags(cmd, &opts.exporter, []string{"areaPath", "includeChildren", "isDefault"})

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

	client, err := ctx.ClientFactory().Work(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Work client: %w", err)
	}

	teamFieldValues, err := client.GetTeamFieldValues(ctx.Context(), work.GetTeamFieldValuesArgs{
		Project: &scope.Project,
		Team:    &scope.Targets[0],
	})
	if err != nil {
		return fmt.Errorf("failed to fetch team field values: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, buildView(teamFieldValues))
	}

	return renderTable(ctx, teamFieldValues)
}

func buildView(tfv *work.TeamFieldValues) []teamFieldValueView {
	if tfv == nil || tfv.Values == nil {
		return nil
	}

	defaultValue := types.GetValue(tfv.DefaultValue, "")

	views := make([]teamFieldValueView, 0, len(*tfv.Values))
	for _, v := range *tfv.Values {
		areaPath := types.GetValue(v.Value, "")
		views = append(views, teamFieldValueView{
			AreaPath:        areaPath,
			IncludeChildren: types.GetValue(v.IncludeChildren, false),
			IsDefault:       strings.EqualFold(areaPath, defaultValue),
		})
	}

	sort.Slice(views, func(i, j int) bool {
		return strings.ToLower(views[i].AreaPath) < strings.ToLower(views[j].AreaPath)
	})

	return views
}

func renderTable(ctx util.CmdContext, tfv *work.TeamFieldValues) error {
	views := buildView(tfv)
	if len(views) == 0 {
		return util.NewNoResultsError("no team area paths found")
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("AREA PATH", "INCLUDE SUB AREAS")
	tp.EndRow()

	for _, v := range views {
		label := v.AreaPath
		if v.IsDefault {
			label += " (default)"
		}
		tp.AddField(label)
		if v.IncludeChildren {
			tp.AddField("yes")
		} else {
			tp.AddField("no")
		}
		tp.EndRow()
	}

	return tp.Render()
}
