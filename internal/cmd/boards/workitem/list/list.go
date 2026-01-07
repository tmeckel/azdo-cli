package list

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/azdo/extensions"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type listOptions struct {
	scopeArg string

	status         []string
	workItemTypes  []string
	assignedTo     []string
	classification []string
	priority       []int
	area           []string
	iteration      []string
	limit          int

	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List work items belonging to a project.",
		Long: heredoc.Doc(`
			List work items belonging to a project within an Azure DevOps organization.

			This command builds and runs a WIQL query to obtain work item IDs and then fetches the
			work item details in batches.
		`),
		Example: heredoc.Doc(`
			# List open work items for a project in the default organization
			azdo boards work-item list Fabrikam

			# List all work items assigned to you
			azdo boards work-item list Fabrikam --assigned-to @me --status all

			# Filter by work item type and priority
			azdo boards work-item list Fabrikam --type "User Story" --priority 1 --priority 2

			# Filter by area subtree
			azdo boards work-item list Fabrikam --area Under:Web/Payments

			# Export JSON
			azdo boards work-item list Fabrikam --json id,fields
		`),
		Aliases: []string{"ls", "l"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runList(ctx, opts)
		},
	}

	cmd.Flags().StringSliceVarP(&opts.status, "status", "s", []string{"open"}, "Filter by state category: open, closed, resolved, all (repeatable)")
	cmd.Flags().StringSliceVarP(&opts.workItemTypes, "type", "T", nil, "Filter by work item type (repeatable)")
	cmd.Flags().StringSliceVarP(&opts.assignedTo, "assigned-to", "a", nil, "Filter by assigned-to identity (repeatable); supports emails, descriptors, and @me")
	cmd.Flags().StringSliceVarP(&opts.classification, "classification", "c", nil, "Filter by severity classification (repeatable): 1 - Critical, 2 - High, 3 - Medium, 4 - Low")
	cmd.Flags().IntSliceVarP(&opts.priority, "priority", "p", nil, "Filter by priority (repeatable): 1-4")
	cmd.Flags().StringSliceVar(&opts.area, "area", nil, "Filter by area path (repeatable); prefix with Under: to include subtree (e.g., Under:Web/Payments)")
	cmd.Flags().StringSliceVar(&opts.iteration, "iteration", nil, "Filter by iteration path (repeatable); prefix with Under: to include subtree (e.g., Under:Release 2025/Sprint 1)")
	cmd.Flags().IntVarP(&opts.limit, "limit", "L", 0, "Maximum number of results to return (>=1)")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"url", "_links", "commentVersionRef", "fields", "id", "relations", "rev"})

	return cmd
}

func runList(ctx util.CmdContext, opts *listOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if err := validateListOptions(opts); err != nil {
		return err
	}

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	witClient, err := ctx.ClientFactory().WorkItemTracking(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create work item tracking client: %w", err)
	}

	assignedToFilter, err := resolveAssignedToFilter(ctx, scope.Organization, opts.assignedTo)
	if err != nil {
		return err
	}

	statusPredicate, err := resolveStatePredicate(ctx, witClient, scope.Project, opts.status, opts.workItemTypes)
	if err != nil {
		return err
	}

	query := buildWiqlQuery(scope.Project, statusPredicate, opts.workItemTypes, assignedToFilter, opts.classification, opts.priority, opts.area, opts.iteration)

	zap.L().Debug("querying work items via WIQL",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.Int("limit", opts.limit),
	)

	wiqlQueryArgs := workitemtracking.QueryByWiqlArgs{
		Wiql:    &workitemtracking.Wiql{Query: &query},
		Project: &scope.Project,
	}

	if opts.limit > 0 {
		wiqlQueryArgs.Top = &opts.limit
	}

	result, err := witClient.QueryByWiql(ctx.Context(), wiqlQueryArgs)
	if err != nil {
		return fmt.Errorf("failed to execute WIQL query: %w", err)
	}

	ids := extractWorkItemIDs(result)
	if len(ids) == 0 {
		return util.NewNoResultsError("no work items matched the provided filters")
	}

	workItems, err := fetchWorkItems(ctx, witClient, scope.Project, ids, result.AsOf, opts.exporter != nil)
	if err != nil {
		return err
	}
	if len(workItems) == 0 {
		return util.NewNoResultsError("no work items matched the provided filters")
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, workItems)
	}

	return renderWorkItemsTable(ctx, workItems)
}

func validateListOptions(opts *listOptions) error {
	if opts == nil {
		return util.FlagErrorf("invalid options")
	}
	if err := validateClassification(opts.classification); err != nil {
		return err
	}
	if err := validatePriority(opts.priority); err != nil {
		return err
	}
	if err := validateUnderPaths("--area", opts.area); err != nil {
		return err
	}
	if err := validateUnderPaths("--iteration", opts.iteration); err != nil {
		return err
	}
	return nil
}

func renderWorkItemsTable(ctx util.CmdContext, workItems []workitemtracking.WorkItem) error {
	tp, err := ctx.Printer("table")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "TYPE", "STATE", "TITLE", "ASSIGNED TO", "AREA", "ITERATION")
	for _, wi := range workItems {
		fields := types.GetValue(wi.Fields, map[string]any{})
		tp.AddField(strconv.Itoa(types.GetValue(wi.Id, 0)))
		tp.AddField(fieldString(fields, "System.WorkItemType"))
		tp.AddField(fieldString(fields, "System.State"))
		tp.AddField(fieldString(fields, "System.Title"))
		tp.AddField(fieldIdentityDisplay(fields, "System.AssignedTo"))
		tp.AddField(fieldString(fields, "System.AreaPath"))
		tp.AddField(fieldString(fields, "System.IterationPath"))
		tp.EndRow()
	}

	return tp.Render()
}

func resolveStatePredicate(ctx util.CmdContext, client workitemtracking.Client, project string, rawStatus []string, rawTypes []string) (string, error) {
	statuses := normalizeStatuses(rawStatus)
	if slices.Contains(statuses, "all") {
		return "", nil
	}

	wantedCategories := make(map[string]struct{})
	for _, s := range statuses {
		switch s {
		case "open":
			addWantedCategory(wantedCategories, "New")
			addWantedCategory(wantedCategories, "Active")
			addWantedCategory(wantedCategories, "Proposed")
			addWantedCategory(wantedCategories, "InProgress")
		case "closed":
			addWantedCategory(wantedCategories, "Completed")
			addWantedCategory(wantedCategories, "Removed")
		case "resolved":
			addWantedCategory(wantedCategories, "Resolved")
		default:
			return "", util.FlagErrorf("invalid value for --status: %q (valid: open, closed, resolved, all)", s)
		}
	}

	stateNames := make([]string, 0)

	trimmedTypes := trimStrings(rawTypes)
	if len(trimmedTypes) > 0 {
		for _, typeName := range trimmedTypes {
			localType := typeName
			states, err := client.GetWorkItemTypeStates(ctx.Context(), workitemtracking.GetWorkItemTypeStatesArgs{
				Project: &project,
				Type:    &localType,
			})
			if err != nil {
				return "", fmt.Errorf("failed to resolve states for work item type %q: %w", localType, err)
			}
			appendStateNamesByCategory(&stateNames, states, wantedCategories)
		}
	} else {
		typesList, err := client.GetWorkItemTypes(ctx.Context(), workitemtracking.GetWorkItemTypesArgs{
			Project: &project,
		})
		if err != nil {
			return "", fmt.Errorf("failed to list work item types: %w", err)
		}
		if typesList == nil || len(*typesList) == 0 {
			return "", util.FlagErrorf("no work item types are available to resolve --status")
		}

		for _, t := range *typesList {
			if types.GetValue(t.IsDisabled, false) {
				continue
			}

			if t.States != nil {
				appendStateNamesByCategory(&stateNames, t.States, wantedCategories)
				continue
			}

			typeName := types.GetValue(t.Name, "")
			if typeName == "" {
				continue
			}
			localType := typeName
			states, err := client.GetWorkItemTypeStates(ctx.Context(), workitemtracking.GetWorkItemTypeStatesArgs{
				Project: &project,
				Type:    &localType,
			})
			if err != nil {
				return "", fmt.Errorf("failed to resolve states for work item type %q: %w", localType, err)
			}
			appendStateNamesByCategory(&stateNames, states, wantedCategories)
		}
	}

	stateNames = types.UniqueComparable(stateNames, strings.ToLower)
	if len(stateNames) == 0 {
		return "", util.FlagErrorf("no states matched --status filters for the selected work item types")
	}

	return fmt.Sprintf("[System.State] IN (%s)", wiqlQuoteList(stateNames)), nil
}

func appendStateNamesByCategory(stateNames *[]string, states *[]workitemtracking.WorkItemStateColor, wantedCategories map[string]struct{}) {
	if stateNames == nil || states == nil || len(*states) == 0 {
		return
	}

	for _, st := range *states {
		category := types.GetValue(st.Category, "")
		name := types.GetValue(st.Name, "")
		if category == "" || name == "" {
			continue
		}
		if _, ok := wantedCategories[canonCategory(category)]; ok {
			*stateNames = append(*stateNames, name)
		}
	}
}

func normalizeStatuses(raw []string) []string {
	if len(raw) == 0 {
		raw = []string{"open"}
	}

	normalized := make([]string, 0, len(raw))
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		normalized = append(normalized, strings.ToLower(v))
	}
	if len(normalized) == 0 {
		normalized = []string{"open"}
	}

	return types.Unique(normalized)
}

func addWantedCategory(categories map[string]struct{}, category string) {
	if categories == nil {
		return
	}
	categories[canonCategory(category)] = struct{}{}
}

func canonCategory(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, " ", "")
	return strings.ToLower(raw)
}

func validateClassification(values []string) error {
	allowed := map[string]struct{}{
		"1 - Critical": {},
		"2 - High":     {},
		"3 - Medium":   {},
		"4 - Low":      {},
	}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := allowed[v]; !ok {
			return util.FlagErrorf("invalid value for --classification: %q", v)
		}
	}
	return nil
}

func validatePriority(values []int) error {
	for _, v := range values {
		if v < 1 || v > 4 {
			return util.FlagErrorf("invalid value for --priority: %d (valid: 1-4)", v)
		}
	}
	return nil
}

func validateUnderPaths(flag string, values []string) error {
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		path := strings.TrimPrefix(raw, "Under:")
		path = strings.TrimSpace(path)
		if path == "" {
			return util.FlagErrorf("%s value %q is invalid; path must not be empty", flag, raw)
		}
	}
	return nil
}

func resolveAssignedToFilter(ctx util.CmdContext, organization string, assignedTo []string) ([]string, error) {
	if len(assignedTo) == 0 {
		return nil, nil
	}

	needsLookup := false
	for _, raw := range assignedTo {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.EqualFold(raw, "@me") {
			needsLookup = true
			break
		}
		if shouldResolveIdentity(raw) {
			needsLookup = true
			break
		}
	}

	var extensionsClient extensions.Client
	var identityClient identity.Client
	if needsLookup {
		ext, err := ctx.ClientFactory().Extensions(ctx.Context(), organization)
		if err != nil {
			return nil, fmt.Errorf("failed to create Extensions client: %w", err)
		}
		extensionsClient = ext

		idc, err := ctx.ClientFactory().Identity(ctx.Context(), organization)
		if err != nil {
			return nil, fmt.Errorf("failed to create Identity client: %w", err)
		}
		identityClient = idc
	}

	resolved := make([]string, 0, len(assignedTo))
	for _, raw := range assignedTo {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		if strings.EqualFold(raw, "@me") {
			selfID, err := extensionsClient.GetSelfID(ctx.Context())
			if err != nil {
				return nil, fmt.Errorf("failed to resolve @me identity: %w", err)
			}
			identityIds := selfID.String()
			identities, err := identityClient.ReadIdentities(ctx.Context(), identity.ReadIdentitiesArgs{
				IdentityIds: &identityIds,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to resolve @me identity details: %w", err)
			}
			if identities == nil || len(*identities) != 1 {
				return nil, fmt.Errorf("failed to resolve @me identity details")
			}
			value := identityAccountOrDisplay((*identities)[0])
			if value == "" {
				return nil, fmt.Errorf("authenticated identity is missing account or display name")
			}
			resolved = append(resolved, value)
			continue
		}

		// WIQL identity fields accept display names/emails directly. Keep those values unchanged and
		// only resolve aliases/descriptors when needed.
		if !shouldResolveIdentity(raw) {
			resolved = append(resolved, raw)
			continue
		}

		ident, err := extensionsClient.ResolveIdentity(ctx.Context(), raw)
		if err != nil {
			return nil, err
		}
		if ident == nil {
			return nil, fmt.Errorf("no identity found for %q", raw)
		}
		value := identityAccountOrDisplay(*ident)
		if value == "" {
			return nil, fmt.Errorf("resolved identity for %q is missing account or display name", raw)
		}
		resolved = append(resolved, value)
	}

	resolved = types.UniqueComparable(resolved, strings.ToLower)
	return resolved, nil
}

func shouldResolveIdentity(raw string) bool {
	// Keep values that already look like display names/emails unchanged.
	return !(strings.Contains(raw, " ") || strings.Contains(raw, "@"))
}

func identityAccountOrDisplay(ident identity.Identity) string {
	if account := identityAccount(ident.Properties); account != "" {
		return account
	}
	return types.GetValue(ident.ProviderDisplayName, "")
}

func identityAccount(properties any) string {
	props, ok := properties.(map[string]any)
	if !ok {
		return ""
	}
	raw, ok := props["Account"]
	if !ok || raw == nil {
		return ""
	}

	account, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	if value, ok := account["$value"].(string); ok {
		return value
	}
	return ""
}

func buildWiqlQuery(project string, stateCategoryPredicate string, typesFilter []string, assignedTo []string, severity []string, priority []int, area []string, iteration []string) string {
	clauses := make([]string, 0)
	clauses = append(clauses, fmt.Sprintf("[System.TeamProject] = %s", wiqlQuote(project)))

	if stateCategoryPredicate != "" {
		clauses = append(clauses, stateCategoryPredicate)
	}

	if len(typesFilter) > 0 {
		trimmed := trimStrings(typesFilter)
		if len(trimmed) > 0 {
			clauses = append(clauses, fmt.Sprintf("[System.WorkItemType] IN (%s)", wiqlQuoteList(trimmed)))
		}
	}

	if len(assignedTo) > 0 {
		clauses = append(clauses, fmt.Sprintf("[System.AssignedTo] IN (%s)", wiqlQuoteList(assignedTo)))
	}

	if len(severity) > 0 {
		trimmed := trimStrings(severity)
		if len(trimmed) > 0 {
			clauses = append(clauses, fmt.Sprintf("[Microsoft.VSTS.Common.Severity] IN (%s)", wiqlQuoteList(trimmed)))
		}
	}

	if len(priority) > 0 {
		clauses = append(clauses, fmt.Sprintf("[Microsoft.VSTS.Common.Priority] IN (%s)", wiqlIntList(priority)))
	}

	if len(area) > 0 {
		if predicate := buildUnderOrEqualsPredicate("[System.AreaPath]", area); predicate != "" {
			clauses = append(clauses, predicate)
		}
	}
	if len(iteration) > 0 {
		if predicate := buildUnderOrEqualsPredicate("[System.IterationPath]", iteration); predicate != "" {
			clauses = append(clauses, predicate)
		}
	}

	return fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE %s ORDER BY [System.ChangedDate] DESC", strings.Join(clauses, " AND "))
}

func buildUnderOrEqualsPredicate(field string, raw []string) string {
	parts := make([]string, 0, len(raw))
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "Under:") {
			path := strings.TrimSpace(strings.TrimPrefix(v, "Under:"))
			parts = append(parts, fmt.Sprintf("%s UNDER %s", field, wiqlQuote(path)))
		} else {
			parts = append(parts, fmt.Sprintf("%s = %s", field, wiqlQuote(v)))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

func wiqlQuote(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", "''") + "'"
}

func wiqlQuoteList(values []string) string {
	escaped := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		escaped = append(escaped, wiqlQuote(v))
	}
	return strings.Join(escaped, ", ")
}

func wiqlIntList(values []int) string {
	values = types.Unique(values)
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, strconv.Itoa(v))
	}
	return strings.Join(parts, ", ")
}

func trimStrings(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		trimmed = append(trimmed, v)
	}
	return types.UniqueComparable(trimmed, strings.ToLower)
}

func extractWorkItemIDs(result *workitemtracking.WorkItemQueryResult) []int {
	if result == nil || result.WorkItems == nil {
		return nil
	}
	workItems := *result.WorkItems
	ids := make([]int, 0, len(workItems))
	for _, wi := range workItems {
		if wi.Id == nil {
			continue
		}
		ids = append(ids, *wi.Id)
	}
	return ids
}

func fetchWorkItems(ctx util.CmdContext, client workitemtracking.Client, project string, ids []int, asOf *azuredevops.Time, includeRelations bool) ([]workitemtracking.WorkItem, error) {
	const batchSize = 200

	if len(ids) == 0 {
		return nil, nil
	}

	fields := []string{
		"System.Id",
		"System.WorkItemType",
		"System.State",
		"System.Title",
		"System.AssignedTo",
		"System.AreaPath",
		"System.IterationPath",
	}
	var expand *workitemtracking.WorkItemExpand
	var requestFields *[]string
	if includeRelations {
		e := workitemtracking.WorkItemExpandValues.All
		expand = &e
		requestFields = nil
	} else {
		requestFields = &fields
	}

	errorPolicy := workitemtracking.WorkItemErrorPolicyValues.Omit

	out := make([]workitemtracking.WorkItem, 0, len(ids))
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		args := workitemtracking.GetWorkItemsBatchArgs{
			Project: &project,
			WorkItemGetRequest: &workitemtracking.WorkItemBatchGetRequest{
				Ids:         &batch,
				AsOf:        asOf,
				Fields:      requestFields,
				Expand:      expand,
				ErrorPolicy: &errorPolicy,
			},
		}

		items, err := client.GetWorkItemsBatch(ctx.Context(), args)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch work item details: %w", err)
		}
		if items == nil || len(*items) == 0 {
			continue
		}
		out = append(out, (*items)...)
	}

	return orderWorkItemsByIDs(out, ids), nil
}

func orderWorkItemsByIDs(items []workitemtracking.WorkItem, ids []int) []workitemtracking.WorkItem {
	if len(items) == 0 || len(ids) == 0 {
		return items
	}

	byID := make(map[int]workitemtracking.WorkItem, len(items))
	for _, item := range items {
		if item.Id == nil {
			continue
		}
		byID[*item.Id] = item
	}

	ordered := make([]workitemtracking.WorkItem, 0, len(items))
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
			seen[id] = struct{}{}
		}
	}

	// Preserve any extra items not in the id list (should not happen, but keeps output stable).
	for _, item := range items {
		if item.Id == nil {
			continue
		}
		if _, ok := seen[*item.Id]; ok {
			continue
		}
		ordered = append(ordered, item)
	}

	return ordered
}

func fieldString(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	v, ok := fields[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(v)
	}
}

func fieldIdentityDisplay(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	v, ok := fields[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if displayName, ok := t["displayName"].(string); ok {
			return displayName
		}
		if uniqueName, ok := t["uniqueName"].(string); ok {
			return uniqueName
		}
		return fmt.Sprint(v)
	default:
		return fmt.Sprint(v)
	}
}
