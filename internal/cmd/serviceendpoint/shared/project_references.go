package shared

import (
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
)

// EnsureProjectReferenceIncluded ensures that the given list of project references includes
// an entry for the current project.
//
// Matching logic:
// 1. If IDs are available, matches by ID.
// 2. Fallback: matches by name (case-insensitive).
//
// If the list is empty/nil, it is initialized with the current project.
// If the current project is not found, it is appended.
func EnsureProjectReferenceIncluded(refs *[]serviceendpoint.ServiceEndpointProjectReference, current *serviceendpoint.ProjectReference) {
	if current == nil || refs == nil {
		return
	}

	if len(*refs) == 0 {
		*refs = []serviceendpoint.ServiceEndpointProjectReference{
			{ProjectReference: current},
		}
		return
	}

	found := false
	for _, ref := range *refs {
		if ref.ProjectReference == nil {
			continue
		}

		// 1. Try ID match
		if current.Id != nil && ref.ProjectReference.Id != nil {
			if *current.Id == *ref.ProjectReference.Id {
				found = true
				break
			}
		}

		// 2. Fallback to Name match
		if current.Name != nil && ref.ProjectReference.Name != nil {
			if strings.EqualFold(*current.Name, *ref.ProjectReference.Name) {
				found = true
				break
			}
		}
	}

	if !found {
		*refs = append(*refs, serviceendpoint.ServiceEndpointProjectReference{
			ProjectReference: current,
		})
	}
}
