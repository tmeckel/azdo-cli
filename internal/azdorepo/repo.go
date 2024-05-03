package azdorepo

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/git"
)

// Repository describes an object that represents an Azure DevOps Git repository.
type Repository interface {
	Hostname() string
	Organization() string
	Project() string
	Name() string
	FullName() string

	Equals(other Repository) bool
	RemoteUrl(protocol string) (string, error)
}

type azdoRepo struct {
	organization string
	project      string
	name         string
	hostname     string
}

func (r *azdoRepo) Hostname() string {
	return r.hostname
}

func (r *azdoRepo) Organization() string {
	return r.organization
}

func (r *azdoRepo) Project() string {
	return r.project
}

func (r *azdoRepo) Name() string {
	return r.name
}

func (r *azdoRepo) FullName() string {
	return fmt.Sprintf("%s/%s/%s", r.organization, r.project, r.name)
}

func (r *azdoRepo) Equals(other Repository) bool {
	return strings.EqualFold(r.hostname, other.Hostname()) &&
		strings.EqualFold(r.organization, other.Organization()) &&
		strings.EqualFold(r.project, other.Project()) &&
		strings.EqualFold(r.name, other.Name())
}

func (r *azdoRepo) RemoteUrl(protocol string) (string, error) {
	switch strings.ToLower(protocol) {
	case "ssh":
		return fmt.Sprintf("git@ssh.%s:v3/%s/%s/%s",
			r.hostname, r.organization, r.project, r.name), nil
	default:
		return fmt.Sprintf("https://%s/%s/%s/_git/%s",
			r.hostname, r.organization, r.project, r.name), nil
	}
}

// New creates a new repository using the default organization.
func New(project, name string) (Repository, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, err
	}
	defaultOrg, err := cfg.Authentication().GetDefaultOrganization()
	if err != nil {
		return nil, err
	}
	return NewWithOrganization(defaultOrg, project, name)
}

// NewWithOrganization creates a new repository with the specified organization.
func NewWithOrganization(organization, project, name string) (Repository, error) {
	hostname, err := getHostnameFromOrganization(organization)
	if err != nil {
		return nil, err
	}
	return &azdoRepo{
		organization: organization,
		project:      project,
		name:         name,
		hostname:     hostname,
	}, nil
}

// FromFullName creates a repository from a full name using the default organization.
func FromFullName(nwo string) (Repository, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return nil, err
	}
	defaultOrg, err := cfg.Authentication().GetDefaultOrganization()
	if err != nil {
		return nil, err
	}
	return FromFullNameWithOrganization(nwo, defaultOrg)
}

// FromFullNameWithOrganization creates a repository from a full name with the specified organization.
func FromFullNameWithOrganization(nwo, defaultOrg string) (Repository, error) {
	return parseWithOrganization(nwo, defaultOrg)
}

// FromURL extracts repository information from a git remote URL.
func FromURL(u *url.URL) (Repository, error) {
	if u.Hostname() == "" {
		return nil, fmt.Errorf("no hostname detected")
	}

	if !git.IsSupportedProtocol(u) {
		return nil, fmt.Errorf("unsupported protocol %q", u.Scheme)
	}

	parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 5)
	if len(parts) < 3 || len(parts) > 4 {
		return nil, fmt.Errorf("invalid path %q", u.Path)
	}

	for _, part := range parts {
		if len(strings.TrimSpace(part)) == 0 {
			return nil, fmt.Errorf("invalid path %q", u.Path)
		}
	}

	hasGitIndicator := strings.Contains(u.Path, "/_git")
	projectNameIdx := 2
	if u.Scheme == "http" || u.Scheme == "https" {
		if !hasGitIndicator {
			return nil, fmt.Errorf("invalid path %q expecting /_git", u.Path)
		}
		if len(parts) < 4 {
			return nil, fmt.Errorf("invalid path %q", u.Path)
		}
		projectNameIdx = 3
	} else if u.Scheme == "ssh" {
		if hasGitIndicator {
			return nil, fmt.Errorf("invalid path %q expecting no /_git", u.Path)
		}
		if !regexp.MustCompile("v[0-9]+").Match([]byte(parts[0])) {
			return nil, fmt.Errorf("invalid ssh url, expecting protocol version, not %q", parts[0])
		}
		parts = parts[1:]
	} else {
		return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}

	hostname, err := getHostnameFromOrganization(parts[0])
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(hostname, strings.TrimPrefix(u.Hostname(), "ssh.")) {
		return nil, fmt.Errorf("hostname %q of URL does not match hostname %q of organization %q", u.Hostname(), hostname, parts[0])
	}

	return NewWithOrganization(parts[0], parts[1], strings.TrimSuffix(parts[projectNameIdx], ".git"))
}

// Helper functions.

func parseWithOrganization(s, organization string) (Repository, error) {
	if git.IsURL(s) {
		u, err := git.ParseURL(s)
		if err != nil {
			return nil, err
		}
		return FromURL(u)
	}

	parts := strings.Split(s, "/")
	for _, part := range parts {
		if len(strings.TrimSpace(part)) == 0 {
			return nil, fmt.Errorf(`expected the "[ORGANIZATION/]PROJECT/REPO" format, got %q`, s)
		}
	}

	if len(parts) == 2 {
		return NewWithOrganization(organization, parts[0], parts[1])
	} else if len(parts) == 3 {
		return NewWithOrganization(parts[0], parts[1], parts[2])
	}
	return nil, fmt.Errorf(`expected the "[ORGANIZATION/]PROJECT/REPO" format, got %q`, s)
}

func getHostnameFromOrganization(organization string) (string, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return "", err
	}
	szURL, err := cfg.Authentication().GetURL(organization)
	if err != nil {
		return "", err
	}
	parsedURL, err := url.Parse(szURL)
	if err != nil {
		return "", err
	}
	return parsedURL.Hostname(), nil
}
