package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	wishared "github.com/tmeckel/azdo-cli/internal/cmd/boards/workitem/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestWorkItemIDFromURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want int
		ok   bool
	}{
		{
			name: "dev.azure full URL",
			raw:  "https://dev.azure.com/myorg/Contoso/_apis/wit/workItems/77",
			want: 77,
			ok:   true,
		},
		{
			name: "visualstudio URL",
			raw:  "https://myorg.visualstudio.com/Contoso/_apis/wit/workItems/42",
			want: 42,
			ok:   true,
		},
		{
			name: "bare path with ID",
			raw:  "/workItems/3",
			want: 3,
			ok:   true,
		},
		{
			name: "artifact URL",
			raw:  "https://example.com/1",
			want: 0,
			ok:   false,
		},
		{
			name: "no workItems segment",
			raw:  "https://dev.azure.com/myorg/_apis/wit/workItems/abc",
			want: 0,
			ok:   false,
		},
		{
			name: "zero ID",
			raw:  "https://dev.azure.com/myorg/_apis/wit/workItems/0",
			want: 0,
			ok:   false,
		},
		{
			name: "malformed",
			raw:  "not a url",
			want: 0,
			ok:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := WorkItemIDFromURL(tt.raw)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetchRelationTarget_success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	wit := mocks.NewMockWorkItemTrackingClient(ctrl)

	fields := map[string]any{
		wishared.TeamProjectField: "Contoso",
		"System.Title":            "Deploy the fix",
	}
	wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
			require.Equal(t, "Fabrikam", *args.Project)
			require.Equal(t, 77, *args.Id)
			require.Equal(t, []string{wishared.TeamProjectField, "System.Title"}, *args.Fields)
			return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
		},
	)

	got := FetchRelationTarget(context.Background(), wit, &util.Path{Organization: "myorg", Project: "Fabrikam"}, 77, "https://dev.azure.com/myorg/Contoso/_apis/wit/workItems/77")
	assert.Equal(t, RelationTarget{Organization: "myorg", Project: "Contoso", ID: 77, Title: "Deploy the fix"}, got)
}

func TestFetchRelationTarget_fallback(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	wit := mocks.NewMockWorkItemTrackingClient(ctrl)
	wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found"))

	got := FetchRelationTarget(context.Background(), wit, &util.Path{Organization: "myorg", Project: "Fabrikam"}, 42, "https://dev.azure.com/otherorg/Proj/_apis/wit/workItems/42")
	assert.Equal(t, RelationTarget{Organization: "otherorg", Project: "Proj", ID: 42}, got)
}

func TestResolveRelationTarget_cacheHitAndMiss(t *testing.T) {
	t.Parallel()

	t.Run("cache hit avoids fetch", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)
		wit := mocks.NewMockWorkItemTrackingClient(ctrl)
		cached := map[int]RelationTarget{
			77: {Organization: "myorg", Project: "Contoso", ID: 77, Title: "Cached"},
		}

		got := ResolveRelationTarget(context.Background(), wit, &util.Path{Organization: "myorg"}, cached, "https://dev.azure.com/myorg/Contoso/_apis/wit/workItems/77")
		assert.Equal(t, cached[77], got)
	})

	t.Run("miss fetches once then caches", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)
		wit := mocks.NewMockWorkItemTrackingClient(ctrl)
		fetches := 0
		wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, args workitemtracking.GetWorkItemArgs) (*workitemtracking.WorkItem, error) {
				fetches++
				fields := map[string]any{
					wishared.TeamProjectField: "Contoso",
					"System.Title":            "Deploy the fix",
				}
				return &workitemtracking.WorkItem{Id: args.Id, Fields: &fields}, nil
			},
		).AnyTimes()

		ctx := context.Background()
		scope := &util.Path{Organization: "myorg", Project: "Fabrikam"}
		cached := map[int]RelationTarget{}
		raw := "https://dev.azure.com/myorg/Contoso/_apis/wit/workItems/77"

		want := RelationTarget{Organization: "myorg", Project: "Contoso", ID: 77, Title: "Deploy the fix"}
		assert.Equal(t, want, ResolveRelationTarget(ctx, wit, scope, cached, raw))
		assert.Equal(t, want, ResolveRelationTarget(ctx, wit, scope, cached, raw))
		assert.Equal(t, 1, fetches)
		assert.Equal(t, want, cached[77])
	})
}

func TestResolveRelationTarget_nonWorkItemURL(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	wit := mocks.NewMockWorkItemTrackingClient(ctrl)

	got := ResolveRelationTarget(context.Background(), wit, &util.Path{Organization: "myorg"}, map[int]RelationTarget{}, "https://example.com/1")
	assert.Equal(t, RelationTarget{Title: "https://example.com/1"}, got)
}

func TestResolveRelationTarget_nilCache(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	wit := mocks.NewMockWorkItemTrackingClient(ctrl)
	fields := map[string]any{
		wishared.TeamProjectField: "Contoso",
		"System.Title":            "Deploy the fix",
	}
	wit.EXPECT().GetWorkItem(gomock.Any(), gomock.Any()).Return(
		&workitemtracking.WorkItem{Id: types.ToPtr(77), Fields: &fields}, nil,
	).AnyTimes()

	ctx := context.Background()
	scope := &util.Path{Organization: "myorg"}
	want := RelationTarget{Organization: "myorg", Project: "Contoso", ID: 77, Title: "Deploy the fix"}

	// A nil cache must not panic; repeated calls keep resolving.
	assert.Equal(t, want, ResolveRelationTarget(ctx, wit, scope, nil, "https://dev.azure.com/myorg/Contoso/_apis/wit/workItems/77"))
	assert.Equal(t, want, ResolveRelationTarget(ctx, wit, scope, nil, "https://dev.azure.com/myorg/Contoso/_apis/wit/workItems/77"))
}
