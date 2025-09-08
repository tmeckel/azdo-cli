package azdo

import (
	"context"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/tmeckel/azdo-cli/internal/util"
)

// ConnectionFactory provides connections and org-scoped SDK clients.
type ConnectionFactory interface {
	util.ContextAware
	Connection(organization string) (*azuredevops.Connection, error)
	Git(ctx context.Context, organization string) (azdogit.Client, error)
	Identity(ctx context.Context, organization string) (identity.Client, error)
	Graph(ctx context.Context, organization string) (graph.Client, error)
	Core(ctx context.Context, organization string) (core.Client, error)
	Security(ctx context.Context, organization string) (security.Client, error)
}
