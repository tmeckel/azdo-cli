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

	if gitRepo.DefaultBranch == nil {
		return nil, fmt.Errorf("repository %s does not have a default branch", repo.FullName())
	}
	defaultBranch := normalizeBranch(*gitRepo.DefaultBranch)

	pathsToCheck := getTemplatePathsInOrder(branch)

	for _, templatePath := range pathsToCheck {
		content, err := fetchTemplateContent(ctx, tm.gitClient, repo, templatePath, defaultBranch)
		if err != nil {
			return nil, err
		}
		if content != nil {
			return &template{
				path: templatePath,
				body: content,
			}, nil
		}
	}

	return nil, nil
}

func getTemplatePathsInOrder(targetBranch string) []string {
	var paths []string
	baseFolders := []string{
		".azuredevops/",
		".vsts/",
		"docs/",
		"", // Root folder
	}

	// Branch-specific templates
	if targetBranch != "" {
		normalizedBranch := normalizeBranch(targetBranch)
		for _, folder := range baseFolders {
			paths = append(paths, folder+"pull_request_template/branches/"+normalizedBranch+".md")
			paths = append(paths, folder+"pull_request_template/branches/"+normalizedBranch+".txt")
		}
	}

	// Default templates
	for _, folder := range baseFolders {
		paths = append(paths, folder+"pull_request_template.md")
		paths = append(paths, folder+"pull_request_template.txt")
	}

	return paths
}

func fetchTemplateContent(ctx context.Context, gitClient git.Client, repo azdo.Repository, templatePath string, defaultBranch string) ([]byte, error) {
	// Use GetItemContent to directly get the file content
	azRepo, err := repo.GitRepository(ctx, gitClient)
	if err != nil {
		return nil, err
	}
	args := git.GetItemContentArgs{
		RepositoryId: types.ToPtr(azRepo.Id.String()),
		Project:      types.ToPtr(repo.Project()),
		Path:         types.ToPtr(templatePath),
		VersionDescriptor: &git.GitVersionDescriptor{
			Version:     types.ToPtr(defaultBranch),
			VersionType: types.ToPtr(git.GitVersionTypeValues.Branch),
		},
	}

	contentStream, err := gitClient.GetItemContent(ctx, args)
	if err != nil {
		// Check if it's a 404 Not Found error
		if util.IsNotFoundError(err) { // Assuming util.IsNotFoundError exists or needs to be created
			return nil, nil // Not found, not an error for our search logic
		}
		return nil, fmt.Errorf("failed to get content for template %q from default branch %q: %w", templatePath, defaultBranch, err)
	}
	defer contentStream.Close()

	content, err := io.ReadAll(contentStream)
	if err != nil {
		return nil, fmt.Errorf("failed to read content for template %q: %w", templatePath, err)
	}

	if len(content) == 0 {
		return nil, nil // Treat empty file as not found for template purposes
	}

	return content, nil
}

// normalizeBranch removes "refs/heads/" prefix if present
func normalizeBranch(b string) string {
	return strings.TrimPrefix(b, "refs/heads/")
}
