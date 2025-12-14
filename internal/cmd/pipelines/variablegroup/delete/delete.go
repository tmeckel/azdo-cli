package delete

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/variablegroup/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type options struct {
	targetArg         string
	yes               bool
	projectReferences []string
	all               bool
	exporter          util.Exporter
}

type deleteResult struct {
	Deleted bool `json:"deleted"`
	GroupID int  `json:"groupId"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &options{}

	cmd := &cobra.Command{
		Use:   "delete [ORGANIZATION/]PROJECT/GROUP",
		Short: "Delete a variable group from a project",
		Long: heredoc.Doc(`
			Delete a variable group from a project using its numeric ID or name. The command prompts
			for confirmation unless --yes is supplied.
		`),
		Example: heredoc.Doc(`
			# Delete a variable group by ID in the default organization
			azdo pipelines variable-group delete MyProject/123 --yes

			# Delete a variable group by name in a specific organization
			azdo pipelines variable-group delete 'myorg/MyProject/Shared Config'

			# Remove a shared group from two additional projects
			azdo pipelines variable-group delete MyProject/SharedConfig --project-reference ProjectB --project-reference ProjectC

			# Remove a group from every project assignment
			azdo pipelines variable-group delete MyProject/SharedConfig --all --yes
		`),
		Aliases: []string{
			"rm",
			"del",
			"d",
		},
		Args: util.ExactArgs(1, "variable group target is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return run(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip the confirmation prompt.")
	cmd.Flags().StringSliceVar(&opts.projectReferences, "project-reference", nil, "Additional project names or IDs to remove the group from (repeatable, comma-separated)")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Remove the variable group from every assigned project")
	util.AddJSONFlags(cmd, &opts.exporter, []string{"deleted", "groupId"})

	return cmd
}

func run(cmdCtx util.CmdContext, opts *options) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if opts.all && len(opts.projectReferences) > 0 {
		return util.FlagErrorf("--all cannot be combined with --project-reference")
	}

	scope, err := util.ParseProjectTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	taskClient, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create task agent client: %w", err)
	}

	group, err := shared.ResolveVariableGroup(cmdCtx, taskClient, scope.Project, scope.Target)
	if err != nil {
		return err
	}

	if group.Id == nil {
		return fmt.Errorf("resolved variable group is missing an ID")
	}
	groupID := *group.Id
	groupName := types.GetValue(group.Name, scope.Target)

	projectIndex := buildProjectIndex(group)
	projectIDs, err := selectProjectIDs(projectIndex, scope.Project, opts.projectReferences, opts.all)
	if err != nil {
		return err
	}
	if len(projectIDs) == 0 {
		return fmt.Errorf("no project assignments found to delete")
	}

	zap.L().Debug("resolved variable group",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("input", scope.Target),
		zap.Int("groupId", groupID),
		zap.String("name", groupName),
	)

	if !opts.yes {
		if !ios.CanPrompt() {
			return util.FlagErrorf("--yes required when not running interactively")
		}
		ios.StopProgressIndicator()
		prompter, err := cmdCtx.Prompter()
		if err != nil {
			return err
		}
		message := confirmationMessage(groupName, scope.Organization, scope.Project, projectIDs, projectIndex, opts.all, len(opts.projectReferences) > 0)
		confirmed, err := prompter.Confirm(message, false)
		if err != nil {
			return err
		}
		if !confirmed {
			zap.L().Debug("variable group deletion canceled by user", zap.String("group", groupName))
			return util.ErrCancel
		}
		ios.StartProgressIndicator()
	}

	deleteArgs := taskagent.DeleteVariableGroupArgs{
		GroupId:    &groupID,
		ProjectIds: &projectIDs,
	}

	if err := taskClient.DeleteVariableGroup(cmdCtx.Context(), deleteArgs); err != nil {
		return fmt.Errorf("failed to delete variable group %d: %w", groupID, err)
	}

	zap.L().Debug("variable group deleted",
		zap.Int("groupId", groupID),
		zap.Strings("projectIds", projectIDs),
		zap.String("organization", scope.Organization),
	)

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, deleteResult{
			Deleted: true,
			GroupID: groupID,
		})
	}

	fmt.Fprintf(ios.Out, "Variable group deleted from %d project(s).\n", len(projectIDs))
	return nil
}

type projectAssignment struct {
	ID   string
	Name string
}

type projectIndex struct {
	assignments []projectAssignment
	keyToID     map[string]string
	nameByID    map[string]string
}

func buildProjectIndex(group *taskagent.VariableGroup) *projectIndex {
	idx := &projectIndex{
		assignments: []projectAssignment{},
		keyToID:     make(map[string]string),
		nameByID:    make(map[string]string),
	}
	if group == nil || group.VariableGroupProjectReferences == nil {
		return idx
	}
	seen := make(map[string]struct{})
	for _, ref := range *group.VariableGroupProjectReferences {
		if ref.ProjectReference == nil || ref.ProjectReference.Id == nil {
			continue
		}
		id := ref.ProjectReference.Id.String()
		idKey := strings.ToLower(strings.TrimSpace(id))
		if strings.TrimSpace(id) == "" {
			continue
		}
		if _, exists := seen[idKey]; exists {
			continue
		}
		seen[idKey] = struct{}{}
		name := types.GetValue(ref.ProjectReference.Name, "")
		idx.assignments = append(idx.assignments, projectAssignment{
			ID:   id,
			Name: name,
		})
		idx.nameByID[id] = name
		idx.keyToID[idKey] = id
		if strings.TrimSpace(name) != "" {
			idx.keyToID[strings.ToLower(name)] = id
		}
	}
	return idx
}

func (idx *projectIndex) lookup(token string) (string, bool) {
	if idx == nil {
		return "", false
	}
	key := normalizeProjectToken(token)
	if key == "" {
		return "", false
	}
	id, ok := idx.keyToID[key]
	return id, ok
}

func (idx *projectIndex) displayName(id string, fallback string) string {
	if idx == nil {
		return fallback
	}
	if name, ok := idx.nameByID[id]; ok && strings.TrimSpace(name) != "" {
		return name
	}
	return fallback
}

func normalizeProjectToken(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	trimmed = parts[len(parts)-1]
	return strings.ToLower(trimmed)
}

func selectProjectIDs(idx *projectIndex, base string, extras []string, all bool) ([]string, error) {
	if idx == nil {
		idx = &projectIndex{
			assignments: []projectAssignment{},
			keyToID:     make(map[string]string),
			nameByID:    make(map[string]string),
		}
	}

	add := func(target string, seen map[string]struct{}, ids *[]string) error {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			return util.FlagErrorf("project reference values must not be empty")
		}
		id, ok := idx.lookup(trimmed)
		if !ok {
			return util.FlagErrorf("variable group is not assigned to project %q", trimmed)
		}
		lowerID := strings.ToLower(id)
		if _, exists := seen[lowerID]; exists {
			return nil
		}
		seen[lowerID] = struct{}{}
		*ids = append(*ids, id)
		return nil
	}

	seen := make(map[string]struct{})
	ids := make([]string, 0)

	if all {
		if len(idx.assignments) == 0 {
			return nil, fmt.Errorf("variable group has no project assignments to remove")
		}
		for _, assignment := range idx.assignments {
			if err := add(assignment.ID, seen, &ids); err != nil {
				return nil, err
			}
		}
		return ids, nil
	}

	if err := add(base, seen, &ids); err != nil {
		return nil, err
	}

	for _, raw := range extras {
		if err := add(raw, seen, &ids); err != nil {
			return nil, err
		}
	}

	return ids, nil
}

func confirmationMessage(groupName, organization, requestedProject string, projectIDs []string, idx *projectIndex, all bool, hasExtras bool) string {
	count := len(projectIDs)
	if all {
		return fmt.Sprintf("Delete variable group %q from all %d assigned project(s)?", groupName, count)
	}
	if !hasExtras && count == 1 {
		projectName := idx.displayName(projectIDs[0], requestedProject)
		return fmt.Sprintf("Delete variable group %q from project %s/%s?", groupName, organization, projectName)
	}
	return fmt.Sprintf("Delete variable group %q from %d project assignment(s)?", groupName, count)
}
