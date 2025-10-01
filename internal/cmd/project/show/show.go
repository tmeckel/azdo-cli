package show

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	project  string
	exporter util.Exporter
}

type projectShowResult struct {
	ID              *string `json:"id,omitempty"`
	Name            *string `json:"name,omitempty"`
	State           *string `json:"state,omitempty"`
	Visibility      *string `json:"visibility,omitempty"`
	Process         *string `json:"process,omitempty"`
	SourceControl   *string `json:"sourceControl,omitempty"`
	LastUpdateTime  *string `json:"lastUpdateTime,omitempty"`
	Revision        *uint64 `json:"revision,omitempty"`
	Description     *string `json:"description,omitempty"`
	URL             *string `json:"url,omitempty"`
	DefaultTeamName *string `json:"defaultTeamName,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]PROJECT",
		Short: "Show details of an Azure DevOps Project",
		Long: heredoc.Doc(`
			Shows details of an Azure DevOps project in the specified organization.

			If the organization name is omitted from the project argument, the default configured organization is used.
		`),
		Example: heredoc.Doc(`
			# Show project details in the default organization
			azdo project show MyProject

			# Show project details in a specific organization
			azdo project show MyOrg/MyProject
		`),
		Args: cobra.ExactArgs(1),
		Aliases: []string{
			"s",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.project = args[0]
			return runCommand(ctx, o)
		},
	}

	util.AddJSONFlags(cmd, &o.exporter, []string{"id", "name", "state", "visibility", "process", "sourceControl", "lastUpdateTime", "revision", "description", "url", "defaultTeamName"})

	return cmd
}

func runCommand(ctx util.CmdContext, o *opts) error {
	prj, err := azdo.ProjectFromName(o.project)
	if err != nil {
		return err
	}

	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), prj.Organization())
	if err != nil {
		return err
	}

	project, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId:           types.ToPtr(prj.Project()),
		IncludeCapabilities: types.ToPtr(true),
	})
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if o.exporter != nil {
		var processName, sourceControlType string
		if project.Capabilities != nil {
			if caps, ok := (*project.Capabilities)["processTemplate"]; ok {
				processName = caps["templateName"]
			}
			if caps, ok := (*project.Capabilities)["versioncontrol"]; ok {
				sourceControlType = caps["sourceControlType"]
			}
		}

		var defaultTeamName string
		if project.DefaultTeam != nil {
			defaultTeamName = *project.DefaultTeam.Name
		}

		lastUpdateTime := project.LastUpdateTime.Time.String()

		var state, visibility string
		if project.State != nil {
			state = string(*project.State)
		}
		if project.Visibility != nil {
			visibility = string(*project.Visibility)
		}

		result := projectShowResult{
			ID:              types.ToPtr(project.Id.String()),
			Name:            project.Name,
			State:           types.ToPtr(state),
			Visibility:      types.ToPtr(visibility),
			Process:         types.ToPtr(processName),
			SourceControl:   types.ToPtr(sourceControlType),
			LastUpdateTime:  &lastUpdateTime,
			Revision:        project.Revision,
			Description:     project.Description,
			URL:             project.Url,
			DefaultTeamName: &defaultTeamName,
		}
		return o.exporter.Write(ios, result)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "Name", "State", "Visibility", "Process", "Source Control", "Last Update Time", "Revision", "Description", "URL", "Default Team")
	tp.EndRow()

	var processName, sourceControlType string
	if project.Capabilities != nil {
		if caps, ok := (*project.Capabilities)["processTemplate"]; ok {
			processName = caps["templateName"]
		}
		if caps, ok := (*project.Capabilities)["versioncontrol"]; ok {
			sourceControlType = caps["sourceControlType"]
		}
	}

	var defaultTeamName string
	if project.DefaultTeam != nil {
		defaultTeamName = *project.DefaultTeam.Name
	}

	var state, visibility string
	if project.State != nil {
		state = string(*project.State)
	}
	if project.Visibility != nil {
		visibility = string(*project.Visibility)
	}

	tp.AddField(project.Id.String())
	tp.AddField(types.GetValue(project.Name, ""))
	tp.AddField(state)
	tp.AddField(visibility)
	tp.AddField(processName)
	tp.AddField(sourceControlType)
	tp.AddField(project.LastUpdateTime.Time.String())
	tp.AddField(fmt.Sprintf("%d", types.GetValue(project.Revision, 0)))
	tp.AddField(types.GetValue(project.Description, ""))
	tp.AddField(types.GetValue(project.Url, ""))
	tp.AddField(defaultTeamName)
	tp.EndRow()

	return tp.Render()
}
