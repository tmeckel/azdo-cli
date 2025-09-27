package azdo

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
)

func Test_repoFromURL(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		result string
		host   string
		err    error
	}{
		{
			name:   "dev.azure.com URL",
			input:  "https://dev.azure.com/defaultorg/monalisa/_git/octo-cat",
			result: "defaultorg/monalisa/octo-cat",
			host:   "dev.azure.com",
			err:    nil,
		},
		{
			name:  "visualstudio.com Invalid URL",
			input: "https://prefix.org.visualstudio.com/monalisa/_git/octo-cat",
			err:   errors.New("url https://prefix.org.visualstudio.com/monalisa/_git/octo-cat is not a valid AzDO remote URL"),
		},
		{
			name:   "visualstudio.com URL",
			input:  "https://vsorg.visualstudio.com/monalisa/_git/octo-cat",
			result: "vsorg/monalisa/octo-cat",
			host:   "vsorg.visualstudio.com",
			err:    nil,
		},
		{
			name:   "dev.azure.com URL with trailing slash",
			input:  "https://dev.azure.com/defaultorg/monalisa/_git/octo-cat/",
			result: "defaultorg/monalisa/octo-cat",
			host:   "dev.azure.com",
			err:    nil,
		},
		{
			name:   "dev.azure.com URL with trailing .git",
			input:  "http://dev.azure.com/defaultorg/monalisa/_git/octo-cat.git",
			result: "defaultorg/monalisa/octo-cat",
			host:   "dev.azure.com",
			err:    nil,
		},
		{
			name:   "SSH URL",
			input:  "ssh://ssh.dev.azure.com/v3/defaultorg/monalisa/octo-cat",
			result: "defaultorg/monalisa/octo-cat",
			host:   "dev.azure.com",
			err:    nil,
		},
		{
			name:   "SSH URL with trailing .git",
			input:  "ssh://ssh.dev.azure.com/v3/defaultorg/monalisa/octo-cat.git",
			result: "defaultorg/monalisa/octo-cat",
			host:   "dev.azure.com",
			err:    nil,
		},
		{
			name:   "URL with spaces",
			input:  "https://dev.azure.com/defaultorg/My%20Project/_git/My%20Repo",
			result: "defaultorg/My Project/My Repo",
			host:   "dev.azure.com",
			err:    nil,
		},
		{
			name:   "too many path components",
			input:  "https://dev.azure.com/defaultorg/monalisa/_git/octo-cat/pulls",
			result: "",
			host:   "",
			err:    errors.New(`invalid path "/defaultorg/monalisa/_git/octo-cat/pulls"`),
		},
		{
			name:   "dev.azure.com HTTPS+SSH URL",
			input:  "https+ssh://dev.azure.com/defaultorg/monalisa/octo-cat.git",
			result: "defaultorg/monalisa/octo-cat",
			host:   "dev.azure.com",
			err:    errors.New(`unsupported scheme "https+ssh"`),
		},
		{
			name:   "dev.azure.com git URL",
			input:  "git://dev.azure.com/defaultorg/monalisa/octo-cat.git",
			result: "defaultorg/monalisa/octo-cat",
			host:   "dev.azure.com",
			err:    errors.New(`unsupported scheme "git"`),
		},
		{
			name:  "non-AzDO URL",
			input: "https://github.com/owner/repo.git",
			err:   errors.New("url https://github.com/owner/repo.git is not a valid AzDO remote URL"),
		},
		{
			name:  "https URL with no _git",
			input: "https://dev.azure.com/defaultorg/monalisa/octo-cat",
			err:   errors.New(`invalid path "/defaultorg/monalisa/octo-cat" expecting /_git`),
		},
		{
			name:  "SSH URL with _git",
			input: "ssh://ssh.dev.azure.com/v3/defaultorg/monalisa/_git/octo-cat",
			err:   errors.New(`invalid path "/v3/defaultorg/monalisa/_git/octo-cat" expecting no /_git`),
		},
		{
			name:  "SSH URL with invalid version",
			input: "ssh://ssh.dev.azure.com/v2/defaultorg/monalisa/octo-cat",
			err:   errors.New(`invalid ssh url, expecting protocol version at least v3, got "v2"`),
		},
		{
			name:  "URL with empty path segments",
			input: "https://dev.azure.com/defaultorg//_git/octo-cat",
			err:   errors.New(`invalid path "/defaultorg//_git/octo-cat"`),
		},
		{
			name:  "URL with hostname that does not match org",
			input: "https://another.com/defaultorg/monalisa/_git/octo-cat",
			err:   errors.New(`url https://another.com/defaultorg/monalisa/_git/octo-cat is not a valid AzDO remote URL`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("AZDO_CONFIG_DIR", "./testdata/config")

			u, err := url.Parse(tt.input)
			if err != nil {
				t.Fatalf("got error %q", err)
			}

			repo, err := RepositoryFromURL(u)
			if tt.err != nil {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.err)
				}
				if err.Error() != tt.err.Error() {
					t.Fatalf("expected error %q, got %q", tt.err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("got unexpected error: %q", err)
			}

			got := repo.FullName()
			if tt.result != got {
				t.Errorf("expected %q, got %q", tt.result, got)
			}
			if tt.host != repo.Hostname() {
				t.Errorf("expected %q, got %q", tt.host, repo.Hostname())
			}
		})
	}
}

func TestFromFullName(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantHost         string
		wantOrganization string
		wantProject      string
		wantName         string
		protocol         string
		wantURL          string
		wantErr          error
	}{
		{
			name:             "ORG/PROJECT/REPO combo",
			input:            "ORG/PROJECT/REPO",
			wantHost:         "dev.azure.com",
			wantOrganization: "ORG",
			wantProject:      "PROJECT",
			wantName:         "REPO",
			wantURL:          "https://dev.azure.com/ORG/PROJECT/_git/REPO",
			wantErr:          nil,
		},
		{
			name:             "PROJECT/REPO combo",
			input:            "PROJECT/REPO",
			wantHost:         "dev.azure.com",
			wantOrganization: "defaultorg",
			wantProject:      "PROJECT",
			wantName:         "REPO",
			wantURL:          "https://dev.azure.com/defaultorg/PROJECT/_git/REPO",
			wantErr:          nil,
		},
		{
			name:    "too few elements",
			input:   "OWNER",
			wantErr: errors.New(`not a valid repository name, expected the "[ORGANIZATION/]PROJECT/REPO" format, got "OWNER"`),
		},
		{
			name:    "too many elements",
			input:   "a/b/c/d",
			wantErr: errors.New(`not a valid repository name, expected the "[ORGANIZATION/]PROJECT/REPO" format, got "a/b/c/d"`),
		},
		{
			name:    "blank value",
			input:   "a/",
			wantErr: errors.New(`not a valid repository name, expected the "[ORGANIZATION/]PROJECT/REPO" format, got "a/"`),
		},
		{
			name:    "Invalid Git URL",
			input:   "https://dev.azure.com/ORG/OWNER/REPO.git",
			wantErr: errors.New(`invalid path "/ORG/OWNER/REPO.git" expecting /_git`),
		},
		{
			name:             "full URL",
			input:            "https://dev.azure.com/ORG/OWNER/_git/REPO.git",
			wantHost:         "dev.azure.com",
			wantOrganization: "org",
			wantProject:      "OWNER",
			wantName:         "REPO",
			wantURL:          "https://dev.azure.com/org/OWNER/_git/REPO",
			wantErr:          nil,
		},
		{
			name:    "full URL with custom host",
			input:   "https://example.com/exampleorg/OWNER/_git/REPO.git",
			wantErr: errors.New("url https://example.com/exampleorg/OWNER/_git/REPO.git is not a valid AzDO remote URL"),
		},
		{
			name:    "full URL hostname do not match",
			input:   "https://example.com/ORG/OWNER/_git/REPO.git",
			wantErr: errors.New(`url https://example.com/ORG/OWNER/_git/REPO.git is not a valid AzDO remote URL`),
		},
		{
			name:             "SSH URL",
			input:            "ssh://ssh.dev.azure.com/v3/ORG/PROJECT/REPO",
			wantHost:         "dev.azure.com",
			wantOrganization: "org",
			wantProject:      "PROJECT",
			wantName:         "REPO",
			protocol:         "ssh",
			wantURL:          "git@ssh.dev.azure.com:v3/org/PROJECT/REPO",
			wantErr:          nil,
		},
		{
			name:    "SSH invalid URL",
			input:   "git@ssh.dev.azure.com:v3/ORG/PROJECT/_git/REPO",
			wantErr: errors.New(`invalid path "/v3/ORG/PROJECT/_git/REPO" expecting no /_git`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("AZDO_CONFIG_DIR", "./testdata/config")
			r, err := RepositoryFromName(tt.input)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("no error in result, expected %v", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Fatalf("expected error %q, got %q", tt.wantErr.Error(), err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("got error %v", err)
			}
			if r.Hostname() != tt.wantHost {
				t.Errorf("expected host %q, got %q", tt.wantHost, r.Hostname())
			}
			if r.Organization() != tt.wantOrganization {
				t.Errorf("expected organization %q, got %q", tt.wantOrganization, r.Organization())
			}
			if r.Project() != tt.wantProject {
				t.Errorf("expected owner %q, got %q", tt.wantProject, r.Project())
			}
			if r.Name() != tt.wantName {
				t.Errorf("expected name %q, got %q", tt.wantName, r.Name())
			}
			proto := "https"
			if tt.protocol != "" {
				proto = tt.protocol
			}

			wantUrl := tt.wantURL
			if wantUrl == "" {
				wantUrl = tt.input
			}
			wantUrl = strings.TrimSuffix(wantUrl, ".git")
			url, _ := r.RemoteUrl(proto)
			if url != wantUrl {
				t.Errorf("generated url %q does not match input %q", url, wantUrl)
			}
		})
	}
}

func TestFormatRemoteURL(t *testing.T) {
	tests := []struct {
		name             string
		repoHost         string
		repoOrganization string
		repoProject      string
		repoName         string
		protocol         string
		want             string
	}{
		{
			name:             "https protocol",
			repoHost:         "dev.azure.com",
			repoOrganization: "ORG",
			repoProject:      "owner",
			repoName:         "name",
			protocol:         "https",
			want:             "https://dev.azure.com/ORG/owner/_git/name",
		},
		{
			name:             "https protocol local host",
			repoHost:         "example.com",
			repoOrganization: "exampleorg",
			repoProject:      "owner",
			repoName:         "name",
			protocol:         "https",
			want:             "https://example.com/exampleorg/owner/_git/name",
		},
		{
			name:             "ssh protocol",
			repoHost:         "dev.azure.com",
			repoOrganization: "ORG",
			repoProject:      "owner",
			repoName:         "name",
			protocol:         "ssh",
			want:             "git@ssh.dev.azure.com:v3/ORG/owner/name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := azdo{
				hostname:     tt.repoHost,
				organization: tt.repoOrganization,
				project:      tt.repoProject,
				name:         tt.repoName,
			}
			url, err := r.RemoteUrl(tt.protocol)
			if err != nil {
				t.Error(err)
			}
			if url != tt.want {
				t.Errorf("expected url %q, got %q", tt.want, url)
			}
		})
	}
}

// func TestRepoInfoFromURL(t *testing.T) {
// }
