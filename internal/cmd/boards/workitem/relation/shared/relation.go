package shared

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"

	wishared "github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// RelationTarget carries the identity of a related work item for table
// renderers. Non-work-item (artifact) relations have an empty identity and put
// the raw URL into Title so the link stays visible.
type RelationTarget struct {
	Organization string
	Project      string
	ID           int
	Title        string
}

// ResolveRelationType resolves a friendly relation-type name to its
// referenceName via a case-insensitive match against the relation types,
// mirroring get_system_relation_name in the Azure DevOps CLI extension.
func ResolveRelationType(relTypes *[]workitemtracking.WorkItemRelationType, friendlyName string) (string, error) {
	if relTypes == nil {
		return "", fmt.Errorf("relation types API returned an empty response")
	}
	for _, relType := range *relTypes {
		if relType.Name != nil && strings.EqualFold(*relType.Name, friendlyName) {
			if relType.ReferenceName == nil || *relType.ReferenceName == "" {
				return "", fmt.Errorf("relation type %q has no reference name", friendlyName)
			}
			return *relType.ReferenceName, nil
		}
	}
	return "", fmt.Errorf("--relation-type is not valid. Use \"azdo boards work-item relation list-type\" command to list possible relation types in your project")
}

// PopulateFriendlyNames replaces each relation's Rel (referenceName) with its
// friendly Name, mirroring fill_friendly_name_for_relations_in_work_item in
// the Azure DevOps CLI extension. Relations are mutated in place.
func PopulateFriendlyNames(relTypes *[]workitemtracking.WorkItemRelationType, wi *workitemtracking.WorkItem) error {
	if wi == nil || wi.Relations == nil || relTypes == nil {
		return nil
	}
	for i := range *wi.Relations {
		rel := &(*wi.Relations)[i]
		if rel.Rel == nil {
			continue
		}
		for _, relType := range *relTypes {
			if relType.ReferenceName != nil && *relType.ReferenceName == *rel.Rel && relType.Name != nil {
				rel.Rel = relType.Name
				break
			}
		}
	}
	return nil
}

var workItemLinkRe = regexp.MustCompile(`/workItems/(\d+)`)

// WorkItemIDFromURL extracts the work item ID carried by a relation URL. The
// second result reports whether the URL points at a work item at all.
func WorkItemIDFromURL(raw string) (int, bool) {
	m := workItemLinkRe.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) != 2 {
		return 0, false
	}
	id, err := strconv.Atoi(m[1])
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// ResolveRelationTarget identifies the work item a relation URL points at.
// Same-organization links are fetched to recover the true project and title
// (fetches are cached by ID across relations when cached is non-nil). Remote
// or unresolvable links fall back to a best-effort URL parse; non-work-item
// links surface the URL as the title.
func ResolveRelationTarget(ctx context.Context, wit workitemtracking.Client, scope *util.Path, cached map[int]RelationTarget, raw string) RelationTarget {
	id, ok := WorkItemIDFromURL(raw)
	if !ok {
		return RelationTarget{Title: raw}
	}
	if cached != nil {
		if t, ok := cached[id]; ok {
			return t
		}
	}
	t := FetchRelationTarget(ctx, wit, scope, id, raw)
	if cached != nil {
		cached[id] = t
	}
	return t
}

// FetchRelationTarget fetches a related work item to recover its real project
// and title. When the fetch fails, the caller-provided URL is parsed as a
// best-effort fallback so remote organization/project remain visible.
func FetchRelationTarget(ctx context.Context, wit workitemtracking.Client, scope *util.Path, id int, raw string) RelationTarget {
	target, err := wit.GetWorkItem(ctx, workitemtracking.GetWorkItemArgs{
		Project: types.ToPtr(scope.Project),
		Id:      types.ToPtr(id),
		Fields:  types.ToPtr([]string{wishared.TeamProjectField, "System.Title"}),
	})
	if err == nil && target != nil {
		fields := types.GetValue(target.Fields, map[string]any{})
		return RelationTarget{
			Organization: scope.Organization,
			Project:      wishared.FieldString(fields, wishared.TeamProjectField),
			ID:           id,
			Title:        wishared.FieldString(fields, "System.Title"),
		}
	}
	org, project := wishared.ParseWorkItemURL(raw)
	return RelationTarget{
		Organization: org,
		Project:      project,
		ID:           id,
	}
}
