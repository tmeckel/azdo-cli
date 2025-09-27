package azdo

import (
	"context"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/tmeckel/azdo-cli/internal/azdo/extensions"
	"github.com/tmeckel/azdo-cli/internal/config"
)

type connectionFactory struct {
	cfg  config.Config
	auth Authenticator
}

func NewConnectionFactory(cfg config.Config, auth Authenticator) (ConnectionFactory, error) {
	return &connectionFactory{
		cfg:  cfg,
		auth: auth,
	}, nil
}

func (c *connectionFactory) Connection(organization string) (client Connection, err error) {
	organization = strings.ToLower(organization)
	organizationURL, err := c.cfg.Get([]string{config.Organizations, organization, "url"})
	if err != nil {
		return client, err
	}

	pat, err := c.auth.GetPAT(organization)
	if err != nil {
		return client, err
	}
	client = NewPatConnection(organizationURL, pat)
	return client, err
}

type clientFactory struct {
	factory ConnectionFactory
}

func NewClientFactory(factory ConnectionFactory) (ClientFactory, error) {
	return &clientFactory{
		factory: factory,
	}, nil
}

func (c *clientFactory) Git(ctx context.Context, org string) (git.Client, error) {
	conn, err := c.factory.Connection(org)
	if err != nil {
		return nil, err
	}
	return git.NewClient(ctx, conn.(*connectionAdapter).conn)
}

func (c *clientFactory) Identity(ctx context.Context, org string) (identity.Client, error) {
	conn, err := c.factory.Connection(org)
	if err != nil {
		return nil, err
	}
	return identity.NewClient(ctx, conn.(*connectionAdapter).conn)
}

func (c *clientFactory) Graph(ctx context.Context, org string) (graph.Client, error) {
	conn, err := c.factory.Connection(org)
	if err != nil {
		return nil, err
	}
	return graph.NewClient(ctx, conn.(*connectionAdapter).conn)
}

func (c *clientFactory) Core(ctx context.Context, org string) (core.Client, error) {
	conn, err := c.factory.Connection(org)
	if err != nil {
		return nil, err
	}
	return core.NewClient(ctx, conn.(*connectionAdapter).conn)
}

func (c *clientFactory) Security(ctx context.Context, org string) (security.Client, error) {
	conn, err := c.factory.Connection(org)
	if err != nil {
		return nil, err
	}
	return security.NewClient(ctx, conn.(*connectionAdapter).conn), nil
}

func (c *clientFactory) Extensions(ctx context.Context, org string) (extensions.Client, error) {
	conn, err := c.factory.Connection(org)
	if err != nil {
		return nil, err
	}
	return extensions.NewClient(ctx, conn.(*connectionAdapter).conn), nil
}
