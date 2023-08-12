package list

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/printer"
)

type listOptions struct {
	organizationName string
	limit            int
	state            string
	format           string
}

func NewCmdProjectList(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Short: "List the projects for an organization",
		Use:   "list",
		Example: heredoc.Doc(`
			# list the default organizations's projects
			azdo project list

			# list the projects for an Azure DevOps organization including closed projects
			azdo project list --organization myorg --closed
		`),
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.organizationName, "organization", "o", "", "Get per-organization configuration")
	util.StringEnumFlag(cmd, &opts.format, "format", "", "tsv", []string{"json"}, "Output format")
	util.StringEnumFlag(cmd, &opts.state, "state", "", "",
		[]string{
			string(core.ProjectStateValues.Deleting),
			string(core.ProjectStateValues.New),
			string(core.ProjectStateValues.WellFormed),
			string(core.ProjectStateValues.CreatePending),
			string(core.ProjectStateValues.All),
			string(core.ProjectStateValues.Unchanged),
			string(core.ProjectStateValues.Deleted),
		}, "Project state filter")
	cmd.Flags().IntVarP(&opts.limit, "limit", "l", 30, "Maximum number of projects to fetch")

	return cmd
}

func runList(ctx util.CmdContext, opts *listOptions) (err error) {
	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}

	var organizationName string
	if opts.organizationName != "" {
		organizationName = opts.organizationName
	} else {
		organizationName, _ = cfg.Authentication().GetDefaultOrganization()
	}
	if organizationName == "" {
		return util.FlagErrorf("no organization specified")
	}
	conn, err := ctx.Connection(organizationName)
	if err != nil {
		return
	}
	rctx, err := ctx.Context()
	if err != nil {
		return err
	}

	orgClient, err := core.NewClient(rctx, conn)
	if err != nil {
		return err
	}

	args := core.GetProjectsArgs{}
	if opts.state != "" {
		state := core.ProjectState(opts.state)
		args.StateFilter = &state
	}
	res, err := orgClient.GetProjects(rctx, args)
	if err != nil {
		return
	}
	if len(res.Value) == 0 {
		return util.NewNoResultsError(fmt.Sprintf("No projects found for organization %s", organizationName))
	}

	tp, err := ctx.Printer(opts.format)
	if err != nil {
		return
	}

	tp.AddColumns("ID", "Name", "State")
	for _, p := range res.Value {
		tp.AddField(p.Id.String(), printer.WithTruncate(nil))
		tp.AddField(*p.Name)
		tp.AddField(string(*p.State))
		tp.EndRow()
	}
	return tp.Render()
}
