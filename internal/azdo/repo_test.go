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
			name:   "too many path components",
			input:  "https://dev.azure.com/defaultorg/monalisa/_git/octo-cat/pulls",
			result: "",
			host:   "",
			err:    errors.New(`invalid path "/defaultorg/monalisa/_git/octo-cat/pulls"`),
		},
		{
			name:   "non-GitHub hostname",
			input:  "https://example.com/exampleorg/one/_git/two",
			result: "exampleorg/one/two",
			host:   "example.com",
			err:    nil,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("AZDO_CONFIG_DIR", "./testdata/config")

			u, err := url.Parse(tt.input)
			if err != nil {
				t.Fatalf("got error %q", err)
			}

			repo, err := RepositoryFromURL(u)
			if err != nil {
				if tt.err == nil {
					t.Fatalf("got error %q", err)
				} else if tt.err.Error() == err.Error() {
					return
				}
				t.Fatalf("got error %q", err)
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
			wantErr: errors.New(`expected the "[ORGANIZATION/]PROJECT/REPO" format, got "OWNER"`),
		},
		{
			name:    "too many elements",
			input:   "a/b/c/d",
			wantErr: errors.New(`expected the "[ORGANIZATION/]PROJECT/REPO" format, got "a/b/c/d"`),
		},
		{
			name:    "blank value",
			input:   "a/",
			wantErr: errors.New(`expected the "[ORGANIZATION/]PROJECT/REPO" format, got "a/"`),
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
			wantOrganization: "ORG",
			wantProject:      "OWNER",
			wantName:         "REPO",
			wantErr:          nil,
		},
		{
			name:             "full URL",
			input:            "https://example.com/exampleorg/OWNER/_git/REPO.git",
			wantHost:         "example.com",
			wantOrganization: "exampleorg",
			wantProject:      "OWNER",
			wantName:         "REPO",
			wantErr:          nil,
		},
		{
			name:    "full URL hostname do not match",
			input:   "https://example.com/ORG/OWNER/_git/REPO.git",
			wantErr: errors.New(`hostname "example.com" of URL does not match hostname "dev.azure.com" of organization "ORG"`),
		},
		{
			name:             "SSH URL",
			input:            "ssh://ssh.dev.azure.com:v3/ORG/PROJECT/REPO",
			wantHost:         "dev.azure.com",
			wantOrganization: "ORG",
			wantProject:      "PROJECT",
			wantName:         "REPO",
			protocol:         "ssh",
			wantErr:          nil,
			wantURL:          "git@ssh.dev.azure.com:v3/ORG/PROJECT/REPO",
		},
		{
			name:    "SSH invalid URL",
			input:   "git@ssh.dev.azure.com:v3/ORG/PROJECT/_git/REPO",
			wantErr: errors.New(`invalid path "/v3/ORG/PROJECT/_git/REPO"`),
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
