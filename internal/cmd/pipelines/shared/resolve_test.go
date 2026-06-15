package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/types"
)

func TestResolvePipelineDefinition_PositiveID(t *testing.T) {
	t.Parallel()

	id, err := ResolvePipelineDefinition(nil, nil, "Fabrikam", "42")

	require.NoError(t, err)
	assert.Equal(t, 42, id)
}

func TestResolvePipelineDefinition_RejectsNonPositiveID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "zero", raw: "0"},
		{name: "negative", raw: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ResolvePipelineDefinition(nil, nil, "Fabrikam", tt.raw)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "pipeline id must be greater than zero")
		})
	}
}

func TestResolvePipelineDefinition_NameAmbiguous(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockBuildClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).Times(1)
	client.EXPECT().GetDefinitions(gomock.Any(), build.GetDefinitionsArgs{
		Project: types.ToPtr("Fabrikam"),
		Name:    types.ToPtr("My Pipeline"),
	}).Return(&build.GetDefinitionsResponseValue{
		Value: []build.BuildDefinitionReference{
			{Id: types.ToPtr(42), Name: types.ToPtr("My Pipeline")},
			{Id: types.ToPtr(99), Name: types.ToPtr("My Pipeline")},
		},
	}, nil)

	_, err := ResolvePipelineDefinition(ctx, client, "Fabrikam", "My Pipeline")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestResolvePipelineDefinition_NameNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockBuildClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).Times(1)
	client.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(&build.GetDefinitionsResponseValue{}, nil)

	_, err := ResolvePipelineDefinition(ctx, client, "Fabrikam", "Ghost")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolvePipelineDefinition_QueryError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockBuildClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).Times(1)
	client.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(nil, errors.New("lookup failed"))

	_, err := ResolvePipelineDefinition(ctx, client, "Fabrikam", "My Pipeline")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to query pipeline definitions")
}

func TestResolvePipelineDefinition_EmptyID(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ctx := mocks.NewMockCmdContext(ctrl)
	client := mocks.NewMockBuildClient(ctrl)
	ctx.EXPECT().Context().Return(context.Background()).Times(1)
	client.EXPECT().GetDefinitions(gomock.Any(), gomock.Any()).Return(&build.GetDefinitionsResponseValue{
		Value: []build.BuildDefinitionReference{{Name: types.ToPtr("My Pipeline")}},
	}, nil)

	_, err := ResolvePipelineDefinition(ctx, client, "Fabrikam", "My Pipeline")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned empty id")
}
