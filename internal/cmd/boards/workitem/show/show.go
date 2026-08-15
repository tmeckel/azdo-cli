package show

import (
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type showOptions struct {
	scopeArg      string
	showComments  bool
	showRelations bool
	exporter      util.Exporter
}

//go:embed show.tpl
var showTpl string

type templateData struct {
	WorkItem    *workitemtracking.WorkItem
	Comments    *[]workitemtracking.Comment
	Relations   bool
	Description string
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:     "show [ORG:]PROJECT/ID",
		Short:   "Show work item details",
		Aliases: []string{"view", "status"},
		Long: heredoc.Doc(`
			Display the details of a single Azure Boards work item by its integer
			ID. The work item is fetched with Expand=All so relations, fields and
			links are returned in one call. The description is rendered
			format-aware: Markdown content is passed through, Html content is
			converted to Markdown first.
		`),
		Example: heredoc.Doc(`
			# Show work item 12345 in the default organization's Fabrikam project
			azdo boards work-item show Fabrikam/12345

			# Show a work item in a specific organization
			azdo boards work-item show myorg:Fabrikam/12345

			# Include the work item's comment thread and relations
			azdo boards work-item show Fabrikam/12345 --comments --relations

			# Export the work item as JSON
			azdo boards work-item show Fabrikam/12345 --json
		`),
		Args: util.ExactArgs(1, "project/work item target required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runShow(ctx, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.showComments, "comments", false, "Fetch and render the work item's comment thread")
	cmd.Flags().BoolVar(&opts.showRelations, "relations", false, "Render the work item's relations block")

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "rev", "fields", "relations", "url", "_links", "commentVersionRef",
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
	workItemID, err := parseWorkItemID(scope.Targets[0])
	if err != nil {
		return err
	}

	expand := workitemtracking.WorkItemExpandValues.All
	zap.L().Debug(
		"fetching work item",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.Int("workItemId", workItemID),
		zap.String("expand", string(expand)),
		zap.Bool("fetchComments", opts.showComments),
	)

	wit, err := ctx.ClientFactory().WorkItemTracking(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create work item tracking client: %w", err)
	}

	res, err := wit.GetWorkItem(ctx.Context(), workitemtracking.GetWorkItemArgs{
		Project: types.ToPtr(scope.Project),
		Id:      types.ToPtr(workItemID),
		Expand:  &expand,
	})
	if err != nil {
		return fmt.Errorf("failed to get work item: %w", err)
	}
	if res == nil {
		return errors.New("work item tracking API returned an empty response")
	}
	if !shared.BelongsToProject(res, scope.Project) {
		return fmt.Errorf("work item %d does not belong to project %q", workItemID, scope.Project)
	}

	var comments *[]workitemtracking.Comment
	if opts.showComments {
		commentList, err := wit.GetComments(ctx.Context(), workitemtracking.GetCommentsArgs{
			Project:    types.ToPtr(scope.Project),
			WorkItemId: types.ToPtr(workItemID),
		})
		if err != nil {
			return fmt.Errorf("failed to get work item comments: %w", err)
		}
		if commentList != nil {
			comments = commentList.Comments
		}
	}

	if opts.exporter != nil {
		ios.StopProgressIndicator()
		return opts.exporter.Write(ios, res)
	}

	// The vendored SDK WorkItem model drops /multilineFieldsFormat, so the
	// raw payload is fetched a second time to learn the description format.
	// ponytail: two GETs per show; ceiling: upstream SDK gains the property,
	// then this call and GetWorkItemEnvelope can be deleted.
	envelope, err := fetchWorkItemEnvelope(ctx, scope, workItemID)
	if err != nil {
		return err
	}
	description, err := descriptionMarkdown(envelope)
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	t := template.New(
		ios.Out,
		ios.TerminalWidth(),
		ios.ColorEnabled(),
	).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"field": func(wi *workitemtracking.WorkItem, key string) string {
				return shared.FieldString(types.GetValue(wi.Fields, map[string]any{}), key)
			},
			"identity": func(wi *workitemtracking.WorkItem, key string) string {
				return shared.FieldIdentityDisplay(types.GetValue(wi.Fields, map[string]any{}), key)
			},
			"unique": func(wi *workitemtracking.WorkItem, key string) string {
				fields := types.GetValue(wi.Fields, map[string]any{})
				v, ok := fields[key]
				if !ok || v == nil {
					return ""
				}
				if m, ok := v.(map[string]any); ok {
					if uniqueName, ok := m["uniqueName"].(string); ok {
						return uniqueName
					}
					if displayName, ok := m["displayName"].(string); ok {
						return displayName
					}
					return ""
				}
				return fmt.Sprint(v)
			},
			"cdate": func(c *workitemtracking.Comment) string {
				if c == nil || c.CreatedDate == nil {
					return ""
				}
				return c.CreatedDate.Time.Format(time.RFC3339)
			},
			"cauthor": func(c *workitemtracking.Comment) string {
				if c == nil || c.CreatedBy == nil {
					return ""
				}
				return template.StringOrEmpty(c.CreatedBy.DisplayName)
			},
			"hasText": template.HasText,
			"s":       template.StringOrEmpty,
		})

	if err := t.Parse(showTpl); err != nil {
		return err
	}

	return t.ExecuteData(templateData{
		WorkItem:    res,
		Comments:    comments,
		Relations:   opts.showRelations,
		Description: description,
	})
}

// parseWorkItemID validates the positional target segment as a positive
// integer work item ID.
func parseWorkItemID(raw string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil {
		return 0, util.FlagErrorf("work item id must be a positive integer, got %q", raw)
	}
	if id <= 0 {
		return 0, util.FlagErrorf("work item id must be a positive integer, got %d", id)
	}
	return id, nil
}

// fetchWorkItemEnvelope resolves the low-level client for the organization
// and fetches the raw work item payload, capturing the multilineFieldsFormat
// map the SDK model drops.
func fetchWorkItemEnvelope(ctx util.CmdContext, scope *util.Path, id int) (*azdo.WorkItemEnvelope, error) {
	cfg, err := ctx.Config()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	orgURL, err := cfg.Authentication().GetURL(scope.Organization)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve organization URL: %w", err)
	}
	conn, err := ctx.ConnectionFactory().Connection(scope.Organization)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}
	client := conn.GetClientByUrl(strings.TrimRight(orgURL, "/"))
	expand := workitemtracking.WorkItemExpandValues.All
	return azdo.GetWorkItemEnvelope(ctx.Context(), client, scope.Project, id, &expand)
}

// descriptionMarkdown returns the work item description as Markdown ready for
// the markdown template func. Markdown-format descriptions pass through
// unconverted; Html-format descriptions (and legacy items without a format
// map) are converted first so literal tags never reach glamour.
func descriptionMarkdown(envelope *azdo.WorkItemEnvelope) (string, error) {
	fields := types.GetValue(envelope.Fields, map[string]any{})
	raw := shared.FieldString(fields, "System.Description")
	if raw == "" {
		return "", nil
	}
	if envelope.MultilineFieldsFormat != nil {
		if format, ok := (*envelope.MultilineFieldsFormat)["System.Description"]; ok && strings.EqualFold(format, "Markdown") {
			return raw, nil
		}
	}
	markdown, err := htmltomarkdown.ConvertString(raw)
	if err != nil {
		return "", fmt.Errorf("failed to convert html description to markdown: %w", err)
	}
	return markdown, nil
}
