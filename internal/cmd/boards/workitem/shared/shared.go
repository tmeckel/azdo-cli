package shared

import (
	"fmt"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// TeamProjectField is the work item field that identifies the parent project.
const TeamProjectField = "System.TeamProject"

// BelongsToProject reports whether the work item's System.TeamProject field
// matches the given project. Used to verify project ownership before
// mutating or deleting work items.
func BelongsToProject(item *workitemtracking.WorkItem, project string) bool {
	if item == nil || item.Fields == nil {
		return false
	}
	got, ok := (*item.Fields)[TeamProjectField]
	return ok && fmt.Sprint(got) == project
}
