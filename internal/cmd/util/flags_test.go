package util

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNilStringFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want *string
	}{
		{name: "omitted", args: nil, want: nil},
		{name: "set", args: []string{"--name", "hello"}, want: ptr("hello")},
		{name: "set empty", args: []string{"--name", ""}, want: ptr("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *string
			cmd := &cobra.Command{Use: "test"}
			NilStringFlag(cmd, &got, "name", "", "a name")

			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, *tt.want, *got)
		})
	}
}

func TestNilBoolFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want *bool
	}{
		{name: "omitted", args: nil, want: nil},
		{name: "long true", args: []string{"--verbose"}, want: ptr(true)},
		{name: "long false", args: []string{"--verbose=false"}, want: ptr(false)},
		{name: "short true", args: []string{"-v"}, want: ptr(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *bool
			cmd := &cobra.Command{Use: "test"}
			NilBoolFlag(cmd, &got, "verbose", "v", "verbose mode")

			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, *tt.want, *got)
		})
	}
}

func TestStringEnumFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr string
	}{
		{name: "default", args: nil, want: "auto"},
		{name: "valid", args: []string{"--mode", "manual"}, want: "manual"},
		{name: "case insensitive", args: []string{"--mode", "Manual"}, want: "Manual"},
		{name: "invalid", args: []string{"--mode", "hybrid"}, wantErr: "valid values are {auto|manual}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ""
			cmd := &cobra.Command{Use: "test"}
			StringEnumFlag(cmd, &got, "mode", "", "auto", []string{"auto", "manual"}, "select mode")

			err := cmd.ParseFlags(tt.args)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNilStringEnumFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    *string
		wantErr string
	}{
		{name: "omitted", args: nil, want: nil},
		{name: "valid", args: []string{"--action", "manage"}, want: ptr("manage")},
		{name: "invalid", args: []string{"--action", "view"}, wantErr: "valid values are {none|manage|use}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *string
			cmd := &cobra.Command{Use: "test"}
			NilStringEnumFlag(cmd, &got, "action", "", []string{"none", "manage", "use"}, "filter action")

			err := cmd.ParseFlags(tt.args)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, *tt.want, *got)
		})
	}
}

func TestStringSliceEnumFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		defaultValue []string
		want         []string
		wantErr      string
	}{
		{name: "default", args: nil, defaultValue: []string{"a"}, want: []string{"a"}},
		{name: "valid single", args: []string{"--type", "b"}, want: []string{"b"}},
		{name: "valid multi", args: []string{"--type", "a,b"}, want: []string{"a", "b"}},
		{name: "invalid single", args: []string{"--type", "d"}, wantErr: "valid values are {a|b|c}"},
		{name: "invalid multi", args: []string{"--type", "a,d"}, wantErr: "valid values are {a|b|c}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			cmd := &cobra.Command{Use: "test"}
			StringSliceEnumFlag(cmd, &got, "type", "", tt.defaultValue, []string{"a", "b", "c"}, "select types")

			err := cmd.ParseFlags(tt.args)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNilStringSliceEnumFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		want    []string
		wantNil bool
		wantErr string
	}{
		{name: "omitted", args: nil, wantNil: true},
		{name: "valid", args: []string{"--type", "b"}, want: []string{"b"}},
		{name: "invalid single", args: []string{"--type", "d"}, wantErr: "valid values are {a|b|c}"},
		{name: "invalid multi", args: []string{"--type", "a,d"}, wantErr: "valid values are {a|b|c}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			cmd := &cobra.Command{Use: "test"}
			NilStringSliceEnumFlag(cmd, &got, "type", "", []string{"a", "b", "c"}, "select types")

			err := cmd.ParseFlags(tt.args)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRegisterBranchCompletionFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		flags             []string
		markRepoChanged   bool
		want              []string
		wantDirective     cobra.ShellCompDirective
		wantRegisterError string
	}{
		{
			name:          "returns branch names",
			flags:         []string{"branch"},
			want:          []string{"pref-main", "pref-dev"},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:              "missing flag returns error",
			flags:             []string{"missing"},
			wantRegisterError: "flag 'missing' does not exist",
		},
		{
			name:            "repo changed disables completion",
			flags:           []string{"branch"},
			markRepoChanged: true,
			want:            nil,
			wantDirective:   cobra.ShellCompDirectiveNoFileComp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().String("branch", "", "branch name")
			cmd.Flags().String("repo", "", "repo")

			err := RegisterBranchCompletionFlags(fakeGitClient{}, cmd, tt.flags...)
			if tt.wantRegisterError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantRegisterError)
				return
			}

			require.NoError(t, err)
			if tt.markRepoChanged {
				require.NoError(t, cmd.Flags().Set("repo", "example"))
			}

			completion, ok := cmd.GetFlagCompletionFunc("branch")
			require.True(t, ok)

			got, directive := completion(cmd, nil, "pref-")
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantDirective, directive)
		})
	}
}

type fakeGitClient struct{}

func (fakeGitClient) TrackingBranchNames(_ context.Context, prefix string) []string {
	return []string{prefix + "main", prefix + "dev"}
}

func ptr[T any](v T) *T {
	return &v
}
