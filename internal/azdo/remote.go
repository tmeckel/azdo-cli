package azdo

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tmeckel/azdo-cli/internal/git"
)

// RemoteSet represents a set of git remotes which point to an AzDO endpoint
type RemoteSet []*Remote

// FindByName returns the first Remote whose name matches the list
func (r RemoteSet) FindByName(names ...string) (*Remote, error) {
	for _, name := range names {
		for _, rem := range r {
			if rem.Name == name || name == "*" {
				return rem, nil
			}
		}
	}
	return nil, fmt.Errorf("no matching remote found")
}

// FindByRepo returns the first Remote that points to a specific Azure DevOps repository
func (r RemoteSet) FindByRepo(repo Repository) (*Remote, error) {
	for _, rem := range r {
		if rem.repo.Equals(repo) {
			return rem, nil
		}
	}
	return nil, fmt.Errorf("no matching remote found")
}

func (r RemoteSet) DefaultRemote() (*Remote, error) {
	if len(r) == 1 {
		ok, err := IsAzDORemoteURL(r[0].FetchURL)
		if err != nil {
			return nil, err
		}
		if ok {
			return r[0], nil
		}
	} else {
		for _, rr := range r {
			if rr.Resolved != "default" {
				return rr, nil
			}
		}
	}
	return nil, fmt.Errorf("no default remote found")
}

// Filter remotes by given organization, maintains original order
func (r RemoteSet) FilterByOrganization(organizations ...string) RemoteSet {
	filtered := make(RemoteSet, 0)
	for _, rr := range r {
		for _, organization := range organizations {
			if strings.EqualFold(rr.repo.Organization(), organization) {
				filtered = append(filtered, rr)
				break
			}
		}
	}
	return filtered
}

func remoteNameSortScore(name string) int {
	switch strings.ToLower(name) {
	case "upstream":
		return 3
	case "azdo":
		return 2
	case "origin":
		return 1
	default:
		return 0
	}
}

// https://golang.org/pkg/sort/#Interface
func (r RemoteSet) Len() int      { return len(r) }
func (r RemoteSet) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r RemoteSet) Less(i, j int) bool {
	return remoteNameSortScore(r[i].Name) > remoteNameSortScore(r[j].Name)
}

// Remote represents a git remote mapped to a AzDO repository
type Remote struct {
	*git.Remote
	repo Repository
}

func (r Remote) Repository() Repository {
	return r.repo
}

type Translator interface {
	Translate(*url.URL) *url.URL
}

type identityTranslator struct{}

func (it identityTranslator) Translate(u *url.URL) *url.URL {
	return u
}

func NewIdentityTranslator() Translator {
	return identityTranslator{}
}

func TranslateRemotes(gitRemotes git.RemoteSet, translator Translator) (remotes RemoteSet) {
	for _, r := range gitRemotes {
		var repo Repository
		if r.FetchURL != nil {
			if isOk, _ := IsAzDORemoteURL(r.FetchURL); !isOk {
				continue
			}
			repo, _ = RepositoryFromURL(translator.Translate(r.FetchURL))
		}
		if r.PushURL != nil && repo == nil {
			if isOk, _ := IsAzDORemoteURL(r.PushURL); !isOk {
				continue
			}
			repo, _ = RepositoryFromURL(translator.Translate(r.PushURL))
		}
		if repo == nil {
			continue
		}
		remotes = append(remotes, &Remote{
			Remote: r,
			repo:   repo,
		})
	}
	return
}
