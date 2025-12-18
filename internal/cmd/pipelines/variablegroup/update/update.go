package update

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelinepermissions"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

const variableGroupResourceType = "variablegroup"

type opts struct {
	targetArg string

	name                     string
	nameChanged              bool
	description              string
	descriptionChanged       bool
	vgType                   string
	vgTypeChanged            bool
	providerDataJSON         string
	providerDataJSONChanged  bool
	clearProviderData        bool
	clearProviderDataChanged bool

	projectReferences             []string
	projectReferencesChanged      bool
	clearProjectReferences        bool
	clearProjectReferencesChanged bool

	authorize        bool
	authorizeChanged bool

	exporter util.Exporter
}

type variableGroupView struct {
	*taskagent.VariableGroup
	PipelinePermissions *pipelinepermissions.ResourcePipelinePermissions `json:"pipelinePermissions,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "update [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME",
		Short: "Update variable group metadata and permissions",
		Long: heredoc.Doc(`
            Update a variable group's metadata (name, description, type, providerData),
            manage cross-project sharing, and optionally toggle 'authorize for all pipelines'.
        `),
		Args: util.ExactArgs(1, "target argument is required and must be in the form [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME"),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.targetArg = args[0]

			// capture which flags were explicitly set
			o.nameChanged = cmd.Flags().Changed("name")
			o.descriptionChanged = cmd.Flags().Changed("description")
			o.vgTypeChanged = cmd.Flags().Changed("type")
			o.providerDataJSONChanged = cmd.Flags().Changed("provider-data-json")
			o.clearProviderDataChanged = cmd.Flags().Changed("clear-provider-data")
			o.projectReferencesChanged = cmd.Flags().Changed("project-reference")
			o.clearProjectReferencesChanged = cmd.Flags().Changed("clear-project-references")
			o.authorizeChanged = cmd.Flags().Changed("authorize")

			return run(ctx, o)
		},
	}

	cmd.Flags().StringVar(&o.name, "name", "", "New display name")
	cmd.Flags().StringVar(&o.description, "description", "", "New description (empty string clears it)")
	cmd.Flags().StringVar(&o.vgType, "type", "", "Variable group type (e.g., Vsts, AzureKeyVault)")
	cmd.Flags().StringVar(&o.providerDataJSON, "provider-data-json", "", "Raw JSON payload for providerData")
	cmd.Flags().BoolVar(&o.clearProviderData, "clear-provider-data", false, "Clear providerData (mutually exclusive with --provider-data-json)")

	cmd.Flags().StringArrayVar(&o.projectReferences, "project-reference", nil, "Project reference to share with (repeatable)")
	cmd.Flags().BoolVar(&o.clearProjectReferences, "clear-project-references", false, "Overwrite existing project references with the provided set; when provided without any --project-reference, removes all references")

	cmd.Flags().BoolVar(&o.authorize, "authorize", false, "Grant (true) or remove (false) access permission to all pipelines")

	util.AddJSONFlags(cmd, &o.exporter, []string{
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

    // validate mutual exclusivity
    if o.providerDataJSONChanged && o.clearProviderDataChanged {
        return util.FlagErrorf("--provider-data-json and --clear-provider-data are mutually exclusive")
    }

    // Determine whether any model fields will be changed
    willUpdateGroup := o.nameChanged || o.descriptionChanged || o.vgTypeChanged || o.providerDataJSONChanged || o.clearProviderDataChanged || o.projectReferencesChanged || o.clearProjectReferencesChanged
    if !willUpdateGroup && !o.authorizeChanged {
        return util.FlagErrorf("at least one mutating flag must be supplied")
    }

    taskClient, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), scope.Organization)
    if err != nil {
        return fmt.Errorf("failed to create task agent client: %w", err)
    }

    // resolve variable group
    group, err := shared.ResolveVariableGroup(cmdCtx, taskClient, scope.Project, scope.Target)
    if err != nil {
        return err
    }
    if group == nil || group.Id == nil {
        return fmt.Errorf("variable group %q not found", scope.Target)
    }

	var updatedGroup *taskagent.VariableGroup

	// If we need to update the variable group model
	if willUpdateGroup {
		params := taskagent.VariableGroupParameters{}

		if o.nameChanged {
			params.Name = types.ToPtr(o.name)
		}
		if o.descriptionChanged {
			params.Description = types.ToPtr(o.description)
		}
		if o.vgTypeChanged {
			params.Type = types.ToPtr(o.vgType)
		}
		if o.providerDataJSONChanged {
			// parse JSON
			var pd interface{}
			if err := json.Unmarshal([]byte(o.providerDataJSON), &pd); err != nil {
				return util.FlagErrorWrap(fmt.Errorf("invalid provider-data-json: %w", err))
			}
			params.ProviderData = pd
		}
		if o.clearProviderDataChanged && o.clearProviderData {
			params.ProviderData = nil
		}

		if o.projectReferencesChanged || o.clearProjectReferencesChanged {
			// build project references if provided
			if o.projectReferencesChanged && len(o.projectReferences) > 0 {
				refs := make([]taskagent.VariableGroupProjectReference, 0, len(o.projectReferences))
				for _, p := range o.projectReferences {
					pr := taskagent.VariableGroupProjectReference{
						ProjectReference: &taskagent.ProjectReference{Name: types.ToPtr(p)},
					}
					refs = append(refs, pr)
				}
				params.VariableGroupProjectReferences = &refs
			} else if o.clearProjectReferencesChanged && o.clearProjectReferences && !o.projectReferencesChanged {
				// explicit clear -> empty slice
				refs := make([]taskagent.VariableGroupProjectReference, 0)
				params.VariableGroupProjectReferences = &refs
			} else if o.clearProjectReferencesChanged && o.clearProjectReferences && o.projectReferencesChanged && len(o.projectReferences) == 0 {
				// clear provided without any --project-reference -> remove all
				refs := make([]taskagent.VariableGroupProjectReference, 0)
				params.VariableGroupProjectReferences = &refs
			}
		}

		// call UpdateVariableGroup
		updated, err := taskClient.UpdateVariableGroup(cmdCtx.Context(), taskagent.UpdateVariableGroupArgs{
			VariableGroupParameters: &params,
			GroupId:                 types.ToPtr(*group.Id),
		})
		if err != nil {
			return err
		}
		updatedGroup = updated
	} else {
		// no model changes, keep original
		updatedGroup = group
	}

	// If authorize changed, call pipeline permissions API
	var perms *pipelinepermissions.ResourcePipelinePermissions
	if o.authorizeChanged {
		permClient, err := cmdCtx.ClientFactory().PipelinePermissions(cmdCtx.Context(), scope.Organization)
		if err != nil {
			return err
		}

		desired := &pipelinepermissions.Permission{Authorized: types.ToPtr(o.authorize)}
		rp := pipelinepermissions.ResourcePipelinePermissions{AllPipelines: desired}
		updatedPerms, err := permClient.UpdatePipelinePermisionsForResource(cmdCtx.Context(), pipelinepermissions.UpdatePipelinePermisionsForResourceArgs{
			ResourceAuthorization: &rp,
			Project:               types.ToPtr(scope.Project),
			ResourceType:          types.ToPtr(variableGroupResourceType),
			ResourceId:            types.ToPtr(strconv.Itoa(*group.Id)),
		})
		if err != nil {
			return err
		}
		perms = updatedPerms
	} else {
		// fetch existing perms to include in output
		permClient, err := cmdCtx.ClientFactory().PipelinePermissions(cmdCtx.Context(), scope.Organization)
		if err == nil {
			p, _ := permClient.GetPipelinePermissionsForResource(cmdCtx.Context(), pipelinepermissions.GetPipelinePermissionsForResourceArgs{
				Project:      types.ToPtr(scope.Project),
				ResourceType: types.ToPtr(variableGroupResourceType),
				ResourceId:   types.ToPtr(strconv.Itoa(*group.Id)),
			})
			perms = p
		}
	}

	ios.StopProgressIndicator()

	view := variableGroupView{VariableGroup: updatedGroup, PipelinePermissions: perms}

	if o.exporter != nil {
		return o.exporter.Write(ios, view)
	}

	// simple textual output
	fmt.Fprintf(ios.Out, "Updated variable group %q (id: %d)\n", types.GetValue(updatedGroup.Name, ""), types.GetValue(updatedGroup.Id, 0))
	if perms != nil && perms.AllPipelines != nil && perms.AllPipelines.Authorized != nil {
		fmt.Fprintf(ios.Out, "Authorize for all pipelines: %v\n", *perms.AllPipelines.Authorized)
	}

	return nil
}
