package update

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type updateOptions struct {
	targetArg string

	title             string   // --title
	description       string   // --description (inline)
	descriptionFiles  []string // --description-file (repeatable; "-" reads stdin)
	descriptionEditor bool     // --description-editor
	descriptionFormat string   // --description-format (markdown|html)
	assignedTo        string
	state             string
	area              string
	iteration         string
	reason            string
	customFields      []string // --fields Ref.Name=value (repeatable)
	discussion        string

	bypassRules           bool
	suppressNotifications bool
	validateOnly          bool
	expand                string
	openInBrowser         bool

	exporter util.Exporter
}

type fieldKV struct{ ref, value string }

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &updateOptions{}

	cmd := &cobra.Command{
		Use:     "update [ORG:]PROJECT/ID",
		Short:   "Update a work item.",
		Aliases: []string{"u"},
		Long: heredoc.Doc(`
			Update one or more fields of an existing work item. The work item is
			identified by ID. Build a JSON Patch document from the supplied flags
			and send it to the server. At least one field flag is required.
		`),
		Example: heredoc.Doc(`
			# update a work item's title
			azdo boards work-item update Fabrikam/1234 --title "New title"

			# update description from a Markdown file
			azdo boards work-item update Fabrikam/1234 --description-file ./updated-repro.md

			# edit description in $EDITOR
			azdo boards work-item update Fabrikam/1234 --description-editor
		`),
		Args: util.ExactArgs(1, "project/work item target required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runUpdate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.title, "title", "", "New title of the work item.")
	cmd.Flags().StringVar(&opts.description, "description", "", "New description content (format set by --description-format; default markdown). Lower priority than --description-file and --description-editor.")
	cmd.Flags().StringArrayVar(&opts.descriptionFiles, "description-file", nil, "Read description from file (repeatable; \"-\" reads from stdin). Higher priority than --description.")
	cmd.Flags().BoolVar(&opts.descriptionEditor, "description-editor", false, "Edit description in $VISUAL/$EDITOR. Highest priority description source.")
	cmd.Flags().StringVar(&opts.descriptionFormat, "description-format", "markdown", "Format of the description input: \"markdown\" (default) or \"html\".")
	cmd.Flags().StringVar(&opts.assignedTo, "assigned-to", "", "Identity the work item is assigned to.")
	cmd.Flags().StringVar(&opts.state, "state", "", "New state of the work item.")
	cmd.Flags().StringVar(&opts.area, "area", "", "New area path of the work item.")
	cmd.Flags().StringVar(&opts.iteration, "iteration", "", "New iteration path of the work item.")
	cmd.Flags().StringVar(&opts.reason, "reason", "", "Reason for the change of state.")
	cmd.Flags().StringArrayVar(&opts.customFields, "fields", nil, "Set a field by reference name (repeatable; Ref.Name=value).")
	cmd.Flags().StringVar(&opts.discussion, "discussion", "", "Comment to add to the work item discussion.")
	cmd.Flags().BoolVar(&opts.bypassRules, "bypass-rules", false, "Do not enforce the work item type rules on this update.")
	cmd.Flags().BoolVar(&opts.suppressNotifications, "suppress-notifications", false, "Do not fire any notifications for this change.")
	cmd.Flags().BoolVar(&opts.validateOnly, "validate-only", false, "Only validate the changes without saving the work item.")
	cmd.Flags().StringVar(&opts.expand, "expand", "", "Expand parameters: None, Relations, Fields, Links, All.")
	cmd.Flags().BoolVar(&opts.openInBrowser, "open", false, "Open the updated work item in the default browser.")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"id", "rev", "fields", "url", "_links", "relations", "commentVersionRef"})

	return cmd
}

func runUpdate(cmdCtx util.CmdContext, opts *updateOptions) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	// Area/iteration paths are project-rooted tree paths with '\' separators
	// (e.g. "Fabrikam Fiber\Area\Voice"); normalize relative slash shorthand
	// (e.g. "Web/Payments") into the canonical form before patching.
	opts.area = shared.NormalizePath(scope.Project, opts.area)
	opts.iteration = shared.NormalizePath(scope.Project, opts.iteration)

	id, err := strconv.Atoi(scope.Targets[0])
	if err != nil || id <= 0 {
		return util.FlagErrorf("work item ID must be a positive integer; got %q", scope.Targets[0])
	}

	if opts.validateOnly {
		if opts.discussion != "" {
			return util.FlagErrorf("--discussion cannot be combined with --validate-only")
		}
		if opts.openInBrowser {
			return util.FlagErrorf("--open cannot be combined with --validate-only")
		}
	}

	formatOpValue, err := shared.NormalizeDescriptionFormat(opts.descriptionFormat)
	if err != nil {
		return err
	}

	zap.L().Debug(
		"resolved work item update target",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.Int("workItemId", id),
	)

	wit, err := cmdCtx.ClientFactory().WorkItemTracking(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create work item tracking client: %w", err)
	}

	item, err := wit.GetWorkItem(cmdCtx.Context(), workitemtracking.GetWorkItemArgs{
		Id:      &id,
		Project: types.ToPtr(scope.Project),
		Fields:  types.ToPtr([]string{shared.TeamProjectField}),
	})
	if err != nil {
		return fmt.Errorf("failed to fetch work item %d: %w", id, err)
	}
	if !shared.BelongsToProject(item, scope.Project) {
		return fmt.Errorf("work item %d does not belong to project %q", id, scope.Project)
	}

	editorCommand := ""
	if opts.descriptionEditor {
		cfg, err := cmdCtx.Config()
		if err != nil {
			return err
		}
		editorCommand, err = config.DetermineEditor(cfg)
		if err != nil {
			return err
		}
	}

	description, err := shared.ResolveDescription(ios, shared.DescriptionOptions{
		Inline:        opts.description,
		Files:         opts.descriptionFiles,
		Editor:        opts.descriptionEditor,
		EditorCommand: editorCommand,
	})
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	customFields, err := parseCustomFields(opts.customFields)
	if err != nil {
		return err
	}

	doc := buildPatchDocument(opts, description, customFields, formatOpValue)
	if len(doc) == 0 {
		return util.FlagErrorf("at least one field flag is required (e.g. --title, --state, --fields)")
	}
	args := workitemtracking.UpdateWorkItemArgs{
		Project:               types.ToPtr(scope.Project),
		Document:              &doc,
		Id:                    &id,
		ValidateOnly:          types.ToPtr(opts.validateOnly),
		BypassRules:           types.ToPtr(opts.bypassRules),
		SuppressNotifications: types.ToPtr(opts.suppressNotifications),
	}
	if opts.expand != "" {
		e := workitemtracking.WorkItemExpand(opts.expand)
		args.Expand = &e
	}

	res, err := wit.UpdateWorkItem(cmdCtx.Context(), args)
	if err != nil {
		return fmt.Errorf("failed to update work item %d: %w", id, err)
	}
	if res == nil {
		return fmt.Errorf("failed to update work item %d: server returned an empty response", id)
	}

	if opts.discussion != "" {
		if _, err := wit.AddComment(cmdCtx.Context(), workitemtracking.AddCommentArgs{
			Project:    types.ToPtr(scope.Project),
			WorkItemId: types.ToPtr(id),
			Request:    &workitemtracking.CommentCreate{Text: types.ToPtr(opts.discussion)},
		}); err != nil {
			return fmt.Errorf("failed to add discussion comment to work item %d: %w", id, err)
		}
	}

	ios.StopProgressIndicator()

	if opts.bypassRules || opts.suppressNotifications {
		fmt.Fprintf(ios.ErrOut, "warning: --bypass-rules/--suppress-notifications bypass work item type rules and notifications\n")
	}

	if opts.exporter != nil {
		if err := opts.exporter.Write(ios, res); err != nil {
			return err
		}
	} else {
		tp, err := cmdCtx.Printer("list")
		if err != nil {
			return err
		}
		tp.AddColumns("ID", "TYPE", "STATE", "TITLE", "ASSIGNED TO", "AREA", "ITERATION")
		fields := types.GetValue(res.Fields, map[string]any{})
		tp.AddField(strconv.Itoa(types.GetValue(res.Id, 0)))
		tp.AddField(shared.FieldString(fields, "System.WorkItemType"))
		tp.AddField(shared.FieldString(fields, "System.State"))
		tp.AddField(shared.FieldString(fields, "System.Title"))
		tp.AddField(shared.FieldIdentityDisplay(fields, "System.AssignedTo"))
		tp.AddField(shared.FieldString(fields, "System.AreaPath"))
		tp.AddField(shared.FieldString(fields, "System.IterationPath"))
		tp.EndRow()
		if err := tp.Render(); err != nil {
			return err
		}
	}

	if opts.openInBrowser {
		if err := shared.OpenURL(types.GetValue(res.Url, "")); err != nil {
			return fmt.Errorf("failed to open work item in browser: %w", err)
		}
	}
	return nil
}

// buildPatchDocument appends ops in a fixed order: Title, Description (plus
// the multiline format op), AssignedTo, State, AreaPath, IterationPath,
// Reason, then raw --fields ops in user-given order. Tests assert this order.
func buildPatchDocument(opts *updateOptions, description string, customFields []fieldKV, formatOpValue string) []webapi.JsonPatchOperation {
	add := webapi.OperationValues.Add
	doc := []webapi.JsonPatchOperation{}
	patch := func(path string, value any) {
		p := path
		doc = append(doc, webapi.JsonPatchOperation{Op: &add, Path: &p, Value: value})
	}

	if opts.title != "" {
		patch("/fields/System.Title", opts.title)
	}
	if description != "" {
		patch("/fields/System.Description", description)
		patch("/multilineFieldsFormat/System.Description", formatOpValue)
	}
	if opts.assignedTo != "" {
		patch("/fields/System.AssignedTo", opts.assignedTo)
	}
	if opts.state != "" {
		patch("/fields/System.State", opts.state)
	}
	if opts.area != "" {
		patch("/fields/System.AreaPath", opts.area)
	}
	if opts.iteration != "" {
		patch("/fields/System.IterationPath", opts.iteration)
	}
	if opts.reason != "" {
		patch("/fields/System.Reason", opts.reason)
	}
	for _, f := range customFields {
		patch("/fields/"+f.ref, f.value)
	}
	return doc
}

// parseCustomFields splits each Ref.Name=value on the first "=" only.
func parseCustomFields(raw []string) ([]fieldKV, error) {
	fields := make([]fieldKV, 0, len(raw))
	for _, r := range raw {
		ref, value, ok := strings.Cut(r, "=")
		if !ok {
			return nil, util.FlagErrorf("--fields value %q must be in the form Ref.Name=value", r)
		}
		fields = append(fields, fieldKV{ref: ref, value: value})
	}
	return fields, nil
}
