package delete

import (
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deleteOptions struct {
	project  string
	yes      bool
	noWait   bool
	maxWait  int
	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Short: "Delete a project",
		Use:   "delete [ORGANIZATION/]PROJECT",
		Example: heredoc.Doc(
			`# delete a project in the default organization
			azdo project delete myproject

			# delete a project in a specific organization
			azdo project delete myorg/myproject`,
		),
		Args: util.ExactArgs(1, "project name required"),
		Aliases: []string{
			"d",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.project = args[0]
			return runDelete(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&opts.noWait, "no-wait", false, "Do not wait for the project deletion to complete")
	cmd.Flags().IntVar(&opts.maxWait, "max-wait", 3600, "Maximum wait time in seconds")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"ID", "Status", "Url"})

	return cmd
}

func runDelete(ctx util.CmdContext, opts *deleteOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	cs := ios.ColorScheme()

	p, err := ctx.Prompter()
	if err != nil {
		return err
	}

	scope, err := util.ParseProjectScope(ctx, opts.project)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	// Confirm
	if !opts.yes {
		confirmed, err := p.Confirm(fmt.Sprintf("Delete project %s?", cs.Bold(fmt.Sprintf("%s/%s", scope.Organization, scope.Project))), false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	// Get project details to resolve name to ID
	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	projectDetails, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId: types.ToPtr(scope.Project),
	})
	if err != nil {
		return err
	}

	// Queue delete
	op, err := coreClient.QueueDeleteProject(ctx.Context(), core.QueueDeleteProjectArgs{
		ProjectId: projectDetails.Id,
	})
	if err != nil {
		return err
	}

	if opts.noWait {
		ios.StopProgressIndicator()

		if opts.exporter != nil {
			jd := struct {
				ID     string `json:"ID"`
				Status string `json:"Status"`
				Url    string `json:"Url"`
			}{
				ID:     op.Id.String(),
				Status: string(*op.Status),
				Url:    *op.Url,
			}
			return opts.exporter.Write(ios, jd)
		}

		// Plain output
		fmt.Fprintf(ios.Out, "%s Project %s deletion queued. Operation ID: %s\n", cs.SuccessIcon(), cs.Bold(fmt.Sprintf("%s/%s", scope.Organization, scope.Project)), op.Id.String())

		return nil
	}

	// Wait for completion
	operationsClient, err := ctx.ClientFactory().Operations(ctx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	timeout := time.Duration(opts.maxWait) * time.Second
	finalOp, err := azdo.PollOperationResult(ctx.Context(), operationsClient, op, timeout)
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		jd := struct {
			ID     string `json:"ID"`
			Status string `json:"Status"`
			Url    string `json:"Url"`
		}{
			ID:     finalOp.Id.String(),
			Status: string(*finalOp.Status),
			Url:    *finalOp.Url,
		}
		return opts.exporter.Write(ios, jd)
	}

	// Plain output
	fmt.Fprintf(ios.Out, "%s Project %s deleted successfully.\n", cs.SuccessIcon(), cs.Bold(fmt.Sprintf("%s/%s", scope.Organization, scope.Project)))

	return nil
}
