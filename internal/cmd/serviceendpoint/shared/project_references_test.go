package shared

import (
	"testing"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// Helper to create a ProjectReference
func createProjectRef(id string, name string) *serviceendpoint.ProjectReference {
	return &serviceendpoint.ProjectReference{
		Id:   types.ToPtr(uuid.MustParse(id)),
		Name: types.ToPtr(name),
	}
}

// Helper to create a ServiceEndpointProjectReference
func createSEProjectRef(id string, name string) serviceendpoint.ServiceEndpointProjectReference {
	return serviceendpoint.ServiceEndpointProjectReference{
		ProjectReference: createProjectRef(id, name),
	}
}

func TestEnsureProjectReferenceIncluded(t *testing.T) {
	t.Parallel()

	uuid1 := uuid.New().String()
	uuid2 := uuid.New().String()
	uuid3 := uuid.New().String()

	tests := []struct {
		name          string
		initialRefs   []serviceendpoint.ServiceEndpointProjectReference
		current       *serviceendpoint.ProjectReference
		nilRefs       bool
		nilCurrent    bool
		expectedRefs  []serviceendpoint.ServiceEndpointProjectReference
		expectedCount int // Expected number of elements after operation
	}{
		{
			name:          "current is nil, no-op",
			initialRefs:   []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1")},
			current:       nil,
			nilCurrent:    true,
			expectedRefs:  []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1")},
			expectedCount: 1,
		},
		{
			name:          "refs is nil, no-op",
			initialRefs:   nil,
			current:       createProjectRef(uuid1, "Proj1"),
			nilRefs:       true,
			expectedRefs:  nil, // Should remain nil
			expectedCount: 0,
		},
		{
			name:          "refs is empty, initializes with current",
			initialRefs:   []serviceendpoint.ServiceEndpointProjectReference{},
			current:       createProjectRef(uuid1, "Proj1"),
			expectedRefs:  []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1")},
			expectedCount: 1,
		},
		{
			name:          "current already present by ID",
			initialRefs:   []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1"), createSEProjectRef(uuid2, "Proj2")},
			current:       createProjectRef(uuid1, "Proj1-Renamed"), // Same ID, different name
			expectedRefs:  []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1"), createSEProjectRef(uuid2, "Proj2")},
			expectedCount: 2,
		},
		{
			name:          "current already present by Name (case-insensitive), no ID match",
			initialRefs:   []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1"), createSEProjectRef(uuid2, "Proj2")},
			current:       createProjectRef(uuid3, "proj2"), // Different ID, same name (case-insensitive)
			expectedRefs:  []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1"), createSEProjectRef(uuid2, "Proj2")},
			expectedCount: 2,
		},
		{
			name:          "current not present, appends",
			initialRefs:   []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1")},
			current:       createProjectRef(uuid2, "Proj2"),
			expectedRefs:  []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1"), createSEProjectRef(uuid2, "Proj2")},
			expectedCount: 2,
		},
		{
			name:          "current not present (no ID on current), appends",
			initialRefs:   []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1")},
			current:       &serviceendpoint.ProjectReference{Name: types.ToPtr("Proj2")}, // No ID
			expectedRefs:  []serviceendpoint.ServiceEndpointProjectReference{createSEProjectRef(uuid1, "Proj1"), {ProjectReference: &serviceendpoint.ProjectReference{Name: types.ToPtr("Proj2")}}},
			expectedCount: 2,
		},
		{
			name:          "current present (no ID on current), no-op",
			initialRefs:   []serviceendpoint.ServiceEndpointProjectReference{{ProjectReference: &serviceendpoint.ProjectReference{Name: types.ToPtr("Proj1")}}},
			current:       &serviceendpoint.ProjectReference{Name: types.ToPtr("proj1")},
			expectedRefs:  []serviceendpoint.ServiceEndpointProjectReference{{ProjectReference: &serviceendpoint.ProjectReference{Name: types.ToPtr("Proj1")}}},
			expectedCount: 1,
		},
		{
			name:          "refs contains nil ProjectReference, ignores and appends",
			initialRefs:   []serviceendpoint.ServiceEndpointProjectReference{{ProjectReference: nil}},
			current:       createProjectRef(uuid1, "NewProj"),
			expectedRefs:  []serviceendpoint.ServiceEndpointProjectReference{{ProjectReference: nil}, createSEProjectRef(uuid1, "NewProj")},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var refs *[]serviceendpoint.ServiceEndpointProjectReference
			if !tt.nilRefs {
				refs = &tt.initialRefs
			}

			var current *serviceendpoint.ProjectReference
			if !tt.nilCurrent {
				current = tt.current
			}

			EnsureProjectReferenceIncluded(refs, current)

			if tt.nilRefs {
				assert.Nil(t, refs)
			} else {
				require.NotNil(t, refs)
				assert.Len(t, *refs, tt.expectedCount)
				if tt.expectedCount > 0 {
					for i, expected := range tt.expectedRefs {
						actual := (*refs)[i]

						if expected.ProjectReference == nil {
							assert.Nil(t, actual.ProjectReference)
							continue
						}

						require.NotNil(t, actual.ProjectReference)

						// Compare IDs if present
						if expected.ProjectReference.Id != nil {
							require.NotNil(t, actual.ProjectReference.Id)
							assert.Equal(t, *expected.ProjectReference.Id, *actual.ProjectReference.Id)
						} else {
							assert.Nil(t, actual.ProjectReference.Id)
						}
						// Compare Names if present
						if expected.ProjectReference.Name != nil {
							require.NotNil(t, actual.ProjectReference.Name)
							assert.Equal(t, *expected.ProjectReference.Name, *actual.ProjectReference.Name)
						} else {
							assert.Nil(t, actual.ProjectReference.Name)
						}
					}
				}
			}
		})
	}
}
