package shared

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type Template interface {
	Path() string
	Body() []byte
}

type template struct {
	path string
	body []byte
}

func (t *template) Path() string {
	return t.path
}

func (t *template) Body() []byte {
	return t.body
}

type TemplateManager interface {
	GetTemplate(ctx context.Context, repo azdo.Repository, branch string) (Template, error)
}

type templateManager struct {
	gitClient git.Client
}

func NewTemplateManager(ctx util.CmdContext) (TemplateManager, error) {
	gitClient, err := ctx.RepoContext().GitClient()
	if err != nil {
		return nil, err
	}
	return &templateManager{
		gitClient: gitClient,
	}, nil
}

func (tm *templateManager) GetTemplate(ctx context.Context, repo azdo.Repository, branch string) (Template, error) {
	gitRepo, err := repo.GitRepository(ctx, tm.gitClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	var pathsToCheck []git.GitItemDescriptor

	// Add branch-specific template paths if a branch is specified
	if branch != "" {
		normalizedBranch := strings.ReplaceAll(branch, "/", "_")
		for _, path := range []string{
			".azuredevops/pull_request_template/branches/",
			".vsts/pull_request_template/branches/",
			"docs/pull_request_template/branches/",
			"pull_request_template/branches/",
		} {
			path = path + normalizedBranch + ".md"
			pathsToCheck = append(pathsToCheck, git.GitItemDescriptor{
				Path:        &path,
				Version:     &branch,
				VersionType: &git.GitVersionTypeValues.Branch,
			})
		}
	}

	for _, path := range []string{
		".azuredevops/pull_request_template.md",
		"PULL_REQUEST_TEMPLATE.md",
		"docs/PULL_REQUEST_TEMPLATE.md",
		".github/PULL_REQUEST_TEMPLATE.md",
	} {
		pathsToCheck = append(pathsToCheck, git.GitItemDescriptor{
			Path: &path,
		})
	}

	items, err := tm.gitClient.GetItemsBatch(ctx, git.GetItemsBatchArgs{
		RepositoryId: types.ToPtr(gitRepo.Id.String()),
		RequestData: &git.GitItemRequestData{
			ItemDescriptors: &pathsToCheck,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request template files: %w", err)
	}

	if len((*items)[0]) > 0 {
		item := (*items)[0][0]
		blob, err := tm.gitClient.GetBlobContent(ctx, git.GetBlobContentArgs{
			RepositoryId: types.ToPtr(gitRepo.Id.String()),
			Sha1:         item.CommitId,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to download file contents %q: %w", *item.Path, err)
		}
		defer blob.Close()
		b, err := io.ReadAll(blob)
		if err != nil {
			return nil, fmt.Errorf("unable to read content of file %q: %w", *item.Path, err)
		}
		return &template{
			path: *item.Path,
			body: b,
		}, nil
	}
	return nil, nil
}
