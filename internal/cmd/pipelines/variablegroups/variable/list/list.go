package list

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo/extensions"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"go.uber.org/zap"
)

type opts struct {
	scope    string
	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT/VARIABLEGROUP",
		Short: "List variables in a variable group",
		Long: heredoc.Doc(`
			List the variables in a variable group.

			The command retrieves a variable group and lists its variables. Secret variables have their
			values masked by default.

			The VARIABLEGROUP can be specified by its ID or name.
		`),
		Example: heredoc.Doc(`
			# List variables in a group by ID within a project
			azdo pipelines variable-groups variable list MyProject/123

			# List variables in a group by name within a project and organization
			azdo pipelines variable-groups variable list 'MyOrg/MyProject/My Variable Group'

			# Export variables to JSON
			azdo pipelines variable-groups variable list MyProject/123 --json
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scope = args[0]
			return run(ctx, opts)
		},
	}

	util.AddJSONFlags(cmd, &opts.exporter, []string{"name", "secret", "value"})

	return cmd
}

func run(ctx util.CmdContext, opts *opts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseTargetWithDefaultOrganization(ctx, opts.scope)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	// Determine if groupIdentifier is ID or Name
	groupID, err := strconv.Atoi(scope.Target)
	if err == nil && groupID < 0 {
		return util.FlagErrorf("Invalid group id %d", groupID)
	} else if err != nil {
		groupID = -1
	}

	client, err := ctx.ClientFactory().TaskAgent(ctx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	var group *taskagent.VariableGroup
	var g *[]taskagent.VariableGroup

	if groupID > -1 {
		zap.L().Debug("Fetching variable group by ID", zap.Int("groupID", groupID))
		g, err = client.GetVariableGroupsById(ctx.Context(), taskagent.GetVariableGroupsByIdArgs{
			Project: &scope.Project,
			GroupIds: &[]int{
				groupID,
			},
		})
	} else {
		zap.L().Debug("Fetching variable group by name", zap.String("groupName", scope.Target))
		g, err = client.GetVariableGroups(ctx.Context(), taskagent.GetVariableGroupsArgs{
			Project:   &scope.Project,
			GroupName: &scope.Target,
		})
	}
	if err != nil {
		return util.FlagErrorWrap(err)
	}
	if g != nil && len(*g) > 0 {
		group = &(*g)[0]
	}

	if group == nil {
		return util.FlagErrorf("variable group %q not found", opts.scope)
	}

	variables := extensions.ToVariableValues(group.Variables)
	sort.Slice(variables, func(i, j int) bool { return variables[i].Name < variables[j].Name })

	ios.StopProgressIndicator()

	// JSON output
	if opts.exporter != nil {
		// Mask secret values for JSON output
		for i := range variables {
			if variables[i].IsSecret {
				variables[i].Value = nil
			}
		}
		return opts.exporter.Write(ios, variables)
	}

	// Table output
	tp, err := ctx.Printer("table")
	if err != nil {
		return err
	}
	tp.AddColumns("NAME", "VALUE", "IS SECRET")
	tp.EndRow()

	for _, v := range variables {
		var value string
		if v.Value != nil {
			value = fmt.Sprintf("%v", v.Value)
		}
		if v.IsSecret {
			value = "***"
		}
		tp.AddField(v.Name)
		tp.AddField(value)
		tp.AddField(strconv.FormatBool(v.IsSecret))
		tp.EndRow()
	}

	return tp.Render()
}
