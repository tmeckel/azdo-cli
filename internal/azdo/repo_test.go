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

func TestOrganizationFromURL(t *testing.T) {
	t.Setenv("AZDO_CONFIG_DIR", "./testdata/config")

	tests := []struct {
		name  string
		input string
		want  string
		err   error
	}{
		{
			name:  "dev.azure.com https",
			input: "https://dev.azure.com/defaultorg/monalisa/_git/octo-cat",
			want:  "defaultorg",
		},
		{
			name:  "visualstudio.com https",
			input: "https://vsorg.visualstudio.com/monalisa/_git/octo-cat",
			want:  "vsorg",
		},
		{
			name:  "ssh URL",
			input: "ssh://ssh.dev.azure.com/v3/defaultorg/monalisa/octo-cat",
			want:  "defaultorg",
		},
		{
			name:  "invalid path",
			input: "https://dev.azure.com/",
			err:   errors.New(`invalid path "/"`),
		},
		{
			name:  "non AzDO URL",
			input: "https://github.com/owner/repo.git",
			err:   errors.New("url https://github.com/owner/repo.git is not a valid AzDO remote URL"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			if err != nil {
				t.Fatalf("got parse error %q", err)
			}

			got, err := OrganizationFromURL(u)
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
				t.Fatalf("unexpected error %q", err)
			}

			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
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
			name:             "ORG:PROJECT/REPO combo",
			input:            "ORG:PROJECT/REPO",
			wantHost:         "dev.azure.com",
			wantOrganization: "ORG",
			wantProject:      "PROJECT",
			wantName:         "REPO",
			wantURL:          "https://dev.azure.com/ORG/PROJECT/_git/REPO",
			wantErr:          nil,
		},
		{
			name:             "git:PROJECT/REPO parsed as name not URL",
			input:            "git:PROJECT/REPO",
			wantHost:         "dev.azure.com",
			wantOrganization: "git",
			wantProject:      "PROJECT",
			wantName:         "REPO",
			wantURL:          "https://dev.azure.com/git/PROJECT/_git/REPO",
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
			name:    "legacy ORGANIZATION/PROJECT/REPO combo",
			input:   "ORG/PROJECT/REPO",
			wantErr: errors.New(`not a valid repository name, legacy ORGANIZATION/PROJECT/REPO form is not supported, use ORG: syntax (expected "[ORG:]PROJECT/REPO"), got "ORG/PROJECT/REPO"`),
		},
		{
			name:    "too few elements",
			input:   "OWNER",
			wantErr: errors.New(`not a valid repository name, expected the "[ORG:]PROJECT/REPO" format, got "OWNER"`),
		},
		{
			name:    "too many elements",
			input:   "a/b/c/d",
			wantErr: errors.New(`not a valid repository name, expected the "[ORG:]PROJECT/REPO" format, got "a/b/c/d"`),
		},
		{
			name:    "too many elements with organization prefix",
			input:   "ORG:a/b/c",
			wantErr: errors.New(`not a valid repository name, expected the "[ORG:]PROJECT/REPO" format, got "ORG:a/b/c"`),
		},
		{
			name:    "blank value",
			input:   "a/",
			wantErr: errors.New(`invalid name "a/": contains empty segment`),
		},
		{
			name:    "multiple colons",
			input:   "ORG:PROJECT:REPO",
			wantErr: errors.New(`invalid name "ORG:PROJECT:REPO": contains multiple colons`),
		},
		{
			name:    "empty organization prefix",
			input:   ":PROJECT/REPO",
			wantErr: errors.New(`invalid name ":PROJECT/REPO": organization must not be empty`),
		},
		{
			name:    "organization with space",
			input:   "bad org:PROJECT/REPO",
			wantErr: errors.New(`invalid name "bad org:PROJECT/REPO": invalid organization name "bad org"`),
		},
		{
			name:    "organization with underscore",
			input:   "bad_org:PROJECT/REPO",
			wantErr: errors.New(`invalid name "bad_org:PROJECT/REPO": invalid organization name "bad_org"`),
		},
		{
			name:    "organization with trailing hyphen",
			input:   "bad-:PROJECT/REPO",
			wantErr: errors.New(`invalid name "bad-:PROJECT/REPO": invalid organization name "bad-"`),
		},
		{
			name:    "colon not directly after organization",
			input:   "ORG/PROJECT:REPO",
			wantErr: errors.New(`invalid name "ORG/PROJECT:REPO": colon must directly follow the organization`),
		},
		{
			name:    "repository name cannot start with underscore",
			input:   "PROJECT/_repo",
			wantErr: errors.New(`repository name "_repo" cannot start with '_' or '.'`),
		},
		{
			name:    "repository name with invalid characters",
			input:   "PROJECT/repo.",
			wantErr: errors.New(`not a valid repository name, expected the "[ORG:]PROJECT/REPO" format, got "PROJECT/repo."`),
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

func TestProjectFromName(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantOrganization string
		wantProject      string
		wantErr          error
	}{
		{
			name:             "PROJECT with default organization",
			input:            "PROJECT",
			wantOrganization: "defaultorg",
			wantProject:      "PROJECT",
		},
		{
			name:             "ORG:PROJECT",
			input:            "ORG:PROJECT",
			wantOrganization: "ORG",
			wantProject:      "PROJECT",
		},
		{
			name:    "legacy ORGANIZATION/PROJECT",
			input:   "ORG/PROJECT",
			wantErr: errors.New(`not a valid project name, legacy ORGANIZATION/PROJECT form is not supported, use ORG: syntax (expected "[ORG:]PROJECT"), got "ORG/PROJECT"`),
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: errors.New(`not a valid project name, expected the "[ORG:]PROJECT" format, got ""`),
		},
		{
			name:    "too many segments",
			input:   "ORG:PROJECT/EXTRA",
			wantErr: errors.New(`not a valid project name, expected the "[ORG:]PROJECT" format, got "ORG:PROJECT/EXTRA"`),
		},
		{
			name:    "empty segment",
			input:   "PROJECT/",
			wantErr: errors.New(`invalid name "PROJECT/": contains empty segment`),
		},
		{
			name:    "organization with leading hyphen",
			input:   "-org:PROJECT/REPO",
			wantErr: errors.New(`invalid name "-org:PROJECT/REPO": invalid organization name "-org"`),
		},
		{
			name:    "organization with single char",
			input:   "o:PROJECT/REPO",
			wantErr: errors.New(`invalid name "o:PROJECT/REPO": invalid organization name "o"`),
		},
		{
			name:    "invalid characters",
			input:   "PROJ$ETC",
			wantErr: errors.New(`not a valid project name, expected the "[ORG:]PROJECT" format, got "PROJ$ETC"`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AZDO_CONFIG_DIR", "./testdata/config")
			p, err := ProjectFromName(tt.input)
			if tt.wantErr != nil {
				if err == nil || err.Error() != tt.wantErr.Error() {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %q", err)
			}
			if p.Organization() != tt.wantOrganization || p.Project() != tt.wantProject {
				t.Fatalf("expected %q/%q, got %q/%q", tt.wantOrganization, tt.wantProject, p.Organization(), p.Project())
			}
		})
	}
}

func TestProjectFromURL(t *testing.T) {
	t.Setenv("AZDO_CONFIG_DIR", "./testdata/config")

	tests := []struct {
		name             string
		input            string
		wantOrganization string
		wantProject      string
		wantErr          error
	}{
		{
			name:             "dev.azure.com URL",
			input:            "https://dev.azure.com/defaultorg/monalisa",
			wantOrganization: "defaultorg",
			wantProject:      "monalisa",
		},
		{
			name:             "visualstudio.com URL",
			input:            "https://vsorg.visualstudio.com/monalisa",
			wantOrganization: "vsorg",
			wantProject:      "monalisa",
		},
		{
			name:    "non-AzDO URL",
			input:   "https://github.com/owner/repo",
			wantErr: errors.New("url https://github.com/owner/repo is not a valid AzDO remote URL"),
		},
		{
			name:    "URL with hostname that does not match org",
			input:   "https://dev.azure.com/exampleorg/monalisa",
			wantErr: errors.New(`hostname "dev.azure.com" of URL does not match configured hostname "example.com" of organization "exampleorg"`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			if err != nil {
				t.Fatalf("got parse error %q", err)
			}

			p, err := ProjectFromURL(u)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr.Error() {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error %q", err)
			}
			if p.Organization() != tt.wantOrganization || p.Project() != tt.wantProject {
				t.Fatalf("expected %q/%q, got %q/%q", tt.wantOrganization, tt.wantProject, p.Organization(), p.Project())
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
