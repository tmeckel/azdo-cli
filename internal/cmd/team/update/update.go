package update

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type updateOptions struct {
	targetArg   string
	name        string
	description string
	exporter    util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &updateOptions{}

	cmd := &cobra.Command{
		Use:   "update [ORGANIZATION/]PROJECT/TEAM",
		Short: "Update a team's name and/or description.",
		Long: heredoc.Doc(`
			Update a team's name and/or description. At least one of --name or
			--description must be provided. The team is identified by its name or
			GUID inside the project.
		`),
		Example: heredoc.Doc(`
			# Rename a team
			azdo team update Fabrikam/"Old Name" --name "New Name"

			# Update a team's description only
			azdo team update MyOrg/Fabrikam/MyTeam --description "New description"
		`),
		Aliases: []string{
			"u",
		},
		Args: util.ExactArgs(1, "team argument required"),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			nameChanged := cmd.Flags().Changed("name")
			descChanged := cmd.Flags().Changed("description")
			if !nameChanged && !descChanged {
				return util.FlagErrorf("at least one of --name or --description is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runUpdate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "New name of the team")
	cmd.Flags().StringVar(&opts.description, "description", "", "New description of the team")

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "name", "description", "url",
		"identity", "identityUrl", "projectId", "projectName",
	})

	return cmd
}

func runUpdate(ctx util.CmdContext, opts *updateOptions) error {
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

	client, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Core client: %w", err)
	}

	payload := &core.WebApiTeam{
		Name:        types.ToPtr(opts.name),
		Description: types.ToPtr(opts.description),
	}

	updated, err := client.UpdateTeam(ctx.Context(), core.UpdateTeamArgs{
		TeamData:  payload,
		ProjectId: &scope.Project,
		TeamId:    &scope.Targets[0],
	})
	if err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, updated)
	}

	return renderTeam(ctx, updated)
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
