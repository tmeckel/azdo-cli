package shared

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

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
