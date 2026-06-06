package delete

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type deleteOptions struct {
	targetArg string
	yes       bool
	exporter  util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete [ORGANIZATION/]PROJECT/TEAM",
		Short: "Delete a team.",
		Long: heredoc.Doc(`
			Delete a team from a project.

			The TEAM argument accepts the ID (GUID) or name of the team.
			A confirmation prompt is shown unless --yes is provided.
		`),
		Example: heredoc.Doc(`
			# Delete a team (with confirmation)
			azdo team delete Fabrikam/"Old Team"

			# Delete a team without confirmation
			azdo team delete MyOrg/Fabrikam/00000002-0000-0000-0000-000000000000 --yes
		`),
		Aliases: []string{"d", "del", "rm"},
		Args:    util.ExactArgs(1, "team argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runDelete(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompt")

	util.AddJSONFlags(cmd, &opts.exporter, []string{})

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

	scope, err := util.ParseProjectTargetWithDefaultOrganization(ctx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	teamID := scope.Targets[0]

	if !opts.yes {
		confirmed, err := p.Confirm(fmt.Sprintf("Delete team %s from project %s?",
			cs.Bold(teamID), cs.Bold(fmt.Sprintf("%s/%s", scope.Organization, scope.Project))), false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	client, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Core client: %w", err)
	}

	if err := client.DeleteTeam(ctx.Context(), core.DeleteTeamArgs{
		ProjectId: &scope.Project,
		TeamId:    &teamID,
	}); err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, struct{}{})
	}

	if ios.IsStdoutTTY() {
		fmt.Fprintf(ios.Out, "%s Team %s deleted successfully.\n", cs.SuccessIcon(), cs.Bold(teamID))
	}

	return nil
}
