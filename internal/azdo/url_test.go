package azdo

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseURL(t *testing.T) {
	t.Parallel()

	parse := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		return u
	}

	tests := []struct {
		name        string
		raw         string
		lax         bool
		nilURL      bool
		wantOrg     string
		wantProject string
		wantErr     error
		wantErrText string
	}{
		{
			name:        "dev.azure.com with project",
			raw:         "https://dev.azure.com/defaultorg/monalisa/_git/octo-cat",
			wantOrg:     "defaultorg",
			wantProject: "monalisa",
		},
		{
			name:    "dev.azure.com org-only",
			raw:     "https://dev.azure.com/defaultorg",
			wantOrg: "defaultorg",
		},
		{
			name:    "dev.azure.com empty path",
			raw:     "https://dev.azure.com",
			wantErr: ErrInvalidPath,
		},
		{
			name:        "dev.azure.com trailing slash",
			raw:         "https://dev.azure.com/",
			wantErr:     ErrInvalidPath,
			wantErrText: `invalid path "/"`,
		},
		{
			name:        "visualstudio.com with project",
			raw:         "https://vsorg.visualstudio.com/monalisa/_git/octo-cat",
			wantOrg:     "vsorg",
			wantProject: "monalisa",
		},
		{
			name:    "visualstudio.com org-only",
			raw:     "https://vsorg.visualstudio.com",
			wantOrg: "vsorg",
		},
		{
			name:        "ssh URL",
			raw:         "ssh://ssh.dev.azure.com/v3/defaultorg/monalisa/octo-cat",
			wantOrg:     "defaultorg",
			wantProject: "monalisa",
		},
		{
			name:    "ssh URL without organization",
			raw:     "ssh://ssh.dev.azure.com/v3",
			wantErr: ErrInvalidPath,
		},
		{
			name:    "non-AzDO host strict",
			raw:     "https://github.com/owner/repo",
			wantErr: ErrNotAzDO,
		},
		{
			name:        "non-AzDO host lax",
			raw:         "https://github.com/owner/repo",
			lax:         true,
			wantOrg:     "github.com",
			wantProject: "owner",
		},
		{
			name:        "empty hostname",
			raw:         "https://",
			wantErrText: "url must have a hostname",
		},
		{
			name:        "nil URL",
			nilURL:      true,
			wantErrText: "url must not be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var u *url.URL
			if !tt.nilURL {
				u = parse(tt.raw)
			}
			id, err := ParseURL(u, tt.lax)
			if tt.wantErr != nil || tt.wantErrText != "" {
				require.Error(t, err)
				if tt.wantErrText != "" {
					// The legacy message text is preserved so existing
					// callers relying on error strings keep behaving
					// identically.
					assert.Equal(t, tt.wantErrText, err.Error())
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOrg, id.Organization)
			assert.Equal(t, tt.wantProject, id.Project)
		})
	}
}

func TestIsVisualStudioHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		hostname string
		want     bool
	}{
		{hostname: "vsorg.visualstudio.com", want: true},
		{hostname: "VSORG.visualstudio.com", want: true},
		{hostname: "org.sub.visualstudio.com", want: true},
		{hostname: "dev.azure.com", want: false},
		{hostname: "ssh.dev.azure.com", want: false},
		{hostname: "vsorg.visualstudio.com.evil.example", want: false},
		{hostname: "example.com", want: false},
		{hostname: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, IsVisualStudioHost(tt.hostname))
		})
	}
}
