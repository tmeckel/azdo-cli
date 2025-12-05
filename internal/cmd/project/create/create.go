package create

import (
	"fmt"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	project     string
	description string
	process     string
	sourceCtrl  string
	visibility  string
	noWait      bool
	maxWait     int
	exporter    util.Exporter
}

type projectCreateResult struct {
	OperationID     *string `json:"operationID,omitempty"`
	OperationStatus *string `json:"operationStatus,omitempty"`
	OperationURL    *string `json:"operationURL,omitempty"`
	ID              *string `json:"id,omitempty"`
	Name            *string `json:"name,omitempty"`
	State           *string `json:"state,omitempty"`
	Visibility      *string `json:"visibility,omitempty"`
	Process         *string `json:"process,omitempty"`
	SourceControl   *string `json:"sourceControl,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "create [ORGANIZATION/]PROJECT",
		Short: "Create a new Azure DevOps Project",
		Long: heredoc.Doc(`
				Create a new Azure DevOps project in the specified organization.

				This command queues a project creation operation and polls for its completion.
				By default, it waits for the project to be created and then displays the project details.

				You can use the --no-wait flag to have the command return immediately after queuing the operation.
				In this case, it will output the operation ID, status, and URL, which you can use to monitor the creation process.

				The --max-wait flag allows you to specify a custom timeout for the polling operation.

				If the organization name is omitted from the project argument, the default configured organization is used.
			`),
		Example: heredoc.Doc(`
				# Create a project in the default organization and wait for completion
				azdo project create MyProject --description "A new project" --process "Scrum" --visibility private

				# Create a public project with TFVC source control in a specific organization
				azdo project create MyOrg/MyPublicProject --description "Public project" --source-control tfvc --visibility public

				# Create a project and return immediately without waiting for completion
				azdo project create MyOrg/MyAsyncProject --no-wait

				# Create a project and wait for a maximum of 5 minutes for completion
				azdo project create MyOrg/MyTimedProject --max-wait 300
			`),
		Args: cobra.ExactArgs(1),
		Aliases: []string{
			"cr",
			"c",
			"new",
			"n",
			"add",
			"a",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			noWaitChanged := cmd.Flags().Changed("no-wait")
			maxWaitChanged := cmd.Flags().Changed("max-wait")

			if noWaitChanged && maxWaitChanged {
				return util.FlagErrorf("--no-wait and --max-wait are mutually exclusive")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.project = args[0]

			return runCommand(ctx, o)
		},
	}

	cmd.Flags().StringVarP(&o.description, "description", "d", "", "Description for the new project")
	cmd.Flags().StringVarP(&o.process, "process", "p", "Agile", "Process to use (e.g., Scrum, Agile, CMMI)")
	cmd.Flags().StringVarP(&o.sourceCtrl, "source-control", "s", "git", "Source control type (git or tfvc)")
	cmd.Flags().StringVar(&o.visibility, "visibility", "private", "Project visibility (private or public)")
	cmd.Flags().BoolVar(&o.noWait, "no-wait", false, "Do not wait for the project to be created")
	cmd.Flags().IntVar(&o.maxWait, "max-wait", 3600, "Maximum wait time in seconds")

	util.AddJSONFlags(cmd, &o.exporter, []string{"operationID", "operationStatus", "operationURL", "id", "name", "state", "visibility", "process", "sourceControl"})

	return cmd
}

func runCommand(ctx util.CmdContext, o *opts) error {
	scope, err := util.ParseProjectScope(ctx, o.project)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	teamProject := &core.TeamProject{
		Name: types.ToPtr(scope.Project),
	}

	if o.description != "" {
		teamProject.Description = types.ToPtr(o.description)
	}

	var scType core.SourceControlTypes
	switch strings.ToLower(o.sourceCtrl) {
	case "git":
		scType = core.SourceControlTypesValues.Git
	case "tfvc":
		scType = core.SourceControlTypesValues.Tfvc
	default:
		return fmt.Errorf("invalid source control type: %s", o.sourceCtrl)
	}

	var vis core.ProjectVisibility
	switch o.visibility {
	case "private":
		vis = core.ProjectVisibilityValues.Private
	case "public":
		vis = core.ProjectVisibilityValues.Public
	default:
		return fmt.Errorf("invalid visibility: %s", o.visibility)
	}
	teamProject.Visibility = &vis

	capabilities := map[string]map[string]string{
		"versioncontrol": {
			"sourceControlType": string(scType),
		},
	}

	if o.process != "" {
		processes, err := coreClient.GetProcesses(ctx.Context(), core.GetProcessesArgs{})
		if err != nil {
			return fmt.Errorf("failed to get processes: %w", err)
		}

		var processID string
		for _, p := range *processes {
			if strings.EqualFold(*p.Name, o.process) {
				processID = p.Id.String()
				break
			}
		}

		if processID == "" {
			return fmt.Errorf("process '%s' not found", o.process)
		}

		capabilities["processTemplate"] = map[string]string{
			"templateTypeId": processID,
		}
	}
	teamProject.Capabilities = &capabilities

	opRef, err := coreClient.QueueCreateProject(ctx.Context(), core.QueueCreateProjectArgs{
		ProjectToCreate: teamProject,
	})
	if err != nil {
		return fmt.Errorf("failed to queue project creation: %w", err)
	}

	if o.noWait {
		ios.StopProgressIndicator()
		if o.exporter != nil {
			result := projectCreateResult{
				OperationID:  types.ToPtr(opRef.Id.String()),
				OperationURL: opRef.Url,
			}
			return o.exporter.Write(ios, result)
		}
		tp, err := ctx.Printer("list")
		if err != nil {
			return err
		}
		tp.AddColumns("ID", "Status", "URL")
		tp.EndRow()
		tp.AddField(opRef.Id.String(), nil)
		tp.AddField(string(*opRef.Status), nil)
		tp.AddField(*opRef.Url, nil)
		tp.EndRow()
		return tp.Render()
	}

	operationsClient, err := ctx.ClientFactory().Operations(ctx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	timeout := time.Duration(o.maxWait) * time.Second
	finalOp, err := azdo.PollOperationResult(ctx.Context(), operationsClient, opRef, timeout)
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	project, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId:           types.ToPtr(scope.Project),
		IncludeCapabilities: types.ToPtr(true),
	})
	if err != nil {
		return err
	}

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
		result := projectCreateResult{
			OperationID:     types.ToPtr(finalOp.Id.String()),
			OperationStatus: types.ToPtr(string(*finalOp.Status)),
			OperationURL:    finalOp.Url,
			ID:              types.ToPtr(project.Id.String()),
			Name:            project.Name,
			State:           types.ToPtr(string(*project.State)),
			Visibility:      types.ToPtr(string(*project.Visibility)),
			Process:         types.ToPtr(processName),
			SourceControl:   types.ToPtr(sourceControlType),
		}
		return o.exporter.Write(ios, result)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "Name", "State", "Visibility", "Process", "Source Control")
	tp.EndRow()
	tp.AddField(project.Id.String())
	tp.AddField(*project.Name, nil)
	tp.AddField(string(*project.State), nil)
	tp.AddField(string(*project.Visibility), nil)

	var processName, sourceControlType string
	if project.Capabilities != nil {
		if caps, ok := (*project.Capabilities)["processTemplate"]; ok {
			processName = caps["templateName"]
		}
		if caps, ok := (*project.Capabilities)["versioncontrol"]; ok {
			sourceControlType = caps["sourceControlType"]
		}
	}

	tp.AddField(processName, nil)
	tp.AddField(sourceControlType, nil)
	tp.EndRow()
	return tp.Render()
}
