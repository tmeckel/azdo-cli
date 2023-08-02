package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/zalando/go-keyring"
)

const (
	azdoOrganization = "AZDO_ORGANIZATION"
	azdoToken        = "AZDO_TOKEN"
)

type AuthConfig interface {
	GetUrl(organizationName string) (string, error)
	GetGitProtocol(organizationName string) (string, error)
	GetDefaultOrganization() (string, error)
	SetDefaultOrganization(organizationName string) error
	GetOrganizations() []string
	GetToken(organizationName string) (string, error)
	Login(organizationName, organizationUrl, token, gitProtocol string, secureStorage bool) error
	Logout(organizationName string) error
}

// AuthConfig is used for interacting with some persistent configuration for azdo,
// with knowledge on how to access encrypted storage when neccesarry.
// Behavior is scoped to authentication specific tasks.
type authConfig struct {
	cfg Config
}

// Token will retrieve the auth token for the given hostname,
// searching environment variables, plain text config, and
// lastly encrypted storage.
func (c *authConfig) GetToken(hostname string) (token string, err error) {
	token, err = c.GetTokenFromEnvOrConfig(hostname)
	if err != nil {
		if errors.Is(err, &KeyNotFoundError{}) {
			token, err = c.GetTokenFromKeyring(hostname)
		}
	}
	return
}

// TokenFromEnvOrConfig retrieves an authentication token from environment variables or the config
// file as fallback, but does not support reading the token from system keyring. Most consumers
// should use TokenForHost.
func (c *authConfig) GetTokenFromEnvOrConfig(host string) (token string, err error) {
	if token, ok := os.LookupEnv(azdoToken); ok {
		return token, nil
	}
	token, err = c.cfg.Get([]string{Organizations, host, "pat"})
	return
}

// TokenFromKeyring will retrieve the auth token for the given hostname,
// only searching in encrypted storage.
func (c *authConfig) GetTokenFromKeyring(hostname string) (string, error) {
	return keyring.Get(keyringServiceName(hostname), "")
}

// GetUrl will retrieve the url for the Azure DevOps organization
func (c *authConfig) GetUrl(organizationName string) (string, error) {
	return c.cfg.Get([]string{Organizations, organizationName, "url"})
}

// GetGitProtocol will retrieve the git protocol for the logged in user at the given hostname.
// If none is set it will return the default value.
func (c *authConfig) GetGitProtocol(hostname string) (string, error) {
	key := "git_protocol"
	val, err := c.cfg.Get([]string{Organizations, hostname, key})
	if err == nil {
		return val, err
	}
	return defaultFor(key), nil
}

// GetDefaultOrganization will return the default organization for Azure DevOps
// If no default organization is set, the empty string will be returned
func (c *authConfig) GetDefaultOrganization() (organizationName string, err error) {
	if organizationName, ok := os.LookupEnv(azdoOrganization); ok {
		return organizationName, nil
	}

	if organizations := c.GetOrganizations(); len(organizations) == 1 {
		organizationName = organizations[0]
	} else {
		key := "default_organization"
		organizationName, err = c.cfg.Get([]string{key})
		if err != nil {
			if errors.Is(err, &KeyNotFoundError{}) {
				return defaultFor(key), nil
			}
		}
	}
	return
}

// SetDefaultOrganization will set the default organization for Azure DevOps
// If an empty string is passed, the default setting will be cleared
func (c *authConfig) SetDefaultOrganization(organizationName string) (err error) {
	key := "default_organization"
	if organizationName == "" {
		err = c.cfg.Remove([]string{key})
		return
	}
	organizationName = strings.ToLower(organizationName)
	for _, v := range c.GetOrganizations() {
		if v == organizationName {
			goto found
		}
	}
	err = fmt.Errorf("organization not found %s", organizationName)
	return

found:
	c.cfg.Set([]string{key}, organizationName)
	return
}

func (c *authConfig) GetOrganizations() []string {
	hosts := hashset.New()
	if c.cfg != nil {
		keys, err := c.cfg.Keys([]string{Organizations})
		if err == nil {
			for _, v := range keys {
				hosts.Add(v)
			}
		}
	}
	values := hosts.Values()
	items := make([]string, len(values))
	for i, v := range values {
		items[i] = v.(string)
	}
	return items
}

// Login will set user, git protocol, and auth token for the given hostname.
// If the encrypt option is specified it will first try to store the auth token
// in encrypted storage and will fall back to the plain text config file.
func (c *authConfig) Login(organizationName, organizationUrl, token, gitProtocol string, secureStorage bool) error {
	var setErr error
	if secureStorage {
		if setErr = keyring.Set(keyringServiceName(organizationName), "", token); setErr == nil {
			// Clean up the previous oauth_token from the config file.
			_ = c.cfg.Remove([]string{Organizations, organizationName, Pat})
		}
	}
	c.cfg.Set([]string{Organizations, organizationName, "url"}, organizationUrl)
	if !secureStorage || setErr != nil {
		c.cfg.Set([]string{Organizations, organizationName, Pat}, token)
	}
	if gitProtocol != "" {
		c.cfg.Set([]string{Organizations, organizationName, "git_protocol"}, gitProtocol)
	}
	return c.cfg.Write()
}

// Logout will remove user, git protocol, and auth token for the given hostname.
// It will remove the auth token from the encrypted storage if it exists there.
func (c *authConfig) Logout(organizationName string) (err error) {
	if organizationName == "" {
		return nil
	}
	err = c.cfg.Remove([]string{Organizations, organizationName})
	if err != nil {
		return
	}
	err = keyring.Delete(keyringServiceName(organizationName), "")
	if err != nil {
		return
	}
	return c.cfg.Write()
}

func keyringServiceName(organizationName string) string {
	return "azdo:" + organizationName
}
