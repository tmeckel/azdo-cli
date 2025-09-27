package util_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tmeckel/azdo-cli/internal/util"
)

// mustParse is a helper to parse URLs in tests.
func mustParse(t *testing.T, s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse URL %q: %v", s, err)
	}
	return u
}

func TestDefaultURLComparer_EqualURLs(t *testing.T) {
	cmp := util.NewURLComparer()
	var nilURL *url.URL
	tests := []struct {
		name string
		a, b *url.URL
		want bool
	}{
		{"both nil", nilURL, nilURL, true},
		{"one nil", nilURL, &url.URL{}, false},
		{
			"case insensitive and default port",
			mustParse(t, "HTTP://Example.COM:80/path"),
			mustParse(t, "http://example.com/path/"),
			true,
		},
		{
			"https default port",
			mustParse(t, "https://example.com:443/path"),
			mustParse(t, "https://example.com/path"),
			true,
		},
		{
			"scheme mismatch",
			mustParse(t, "http://example.com"),
			mustParse(t, "https://example.com"),
			false,
		},
		{
			"remove user info",
			mustParse(t, "http://user:pass@example.com/path"),
			mustParse(t, "http://example.com/path"),
			true,
		},
		{
			"sort query params",
			mustParse(t, "http://example.com/path?b=2&a=1"),
			mustParse(t, "http://example.com/path?a=1&b=2"),
			true,
		},
		{
			"remove default port and slash",
			mustParse(t, "HTTP://Example.COM:80/path/"),
			mustParse(t, "http://example.com/path"),
			true,
		},
		{
			"remove fragment",
			mustParse(t, "http://example.com/path#section1"),
			mustParse(t, "http://example.com/path"),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmp.EqualURLs(tt.a, tt.b)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultURLComparer_EqualStrings(t *testing.T) {
	cmp := util.NewURLComparer()
	tests := []struct {
		name    string
		a, b    string
		want    bool
		wantErr bool
	}{
		{"equal urls", "HTTP://Ex.com/", "http://ex.com", true, false},
		{"invalid a", "://bad", "http://example.com", false, true},
		{"invalid b", "http://example.com", "://bad", false, true},
		{"not equal", "http://a.com", "http://b.com", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cmp.EqualStrings(tt.a, tt.b)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
