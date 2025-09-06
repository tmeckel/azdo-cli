package azdo

import (
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/tmeckel/azdo-cli/internal/util"
)

type ConnectionFactory interface {
	util.ContextAware
	Connection(organization string) (*azuredevops.Connection, error)
}
