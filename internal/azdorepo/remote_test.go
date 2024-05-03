package azdorepo

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tmeckel/azdo-cli/internal/git"
)

func createRepository(t *testing.T, organization, project, name string) (repo Repository) {
	repo, err := NewWithOrganization(organization, project, name)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	return
}

func Test_Remotes_FindByName(t *testing.T) {
	os.Setenv("AZDO_CONFIG_DIR", "./testdata/config")

	list := Remotes{
		&Remote{
			Remote: &git.Remote{
				Name: "mona",
			},
			repo: createRepository(t, "ORG", "monalisa", "myfork"),
		},
		&Remote{
			Remote: &git.Remote{
				Name: "origin",
			},
			repo: createRepository(t, "ORG", "monalisa", "octo-cat"),
		},
		&Remote{
			Remote: &git.Remote{
				Name: "upstream",
			},
			repo: createRepository(t, "ORG", "hubot", "tools"),
		},
	}

	r, err := list.FindByName("upstream", "origin")
	assert.NoError(t, err)
	assert.Equal(t, "upstream", r.Name)

	r, err = list.FindByName("nonexistent", "*")
	assert.NoError(t, err)
	assert.Equal(t, "mona", r.Name)

	_, err = list.FindByName("nonexistent")
	assert.Error(t, err, "no GitHub remotes found")
}

func Test_TranslateRemotes(t *testing.T) {
	os.Setenv("AZDO_CONFIG_DIR", "./testdata/config")

	publicURL, _ := url.Parse("https://github.com/monalisa/hello")
	originURL, _ := url.Parse("http://example.com/repo")
	defaultorgURL, _ := url.Parse("https://dev.azure.com/DEFAULTORG/project1/_git/repo1")
	exampleorgURL, _ := url.Parse("https://example.com/EXAMPLEORG/project2/_git/repo2")

	gitRemotes := git.RemoteSet{
		&git.Remote{
			Name:     "origin",
			FetchURL: originURL,
		},
		&git.Remote{
			Name:     "public",
			FetchURL: publicURL,
		},
		&git.Remote{
			Name:     "defaultOrg",
			FetchURL: defaultorgURL,
		},
		&git.Remote{
			Name:     "exampleOrg",
			FetchURL: exampleorgURL,
		},
	}

	result := TranslateRemotes(gitRemotes, NewIdentityTranslator())

	if len(result) != 2 {
		t.Fatalf("got %d results", len(result))
	}
	if result[0].Name != "defaultOrg" {
		t.Errorf("got %q", result[0].Name)
	}
	if result[1].Name != "exampleOrg" {
		t.Errorf("got %q", result[1].Name)
	}
}

func Test_FilterByHosts(t *testing.T) {
	os.Setenv("AZDO_CONFIG_DIR", "./testdata/config")

	r1 := &Remote{
		Remote: &git.Remote{
			Name: "mona",
		},
		repo: createRepository(t, "DEFAULTORG", "myfork", "test.com"),
	}
	r2 := &Remote{
		Remote: &git.Remote{
			Name: "origin",
		},
		repo: createRepository(t, "EXAMPLEORG", "octo-cat", "example.com"),
	}
	r3 := &Remote{
		Remote: &git.Remote{
			Name: "upstream",
		},
		repo: createRepository(t, "ORG", "hubot", "tools"),
	}
	list := Remotes{r1, r2, r3}
	f := list.FilterByOrganization([]string{"DEFAULTORG", "ORG"})
	assert.Equal(t, 2, len(f))
	assert.Equal(t, r1, f[0])
	assert.Equal(t, r3, f[1])
}
