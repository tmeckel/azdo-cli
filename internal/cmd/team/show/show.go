package show

import (
	_ "embed"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

type showOptions struct {
	targetArg string
	exporter  util.Exporter
}

//go:embed show.tpl
var showTpl string

type teamTemplateData struct {
	Id          string
	Name        string
	Description string
	ProjectId   string
	ProjectName string
	Url         string
	Identity    string
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]PROJECT/TEAM",
		Short: "Show details of a team.",
		Long: heredoc.Doc(`
			Show details of a single team in a project. The team is identified by its
			name or GUID inside the project. The organization falls back to the
			configured default when omitted.
		`),
		Example: heredoc.Doc(`
			# Show a team by name in the default organization
			azdo team show Fabrikam/"Fabrikam Engineering"

			# Show a team by ID in a specific organization
			azdo team show MyOrg/Fabrikam/00000002-0000-0000-0000-000000000000
		`),
		Aliases: []string{"s"},
		Args:    util.ExactArgs(1, "team argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runShow(ctx, opts)
		},
	}

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "name", "description", "url",
		"identity", "identityUrl", "projectId", "projectName",
	})

	return cmd
}

func runShow(ctx util.CmdContext, opts *showOptions) error {
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

	zap.L().Debug(
		"show team",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("teamId", scope.Targets[0]),
	)

	client, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Core client: %w", err)
	}

	team, err := client.GetTeam(ctx.Context(), core.GetTeamArgs{
		ProjectId: &scope.Project,
		TeamId:    &scope.Targets[0],
	})
	if err != nil {
		return fmt.Errorf("failed to get team: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, team)
	}

	return renderTeam(ctx, ios, team)
}

func renderTeam(ctx util.CmdContext, ios *iostreams.IOStreams, team *core.WebApiTeam) error {
	data := teamTemplateData{
		Id:          types.GetValue(team.Id, uuid.UUID{}).String(),
		Name:        types.GetValue(team.Name, ""),
		Description: types.GetValue(team.Description, ""),
		ProjectId:   types.GetValue(team.ProjectId, uuid.UUID{}).String(),
		ProjectName: types.GetValue(team.ProjectName, ""),
		Url:         types.GetValue(team.Url, ""),
		Identity:    formatIdentity(team.Identity),
	}

	t := template.New(
		ios.Out,
		ios.TerminalWidth(),
		ios.ColorEnabled(),
	).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"s":       template.StringOrEmpty,
			"hasText": template.HasText,
		})

	if err := t.Parse(showTpl); err != nil {
		return err
	}

	return t.ExecuteData(data)
}

func formatIdentity(ident *identity.Identity) string {
	if ident == nil {
		return ""
	}
	displayName := types.GetValue(ident.ProviderDisplayName, "")
	descriptor := types.GetValue(ident.Descriptor, "")
	if displayName != "" && descriptor != "" {
		return fmt.Sprintf("%s (%s)", displayName, descriptor)
	}
	if displayName != "" {
		return displayName
	}
	return descriptor
}
