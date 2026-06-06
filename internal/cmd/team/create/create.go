package create

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type createOptions struct {
	scopeArg    string
	name        string
	description string
	exporter    util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:   "create [ORGANIZATION/]PROJECT",
		Short: "Create a new team in a project.",
		Long: heredoc.Doc(`
			Create a new team in the specified project. The --name flag is required.
			The project argument is required; the organization falls back to the
			configured default when omitted.
		`),
		Example: heredoc.Doc(`
			# Create a team in the default organization
			azdo team create Fabrikam --name "Fabrikam Engineering"

			# Create a team with a description
			azdo team create MyOrg/Fabrikam --name "My Team" --description "Owns the web app"
		`),
		Aliases: []string{
			"c",
			"cr",
			"new",
			"n",
			"add",
			"a",
		},
		Args: util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runCreate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Name of the new team (required)")
	cmd.Flags().StringVar(&opts.description, "description", "", "Description of the new team")
	_ = cmd.MarkFlagRequired("name")

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "name", "description", "url",
		"identity", "identityUrl", "projectId", "projectName",
	})

	return cmd
}

func runCreate(ctx util.CmdContext, opts *createOptions) error {
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

	team := &core.WebApiTeam{
		Name:        types.ToPtr(opts.name),
		Description: types.ToPtr(opts.description),
	}

	created, err := client.CreateTeam(ctx.Context(), core.CreateTeamArgs{
		Team:      team,
		ProjectId: &scope.Project,
	})
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, created)
	}

	return renderTeam(ctx, created)
}

func renderTeam(ctx util.CmdContext, team *core.WebApiTeam) error {
	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("ID", "NAME", "DESCRIPTION", "PROJECT", "URL")
	tp.EndRow()
	tp.AddField(types.GetValue(team.Id, uuid.UUID{}).String())
	tp.AddField(types.GetValue(team.Name, ""))
	tp.AddField(types.GetValue(team.Description, ""))
	tp.AddField(types.GetValue(team.ProjectName, ""))
	tp.AddField(types.GetValue(team.Url, ""))
	tp.EndRow()
	return tp.Render()
}
