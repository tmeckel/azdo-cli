package azdorepo

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tmeckel/azdo-cli/internal/git"
)

// Remotes represents a set of git remotes
type Remotes []*Remote

// FindByName returns the first Remote whose name matches the list
func (r Remotes) FindByName(names ...string) (*Remote, error) {
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
func (r Remotes) FindByRepo(project, name string) (*Remote, error) {
	for _, rem := range r {
		if strings.EqualFold(rem.repo.Project(), project) && strings.EqualFold(rem.repo.Name(), name) {
			return rem, nil
		}
	}
	return nil, fmt.Errorf("no matching remote found")
}

func (r Remotes) ResolvedRemote() (*Remote, error) {
	for _, rr := range r {
		if rr.Resolved != "" {
			return rr, nil
		}
	}
	return nil, fmt.Errorf("no resolved remote found")
}

// Filter remotes by given organization, maintains original order
func (r Remotes) FilterByOrganization(organizations []string) Remotes {
	filtered := make(Remotes, 0)
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
func (r Remotes) Len() int      { return len(r) }
func (r Remotes) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r Remotes) Less(i, j int) bool {
	return remoteNameSortScore(r[i].Name) > remoteNameSortScore(r[j].Name)
}

// Remote represents a git remote mapped to a GitHub repository
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

func TranslateRemotes(gitRemotes git.RemoteSet, translator Translator) (remotes Remotes) {
	for _, r := range gitRemotes {
		var repo Repository
		if r.FetchURL != nil {
			repo, _ = FromURL(translator.Translate(r.FetchURL))
		}
		if r.PushURL != nil && repo == nil {
			repo, _ = FromURL(translator.Translate(r.PushURL))
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
