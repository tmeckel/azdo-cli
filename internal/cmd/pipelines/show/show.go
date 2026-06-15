package show

import (
	_ "embed"
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type showOptions struct {
	exporter util.Exporter
	scopeArg string
}

//go:embed show.tpl
var showTmpl string

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]PROJECT/PIPELINE",
		Short: "Show details of a pipeline definition",
		Long: heredoc.Doc(`
			Display the details of a single Azure Pipelines definition.

			The pipeline may be specified by ID (integer) or name (string).
			When the organization segment is omitted the default organization
			from configuration is used.
		`),
		Example: heredoc.Doc(`
			# Show a pipeline by ID using the default organization
			azdo pipelines show Fabrikam/42

			# Show a pipeline by name
			azdo pipelines show Fabrikam/My Pipeline

			# Show with explicit organization
			azdo pipelines show MyOrg/Fabrikam/42

			# Export as JSON
			azdo pipelines show Fabrikam/42 --json id,name,revision
		`),
		Aliases: []string{"view", "status"},
		Args:    util.ExactArgs(1, "pipeline target is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runShow(ctx, opts)
		},
	}

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "name", "revision", "description", "path", "type", "url", "_links",
		"process", "repository", "queue", "authoredBy", "createdDate", "quality",
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

	scope, err := util.ParseProjectTargetWithDefaultOrganization(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	clientFact := ctx.ClientFactory()

	buildClient, err := clientFact.Build(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Build client: %w", err)
	}

	logger := zap.L().With(
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("pipeline", scope.Targets[0]),
	)

	pipelineID, err := shared.ResolvePipelineDefinition(ctx, buildClient, scope.Project, scope.Targets[0])
	if err != nil {
		return err
	}

	logger.Debug("fetching pipeline definition", zap.Int("pipelineId", pipelineID))

	definition, err := buildClient.GetDefinition(ctx.Context(), build.GetDefinitionArgs{
		Project:      types.ToPtr(scope.Project),
		DefinitionId: types.ToPtr(pipelineID),
	})
	if err != nil {
		return fmt.Errorf("failed to fetch pipeline definition: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, definition)
	}

	t := template.New(
		ios.Out,
		ios.TerminalWidth(),
		ios.ColorEnabled(),
	).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"formatEntity": func(primary, secondary any) string {
				first := template.StringOrEmpty(primary)
				second := template.StringOrEmpty(secondary)
				switch {
				case first != "" && second != "":
					return fmt.Sprintf("%s (%s)", first, second)
				case first != "":
					return first
				default:
					return second
				}
			},
			"identityDisplay": func(id *webapi.IdentityRef) string {
				if id == nil {
					return ""
				}
				display := types.GetValue(id.DisplayName, "")
				unique := types.GetValue(id.UniqueName, "")
				switch {
				case display != "" && unique != "":
					return fmt.Sprintf("%s (%s)", display, unique)
				case display != "":
					return display
				default:
					return unique
				}
			},
			"hasItems": template.HasItems,
			"hasText":  template.HasText,
			"s":        template.StringOrEmpty,
			"int": func(v *int) string {
				if v == nil {
					return ""
				}
				return strconv.Itoa(*v)
			},
		})

	if err := t.Parse(showTmpl); err != nil {
		return err
	}

	return t.ExecuteData(*definition)
}
