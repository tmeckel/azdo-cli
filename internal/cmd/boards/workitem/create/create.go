package create

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type createOptions struct {
	scopeArg string

	workItemType string // --type
	title        string // --title

	description       string   // --description (inline)
	descriptionFiles  []string // --description-file (repeatable; "-" reads stdin)
	descriptionEditor bool     // --description-editor
	descriptionFormat string   // --description-format (markdown|html)
	assignedTo        string
	area              string
	iteration         string
	tags              []string
	priority          int
	prioritySet       bool // --priority explicitly provided (allows sending 0)
	severity          string
	parent            int
	parentSet         bool // --parent explicitly provided (rejects non-positive values)
	reason            string
	state             string
	customFields      []string // --fields Ref.Name=value
	links             []string // --link rel,url

	bypassRules           bool
	suppressNotifications bool
	validateOnly          bool
	expand                string
	openInBrowser         bool
	discussion            string

	exporter util.Exporter
}

type (
	fieldKV struct{ ref, value string }
	linkKV  struct{ rel, url string }
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:     "create [ORG:]PROJECT",
		Short:   "Create a work item in a project.",
		Aliases: []string{"c", "cr"},
		Long: heredoc.Doc(`
			Create a work item of a given type in a project within an Azure DevOps
			organization. The work item is built from a JSON Patch document assembled
			from the supplied flags.
		`),
		Example: heredoc.Doc(`
			# create a bug in the default org's Fabrikam project
			azdo boards work-item create Fabrikam --type Bug --title "Login is broken"

			# create from a description file
			azdo boards work-item create Fabrikam --type Bug --title "Login broken" --description-file ./repro.md

			# open the description in the user's $EDITOR
			azdo boards work-item create Fabrikam --type Bug --title "Login broken" --description-editor
		`),
		Args: util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			opts.prioritySet = cmd.Flags().Changed("priority")
			opts.parentSet = cmd.Flags().Changed("parent")
			return runCreate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.workItemType, "type", "", "Work item type to create (e.g. Bug, Task, User Story).")
	cmd.Flags().StringVar(&opts.title, "title", "", "Title of the new work item.")
	cmd.Flags().StringVar(&opts.description, "description", "", "Description content (format set by --description-format; default markdown). Lower priority than --description-file and --description-editor.")
	cmd.Flags().StringArrayVar(&opts.descriptionFiles, "description-file", nil, "Read description from file (repeatable; \"-\" reads from stdin). Higher priority than --description.")
	cmd.Flags().BoolVar(&opts.descriptionEditor, "description-editor", false, "Edit description in $VISUAL/$EDITOR. Highest priority description source.")
	cmd.Flags().StringVar(&opts.descriptionFormat, "description-format", "markdown", "Format of the description input: \"markdown\" (default) or \"html\".")
	cmd.Flags().StringVar(&opts.assignedTo, "assigned-to", "", "Identity the work item is assigned to.")
	cmd.Flags().StringVar(&opts.area, "area", "", "Area path of the new work item.")
	cmd.Flags().StringVar(&opts.iteration, "iteration", "", "Iteration path of the new work item.")
	cmd.Flags().StringArrayVar(&opts.tags, "tag", nil, "Tag to add (repeatable); values are joined with '; '.")
	cmd.Flags().IntVar(&opts.priority, "priority", 0, "Priority of the new work item (numeric value defined by the work item process).")
	cmd.Flags().StringVar(&opts.severity, "severity", "", "Severity of the new work item (value defined by the work item process, e.g. 1 - Critical).")
	cmd.Flags().IntVar(&opts.parent, "parent", 0, "ID of the parent work item; adds a System.LinkTypes.Hierarchy-Reverse link.")
	cmd.Flags().StringVar(&opts.reason, "reason", "", "Reason for the state of the new work item.")
	cmd.Flags().StringVar(&opts.state, "state", "", "Initial state of the new work item.")
	cmd.Flags().StringArrayVar(&opts.customFields, "fields", nil, "Set a field by reference name (repeatable; Ref.Name=value).")
	cmd.Flags().StringArrayVar(&opts.links, "link", nil, "Add a link to the new work item (repeatable; rel,url).")
	cmd.Flags().BoolVar(&opts.bypassRules, "bypass-rules", false, "Do not enforce the work item type rules on this create.")
	cmd.Flags().BoolVar(&opts.suppressNotifications, "suppress-notifications", false, "Do not fire any notifications for this create.")
	cmd.Flags().BoolVar(&opts.validateOnly, "validate-only", false, "Only validate the create without saving the work item.")
	cmd.Flags().StringVar(&opts.expand, "expand", "", "Expand parameters: None, Relations, Fields, Links, All.")
	cmd.Flags().BoolVar(&opts.openInBrowser, "open", false, "Open the created work item in the default browser.")
	cmd.Flags().StringVar(&opts.discussion, "discussion", "", "Comment to add to the new work item's discussion.")

	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("title")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"id", "rev", "fields", "url", "_links", "relations", "commentVersionRef"})

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

	// Area/iteration paths are project-rooted tree paths with '\' separators
	// (e.g. "Fabrikam Fiber\Area\Voice"); normalize relative slash shorthand
	// (e.g. "Web/Payments") into the canonical form before patching.
	opts.area = shared.NormalizePath(scope.Project, opts.area)
	opts.iteration = shared.NormalizePath(scope.Project, opts.iteration)

	if opts.parentSet && opts.parent <= 0 {
		return util.FlagErrorf("--parent must be a positive work item ID, got %d", opts.parent)
	}

	if opts.validateOnly {
		if opts.discussion != "" {
			return util.FlagErrorf("--discussion cannot be combined with --validate-only")
		}
		if opts.openInBrowser {
			return util.FlagErrorf("--open cannot be combined with --validate-only")
		}
	}

	if err := shared.ValidateTags("--tag", opts.tags); err != nil {
		return err
	}

	formatOpValue, err := shared.NormalizeDescriptionFormat(opts.descriptionFormat)
	if err != nil {
		return err
	}

	customFields, err := parseCustomFields(opts.customFields)
	if err != nil {
		return err
	}
	links, err := parseLinks(opts.links)
	if err != nil {
		return err
	}

	if opts.parentSet {
		cfg, err := ctx.Config()
		if err != nil {
			return err
		}
		orgURL, err := cfg.Authentication().GetURL(scope.Organization)
		if err != nil {
			return fmt.Errorf("failed to resolve organization URL: %w", err)
		}
		parentURL := fmt.Sprintf("%s/_apis/wit/workItems/%d", strings.TrimRight(orgURL, "/"), opts.parent)
		links = append([]linkKV{{rel: "System.LinkTypes.Hierarchy-Reverse", url: parentURL}}, links...)
	}

	editorCommand := ""
	if opts.descriptionEditor {
		cfg, err := ctx.Config()
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

	doc := buildPatchDocument(opts, description, customFields, links, formatOpValue)
	args := workitemtracking.CreateWorkItemArgs{
		Document:              &doc,
		Project:               types.ToPtr(scope.Project),
		Type:                  types.ToPtr(opts.workItemType),
		ValidateOnly:          types.ToPtr(opts.validateOnly),
		BypassRules:           types.ToPtr(opts.bypassRules),
		SuppressNotifications: types.ToPtr(opts.suppressNotifications),
	}
	if opts.expand != "" {
		e := workitemtracking.WorkItemExpand(opts.expand)
		args.Expand = &e
	}

	wit, err := ctx.ClientFactory().WorkItemTracking(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create work item tracking client: %w", err)
	}

	res, err := wit.CreateWorkItem(ctx.Context(), args)
	if err != nil {
		return err
	}
	if res == nil {
		return errors.New("work item tracking API returned an empty response")
	}

	if opts.discussion != "" {
		if _, err := wit.AddComment(ctx.Context(), workitemtracking.AddCommentArgs{
			Project:    types.ToPtr(scope.Project),
			WorkItemId: res.Id,
			Request:    &workitemtracking.CommentCreate{Text: types.ToPtr(opts.discussion)},
		}); err != nil {
			return fmt.Errorf("failed to add discussion comment: %w", err)
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
		if opts.openInBrowser {
			return openWorkItem(res)
		}
		return nil
	}

	tp, err := ctx.Printer("list")
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

	if opts.openInBrowser {
		return openWorkItem(res)
	}
	return nil
}

// openWorkItem opens the created work item's URL in the default browser.
func openWorkItem(res *workitemtracking.WorkItem) error {
	if err := shared.OpenURL(types.GetValue(res.Url, "")); err != nil {
		return fmt.Errorf("failed to open work item in browser: %w", err)
	}
	return nil
}

// buildPatchDocument appends ops in a fixed order: Title, Description (plus
// the multiline format op), AssignedTo, AreaPath, IterationPath, Tags,
// Priority, Severity, Reason, State, then raw --fields and --link ops in
// user-given order (the --parent relation is folded into --link by
// runCreate). Tests assert this order.
func buildPatchDocument(opts *createOptions, description string, customFields []fieldKV, links []linkKV, formatOpValue string) []webapi.JsonPatchOperation {
	add := webapi.OperationValues.Add
	doc := []webapi.JsonPatchOperation{}
	patch := func(path string, value any) {
		p := path
		doc = append(doc, webapi.JsonPatchOperation{Op: &add, Path: &p, Value: value})
	}

	patch("/fields/System.Title", opts.title)
	if description != "" {
		patch("/fields/System.Description", description)
		patch("/multilineFieldsFormat/System.Description", formatOpValue)
	}
	if opts.assignedTo != "" {
		patch("/fields/System.AssignedTo", opts.assignedTo)
	}
	if opts.area != "" {
		patch("/fields/System.AreaPath", opts.area)
	}
	if opts.iteration != "" {
		patch("/fields/System.IterationPath", opts.iteration)
	}
	if len(opts.tags) > 0 {
		patch("/fields/System.Tags", strings.Join(opts.tags, "; "))
	}
	if opts.prioritySet {
		patch("/fields/Microsoft.VSTS.Common.Priority", opts.priority)
	}
	if opts.severity != "" {
		patch("/fields/Microsoft.VSTS.Common.Severity", opts.severity)
	}
	if opts.reason != "" {
		patch("/fields/System.Reason", opts.reason)
	}
	if opts.state != "" {
		patch("/fields/System.State", opts.state)
	}
	for _, f := range customFields {
		patch("/fields/"+f.ref, f.value)
	}
	for _, l := range links {
		patch("/relations/-", map[string]any{"rel": l.rel, "url": l.url})
	}
	return doc
}

//nolint:dupl // intentional duplicate of update.go's parser; sharing would require touching the sibling command.
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

// parseLinks splits each rel,url value on the first comma only.
func parseLinks(raw []string) ([]linkKV, error) {
	links := make([]linkKV, 0, len(raw))
	for _, r := range raw {
		rel, url, ok := strings.Cut(r, ",")
		if !ok || rel == "" || url == "" {
			return nil, util.FlagErrorf("--link value %q must be in the form rel,url", r)
		}
		links = append(links, linkKV{rel: rel, url: url})
	}
	return links, nil
}
