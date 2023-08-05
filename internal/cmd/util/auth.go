package util

import (
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type Authenticator interface {
	GetAuthorizationHeader(organizationName string) (string, error)
}

type patAuthenticator struct {
	cfg config.Config
}

func NewPatAuthenticator(cfg config.Config) (instance Authenticator, err error) {
	instance = &patAuthenticator{
		cfg: cfg,
	}
	return
}

func (p *patAuthenticator) GetAuthorizationHeader(organizationName string) (hdrValue string, err error) {
	pat, err := p.cfg.Authentication().GetToken(organizationName)
	if err != nil {
		return
	}
	hdrValue = azuredevops.CreateBasicAuthHeaderValue("", pat)
	return
}
