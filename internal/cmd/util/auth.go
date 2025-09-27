package util

import (
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type patAuthenticator struct {
	cfg config.Config
}

func NewPatAuthenticator(cfg config.Config) (instance azdo.Authenticator, err error) {
	instance = &patAuthenticator{
		cfg: cfg,
	}
	return instance, err
}

func (p *patAuthenticator) GetPAT(organizationName string) (string, error) {
	return p.cfg.Authentication().GetToken(organizationName)
}

func (p *patAuthenticator) GetAuthorizationHeader(organizationName string) (hdrValue string, err error) {
	pat, err := p.GetPAT(organizationName)
	if err != nil {
		return hdrValue, err
	}
	hdrValue = azuredevops.CreateBasicAuthHeaderValue("", pat)
	return hdrValue, err
}
