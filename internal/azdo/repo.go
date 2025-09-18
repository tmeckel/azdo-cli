package azdo

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/git"
	"github.com/tmeckel/azdo-cli/internal/types"
)

var (
	orgNameRE  = regexp.MustCompile(`^(?P<Org>[\w-_.\s]+)$`)
	projNameRE = regexp.MustCompile(`^((?P<Org>[\w-_.\s]+)\/)?(?P<Prj>[\w-_.\s]+)$`)
	repoNameRE = regexp.MustCompile(`^((?P<Org>[\w-_.\s]+)\/)?(?P<Prj>[\w-_.\s]+)\/(?P<Repo>[\w-_.\s]+)$`)
)

type OrganizationName interface {
	Organization() string
	FullName() string
}

type orgName struct {
	org string
}

var _ OrganizationName = &orgName{}

func ParseOrgnizationName(n string) (OrganizationName, error) {
	m := orgNameRE.FindStringSubmatch(n)
	if m == nil {
		return nil, fmt.Errorf("not a valid repository name, got %q", n)
	}

	return &orgName{
		org: m[orgNameRE.SubexpIndex("Org")],
	}, nil
}

func (n *orgName) Organization() string {
	return n.org
}

func (n *orgName) FullName() string {
	return n.Organization()
}

type ProjectName interface {
	OrganizationName
	Project() string
}

type projectName struct {
	orgName
	proj string
}

var _ ProjectName = &projectName{}

func ParseProjectName(n string) (ProjectName, error) {
	m := projNameRE.FindStringSubmatch(n)
	if m == nil {
		return nil, fmt.Errorf("not a valid repository name, expected the \"[ORGANIZATION/]PROJECT\" format, got %q", n)
	}

	var org, proj string

	for _, g := range []string{"Org", "Prj"} {
		gi := repoNameRE.SubexpIndex(g)
		if gi < 0 || gi > len(m) {
			continue
		}
		switch g {
		case "Org":
			org = m[gi]
		case "Prj":
			proj = m[gi]
		}
	}
	return &projectName{
		orgName: orgName{
			org: org,
		},
		proj: proj,
	}, nil
}

func (n *projectName) Organization() string {
	return n.org
}

func (n *projectName) Project() string {
	return n.proj
}

func (n *projectName) FullName() string {
	on := n.orgName.FullName()
	if on != "" {
		return on + "/" + n.proj
	}
	return n.proj
}

type RepositoryName interface {
	ProjectName
	Name() string
}

type repositoryName struct {
	projectName
	name string
}

var _ RepositoryName = &repositoryName{}

func ParseRepositoryName(n string) (RepositoryName, error) {
	m := repoNameRE.FindStringSubmatch(n)
	if m == nil {
		return nil, fmt.Errorf("not a valid repository name, expected the \"[ORGANIZATION/]PROJECT/REPO\" format, got %q", n)
	}

	var org, proj, repo string

	for _, g := range []string{"Org", "Prj", "Repo"} {
		gi := repoNameRE.SubexpIndex(g)
		if gi < 0 || gi > len(m) {
			continue
		}
		switch g {
		case "Org":
			org = m[gi]
		case "Prj":
			proj = m[gi]
		case "Repo":
			repo = m[gi]
		}
	}
	return &repositoryName{
		projectName: projectName{
			orgName: orgName{
				org: org,
			},
			proj: proj,
		},
		name: repo,
	}, nil
}

func (n *repositoryName) Organization() string {
	return n.org
}

func (n *repositoryName) Project() string {
	return n.proj
}

func (n *repositoryName) Name() string {
	return n.name
}

func (n *repositoryName) FullName() string {
	pn := n.projectName.FullName()
	if pn != "" {
		return pn + "/" + n.name
	}
	return n.name
}

// Repository describes an object that represents an Azure DevOps Git repository.
type Repository interface {
	fmt.Stringer
	RepositoryName

	Hostname() string
	Equals(other Repository) bool
	RemoteUrl(protocol string) (string, error)
	OrganizationUrl() (string, error)
	ProjectUrl() (string, error)
	GitClient(ctx context.Context, connectionFactory ConnectionFactory) (azdogit.Client, error)
	GitRepository(ctx context.Context, repoClient azdogit.Client) (*azdogit.GitRepository, error)
}

type azdo struct {
	organization string
	project      string
	name         string
	hostname     string
}

func (r *azdo) Hostname() string {
	return r.hostname
}

func (r *azdo) Organization() string {
	return r.organization
}

func (r *azdo) Project() string {
	return r.project
}

func (r *azdo) Name() string {
	return r.name
}

func (r *azdo) FullName() string {
	return fmt.Sprintf("%s/%s/%s", r.organization, r.project, r.name)
}

func (r *azdo) String() string {
	return r.FullName()
}

func (r *azdo) Equals(other Repository) bool {
	return normalizeHostname(r.hostname) == normalizeHostname(other.Hostname()) &&
		strings.EqualFold(r.organization, other.Organization()) &&
		strings.EqualFold(r.project, other.Project()) &&
		strings.EqualFold(r.name, other.Name())
}

func (r *azdo) RemoteUrl(protocol string) (string, error) {
	switch strings.ToLower(protocol) {
	case "ssh":
		return fmt.Sprintf("git@ssh.%s:v3/%s/%s/%s",
			r.hostname, r.organization, r.project, r.name), nil
	default:
		return fmt.Sprintf("https://%s/%s/%s/_git/%s",
			r.hostname, r.organization, r.project, r.name), nil
	}
}

func (r *azdo) OrganizationUrl() (url string, err error) {
	url = fmt.Sprintf("https://%s/%s", r.hostname, r.organization)
	return url, err
}

func (r *azdo) ProjectUrl() (url string, err error) {
	orgUrl, err := r.OrganizationUrl()
	if err != nil {
		return url, err
	}
	url = fmt.Sprintf("%s/%s", orgUrl, r.project)
	return url, err
}

func (r *azdo) GitClient(ctx context.Context, connectionFactory ConnectionFactory) (azdogit.Client, error) {
	conn, err := connectionFactory.Connection(r.Organization())
	if err != nil {
		return nil, err
	}

	return azdogit.NewClient(ctx, conn)
}

func (r *azdo) GitRepository(ctx context.Context, repoClient azdogit.Client) (*azdogit.GitRepository, error) {
	repoList, err := repoClient.GetRepositories(ctx, azdogit.GetRepositoriesArgs{
		Project:       types.ToPtr(r.Project()),
		IncludeHidden: types.ToPtr(true),
	})
	if err != nil {
		return nil, err
	}
	if repoList == nil || len(*repoList) == 0 {
		return nil, fmt.Errorf("project %s at organization %s contains no repositories", r.Project(), r.Organization())
	}

	for _, repo := range *repoList {
		if strings.EqualFold(*repo.Name, r.Name()) {
			return &repo, nil
		}
	}

	return nil, fmt.Errorf("repository %s not found in project %s at organization %s", r.Name(), r.Project(), r.Organization())
}

// New creates a new repository using the default organization.
func NewRepository(project, name string) (Repository, error) {
	return NewRepositoryWithOrganization("", project, name)
}

// NewWithOrganization creates a new repository with the specified organization.
func NewRepositoryWithOrganization(organization, project, name string) (Repository, error) {
	if organization == "" {
		cfg, err := config.NewConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create config instance: %w", err)
		}
		o, err := cfg.Authentication().GetDefaultOrganization()
		if err != nil {
			return nil, fmt.Errorf("failed to get default organization: %w", err)
		}
		organization = o
	}

	hostname, err := getHostnameFromOrganization(organization)
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname for organization %q: %w", organization, err)
	}
	return &azdo{
		organization: organization,
		project:      project,
		name:         name,
		hostname:     hostname,
	}, nil
}

func RepositoryFromName(name string) (Repository, error) {
	return parseWithOrganization(name)
}

var rx_azdoHostName = regexp.MustCompile(`^dev\.azure\.com$|\.visualstudio\.com$`)

func IsAzDORemoteURL(u *url.URL) (result bool, err error) {
	if u.Hostname() == "" {
		err = fmt.Errorf("no hostname detected")
		return result, err
	}

	if !git.IsSupportedProtocol(u) {
		err = fmt.Errorf("unsupported protocol %q", u.Scheme)
		return result, err
	}
	result = rx_azdoHostName.Match([]byte(u.Hostname()))
	return result, err
}

// FromURL extracts repository information from a git remote URL.
func RepositoryFromURL(u *url.URL) (Repository, error) {
	if isOk, err := IsAzDORemoteURL(u); err != nil || !isOk {
		if err != nil {
			return nil, err
		}
		if !isOk {
			return nil, fmt.Errorf("url %s is not a valid AzDO remote URL", u.String())
		}
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

	organization := strings.ToLower(parts[0])
	hostname, err := getHostnameFromOrganization(organization)
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(hostname, strings.TrimPrefix(u.Hostname(), "ssh.")) {
		return nil, fmt.Errorf("hostname %q of URL does not match hostname %q of organization %q", u.Hostname(), hostname, parts[0])
	}

	return NewRepositoryWithOrganization(organization, parts[1], strings.TrimSuffix(parts[projectNameIdx], ".git"))
}

// Helper functions.
func parseWithOrganization(s string) (Repository, error) {
	if git.IsURL(s) {
		u, err := git.ParseURL(s)
		if err != nil {
			return nil, err
		}
		return RepositoryFromURL(u)
	}

	n, err := ParseRepositoryName(s)
	if err != nil {
		return nil, err
	}
	org := n.Organization()
	if org == "" {
		cfg, err := config.NewConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create config instance: %w", err)
		}
		o, err := cfg.Authentication().GetDefaultOrganization()
		if err != nil {
			return nil, fmt.Errorf("failed to get default organization: %w", err)
		}
		org = o
	}

	return NewRepositoryWithOrganization(org, n.Project(), n.Name())
}

func getHostnameFromOrganization(organization string) (string, error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return "", err //nolint:error,wrapcheck
	}
	szURL, err := cfg.Authentication().GetURL(organization)
	if err != nil {
		return "", fmt.Errorf("failed to get URL for organization %q: %w", organization, err)
	}
	parsedURL, err := url.Parse(szURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL %q for organization %q: %w", szURL, organization, err)
	}
	return normalizeHostname(parsedURL.Hostname()), nil
}

func normalizeHostname(h string) string {
	return strings.ToLower(strings.TrimPrefix(h, "www."))
}
