package show

import (
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelinepermissions"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/types"
)

//go:embed show.tpl
var showTpl string

const variableGroupResourceType = "variablegroup"

type opts struct {
	targetArg string

	includeVariables         bool
	includeProjectReferences bool
	includeProviderData      bool

	exporter util.Exporter
}

type variableView struct {
	Name     string  `json:"name"`
	Secret   bool    `json:"secret"`
	ReadOnly bool    `json:"readOnly"`
	Value    *string `json:"value,omitempty"`
}

type authorizedPipelineView struct {
	ID   int    `json:"id"`
	Name string `json:"name,omitempty"`
}

type variableGroupView struct {
	*taskagent.VariableGroup

	PipelinePermissions *pipelinepermissions.ResourcePipelinePermissions `json:"pipelinePermissions,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME",
		Short: "Show variable group details",
		Long: heredoc.Doc(`
			Display metadata for a variable group, including its authorization state and variables.

			The positional argument accepts the form [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME.
			When the organization segment is omitted the default organization from configuration is used.
		`),
		Args: util.ExactArgs(1, "target argument is required and must be in the form [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME"),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.targetArg = args[0]
			return run(ctx, o)
		},
	}

	cmd.Flags().BoolVar(&o.includeVariables, "include-variables", false, "Include variable values (secrets remain redacted)")
	cmd.Flags().BoolVar(&o.includeProjectReferences, "include-project-references", false, "Include variableGroupProjectReferences details")
	cmd.Flags().BoolVar(&o.includeProviderData, "include-provider-data", false, "Include providerData payloads")

	util.AddJSONFlags(cmd, &o.exporter, []string{
		// Embedded taskagent.VariableGroup fields
		"id",
		"name",
		"type",
		"description",
		"isShared",
		"variables",
		"variableGroupProjectReferences",
		"providerData",
		"createdBy",
		"createdOn",
		"modifiedBy",
		"modifiedOn",
		// Augmented fields
		"pipelinePermissions",
	})

	return cmd
}

func run(cmdCtx util.CmdContext, o *opts) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectTargetWithDefaultOrganization(cmdCtx, o.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	taskClient, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create task agent client: %w", err)
	}

	logger := zap.L().With(
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("variableGroup", scope.Target),
	)

	logger.Debug("resolving variable group identifier")
	group, err := shared.ResolveVariableGroup(cmdCtx, taskClient, scope.Project, scope.Target)
	if err != nil {
		return err
	}

	if group == nil || group.Id == nil {
		return fmt.Errorf("variable group %q not found", scope.Target)
	}

	permissionsClient, err := cmdCtx.ClientFactory().PipelinePermissions(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	logger.Debug("fetching pipeline permissions for variable group")
	perms, err := permissionsClient.GetPipelinePermissionsForResource(cmdCtx.Context(), pipelinepermissions.GetPipelinePermissionsForResourceArgs{
		Project:      types.ToPtr(scope.Project),
		ResourceType: types.ToPtr(variableGroupResourceType),
		ResourceId:   types.ToPtr(strconv.Itoa(*group.Id)),
	})
	if err != nil {
		return err
	}

	var idToName map[int]string
	if o.exporter == nil {
		buildClient, err := cmdCtx.ClientFactory().Build(cmdCtx.Context(), scope.Organization)
		if err != nil {
			return err
		}
		pipelineIDs := pipelinePermissionIDs(perms)
		if len(pipelineIDs) > 0 {
			logger.Debug("resolving pipeline names for permissions", zap.Int("count", len(pipelineIDs)))
			definitionIDs := pipelineIDs
			defs, err := buildClient.GetDefinitions(cmdCtx.Context(), build.GetDefinitionsArgs{
				Project:       types.ToPtr(scope.Project),
				DefinitionIds: &definitionIDs,
			})
			if err != nil {
				return err
			}

			idToName = make(map[int]string, len(defs.Value))
			for _, d := range defs.Value {
				if d.Id == nil {
					continue
				}
				name := strings.TrimSpace(types.GetValue(d.Name, ""))
				if name == "" {
					continue
				}
				idToName[*d.Id] = name
			}
		}
	}

	view := variableGroupView{
		VariableGroup:       group,
		PipelinePermissions: perms,
	}

	ios.StopProgressIndicator()

	if o.exporter != nil {
		return o.exporter.Write(ios, view)
	}

	t := template.New(ios.Out, ios.TerminalWidth(), ios.ColorEnabled()).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"authorized": func() *bool {
				permission := pipelinepermissions.Permission{}
				if perms != nil {
					permission = types.GetValue(perms.AllPipelines, pipelinepermissions.Permission{})
				}
				return types.ToPtr(types.GetValue(permission.Authorized, false))
			},
			"i": func(v *int) string { return strconv.Itoa(types.GetValue(v, 0)) },
			"s": func(v *string) string { return types.GetValue(v, "") },
			"b": func(v *bool) string {
				if v == nil {
					return ""
				}
				return fmt.Sprintf("%v", *v)
			},
			"ts": func(v *azuredevops.Time) string { return types.GetValue(formatTimePtr(v), "") },
			"identity": func(id *webapi.IdentityRef) string {
				if id == nil {
					return ""
				}
				display := types.GetValue(id.DisplayName, "")
				unique := types.GetValue(id.UniqueName, "")
				identifier := types.GetValue(id.Id, "")

				switch {
				case display != "" && unique != "":
					return fmt.Sprintf("%s (%s)", display, unique)
				case display != "":
					return display
				case unique != "":
					return unique
				default:
					return identifier
				}
			},
			"hasText": func(v *string) bool { return strings.TrimSpace(types.GetValue(v, "")) != "" },
			"hasAny":  func(v any) bool { return v != nil },
			"vars": func(v *map[string]interface{}) []variableView {
				return expandVariables(v)
			},
			"pipelines": func() []authorizedPipelineView {
				return toAuthorizedPipelines(perms, idToName)
			},
			"projRefs": func(v *[]taskagent.VariableGroupProjectReference) []taskagent.VariableGroupProjectReference {
				if v == nil {
					return []taskagent.VariableGroupProjectReference{}
				}
				return *v
			},
		})
	if err := t.Parse(showTpl); err != nil {
		return err
	}

	return t.ExecuteData(view)
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

func toAuthorizedPipelines(perms *pipelinepermissions.ResourcePipelinePermissions, idToName map[int]string) []authorizedPipelineView {
	if perms == nil {
		return nil
	}
	pipelines := types.GetValue(perms.Pipelines, []pipelinepermissions.PipelinePermission(nil))
	if len(pipelines) == 0 {
		return nil
	}
	results := make([]authorizedPipelineView, 0, len(pipelines))
	for _, p := range pipelines {
		if p.Id == nil {
			continue
		}
		id := *p.Id
		name := ""
		if idToName != nil {
			name = idToName[id]
		}
		results = append(results, authorizedPipelineView{ID: id, Name: name})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Name == results[j].Name {
			return results[i].ID < results[j].ID
		}
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})
	return results
}

func pipelinePermissionIDs(perms *pipelinepermissions.ResourcePipelinePermissions) []int {
	if perms == nil {
		return nil
	}
	pipelines := types.GetValue(perms.Pipelines, []pipelinepermissions.PipelinePermission(nil))
	if len(pipelines) == 0 {
		return nil
	}
	ids := make([]int, 0, len(pipelines))
	seen := make(map[int]struct{})
	for _, p := range pipelines {
		if p.Id == nil {
			continue
		}
		id := *p.Id
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func expandVariables(vars *map[string]interface{}) []variableView {
	varMap := types.GetValue(vars, map[string]interface{}{})
	if len(varMap) == 0 {
		return nil
	}
	results := make([]variableView, 0, len(varMap))
	for name, raw := range varMap {
		v := variableView{Name: name}

		switch typed := raw.(type) {
		case taskagent.VariableValue:
			v.Secret = types.GetValue(typed.IsSecret, false)
			v.ReadOnly = types.GetValue(typed.IsReadOnly, false)
			if !v.Secret {
				v.Value = typed.Value
			}
		case *taskagent.VariableValue:
			if typed != nil {
				v.Secret = types.GetValue(typed.IsSecret, false)
				v.ReadOnly = types.GetValue(typed.IsReadOnly, false)
				if !v.Secret {
					v.Value = typed.Value
				}
			}
		case map[string]any:
			if secret, ok := typed["isSecret"].(bool); ok {
				v.Secret = secret
			}
			if ro, ok := typed["isReadOnly"].(bool); ok {
				v.ReadOnly = ro
			}
			if value, ok := typed["value"].(string); ok && !v.Secret {
				v.Value = &value
			}
		}

		results = append(results, v)
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return results
}
