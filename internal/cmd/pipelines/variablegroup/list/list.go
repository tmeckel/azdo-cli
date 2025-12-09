package list

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

type opts struct {
	scope    string
	action   string
	order    string
	top      int
	maxItems int
	exporter util.Exporter
}

type identityJSON struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	UniqueName  string `json:"uniqueName,omitempty"`
}

type variableGroupJSON struct {
	ID          *int                                       `json:"id,omitempty"`
	Name        *string                                    `json:"name,omitempty"`
	Type        *string                                    `json:"type,omitempty"`
	Description *string                                    `json:"description,omitempty"`
	IsShared    *bool                                      `json:"isShared,omitempty"`
	CreatedBy   *identityJSON                              `json:"createdBy,omitempty"`
	CreatedOn   *string                                    `json:"createdOn,omitempty"`
	ModifiedBy  *identityJSON                              `json:"modifiedBy,omitempty"`
	ModifiedOn  *string                                    `json:"modifiedOn,omitempty"`
	ProjectRefs *[]taskagent.VariableGroupProjectReference `json:"projectReferences,omitempty"`
	Variables   *map[string]interface{}                    `json:"variables,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List variable groups",
		Long: heredoc.Doc(`
			List every variable group defined in a project with optional filtering.
		`),
		Example: heredoc.Doc(`
			# List all variable groups in a project
			$ azdo pipelines variable-groups list "my-project"

			# List variable groups with a specific name
			$ azdo pipelines variable-groups list "my-project" --name "my-variable-group"
		`),
		Aliases: []string{
			"ls",
			"l",
		},
		Args: util.ExactArgs(1, "project argument is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scope = args[0]
			return run(ctx, opts)
		},
	}

	util.StringEnumFlag(cmd, &opts.action, "action", "", "", []string{"none", "manage", "use"}, "Action filter string (e.g., 'manage', 'use')")
	util.StringEnumFlag(cmd, &opts.order, "order", "", "desc", []string{"desc", "asc"}, "Order of variable groups (asc, desc)")
	cmd.Flags().IntVar(&opts.top, "top", 0, "Server-side page size hint (positive integer)")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Optional client-side cap on results; stop fetching once reached")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id",
		"name",
		"type",
		"description",
		"isShared",
		"createdBy",
		"createdOn",
		"modifiedBy",
		"modifiedOn",
		"projectReferences",
		"variables",
	})

	return cmd
}

func run(cmdCtx util.CmdContext, opts *opts) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if opts.top < 0 {
		return util.FlagErrorf("invalid --top value %d; must be greater than 0", opts.top)
	}
	if opts.maxItems < 0 {
		return util.FlagErrorf("invalid --max-items value %d; must be greater than 0", opts.maxItems)
	}

	scope, err := util.ParseProjectScope(cmdCtx, opts.scope)
	if err != nil {
		return util.FlagErrorf("invalid project argument: %w", err)
	}

	var actionFilter *taskagent.VariableGroupActionFilter
	if strings.TrimSpace(opts.action) != "" {
		switch strings.ToLower(strings.TrimSpace(opts.action)) {
		case "manage":
			value := taskagent.VariableGroupActionFilterValues.Manage
			actionFilter = &value
		case "use":
			value := taskagent.VariableGroupActionFilterValues.Use
			actionFilter = &value
		case "none":
			value := taskagent.VariableGroupActionFilterValues.None
			actionFilter = &value
		default:
			return util.FlagErrorf("invalid action %q; expected manage, use, or none", opts.action)
		}
	}

	var queryOrder taskagent.VariableGroupQueryOrder
	switch strings.ToLower(strings.TrimSpace(opts.order)) {
	case "", "desc", "iddescending":
		queryOrder = taskagent.VariableGroupQueryOrderValues.IdDescending
	case "asc", "idascending":
		queryOrder = taskagent.VariableGroupQueryOrderValues.IdAscending
	default:
		return util.FlagErrorf("invalid order %q; expected asc or desc", opts.order)
	}

	var top *int
	if opts.top > 0 {
		top = types.ToPtr(opts.top)
	}

	extensionClient, err := cmdCtx.ClientFactory().Extensions(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	getVariableGroupsArgs := taskagent.GetVariableGroupsArgs{
		Project:      types.ToPtr(scope.Project),
		ActionFilter: actionFilter,
		QueryOrder:   &queryOrder,
		Top:          top,
	}

	variableGroups, err := azdo.GetVariableGroups(cmdCtx.Context(), extensionClient, getVariableGroupsArgs)
	if err != nil {
		return err
	}

	logger := zap.L()

	if opts.maxItems > 0 && len(variableGroups) > opts.maxItems {
		logger.Debug("truncating result set to max-items", zap.Int("maxItems", opts.maxItems))
		variableGroups = variableGroups[:opts.maxItems]
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		payload := make([]variableGroupJSON, 0, len(variableGroups))
		for _, vg := range variableGroups {
			payload = append(payload, newVariableGroupJSON(vg))
		}
		return opts.exporter.Write(ios, payload)
	}

	tp, err := cmdCtx.Printer("table")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "NAME", "TYPE", "VARIABLE COUNT", "DESCRIPTION")

	for _, vg := range variableGroups {
		id := types.GetValue(vg.Id, 0)
		name := types.GetValue(vg.Name, "")
		typeVal := types.GetValue(vg.Type, "")

		var variableCount int
		if vg.Variables != nil {
			variableCount = len(*vg.Variables)
		}

		description := types.GetValue(vg.Description, "")

		tp.AddField(fmt.Sprintf("%d", id))
		tp.AddField(name)
		tp.AddField(typeVal)
		tp.AddField(fmt.Sprintf("%d", variableCount))
		tp.AddField(description)
		tp.EndRow()
	}

	return tp.Render()
}

func newVariableGroupJSON(vg taskagent.VariableGroup) variableGroupJSON {
	return variableGroupJSON{
		ID:          vg.Id,
		Name:        vg.Name,
		Type:        vg.Type,
		Description: vg.Description,
		IsShared:    vg.IsShared,
		CreatedBy:   newIdentityJSON(vg.CreatedBy),
		CreatedOn:   formatTimePtr(vg.CreatedOn),
		ModifiedBy:  newIdentityJSON(vg.ModifiedBy),
		ModifiedOn:  formatTimePtr(vg.ModifiedOn),
		ProjectRefs: vg.VariableGroupProjectReferences,
		Variables:   vg.Variables,
	}
}

func newIdentityJSON(ref *webapi.IdentityRef) *identityJSON {
	if ref == nil {
		return nil
	}

	id := types.GetValue(ref.Id, "")
	display := types.GetValue(ref.DisplayName, "")
	unique := types.GetValue(ref.UniqueName, "")

	if id == "" && display == "" && unique == "" {
		return nil
	}

	return &identityJSON{
		ID:          id,
		DisplayName: display,
		UniqueName:  unique,
	}
}

func formatTimePtr(ts *azuredevops.Time) *string {
	if ts == nil {
		return nil
	}
	formatted := ts.AsQueryParameter()
	if strings.TrimSpace(formatted) == "" {
		return nil
	}
	return &formatted
}
