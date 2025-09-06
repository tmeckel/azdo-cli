package config

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/zalando/go-keyring"
	"go.uber.org/zap"
	"golang.org/x/text/encoding/unicode"
)

const (
	azdoOrganization = "AZDO_ORGANIZATION"
	azdoToken        = "AZDO_TOKEN"
)

type AuthConfig interface {
	GetURL(organizationName string) (string, error)
	GetGitProtocol(organizationName string) (string, error)
	GetDefaultOrganization() (string, error)
	SetDefaultOrganization(organizationName string) error
	GetOrganizations() []string
	GetToken(organizationName string) (string, error)
	Login(organizationName, organizationURL, token, gitProtocol string, secureStorage bool) error
	Logout(organizationName string) error
}

// https://stackoverflow.com/a/53286786/874043
var nativeEndian unicode.Endianness

func init() {
	buf := [2]byte{}
	*(*uint16)(unsafe.Pointer(&buf[0])) = uint16(0xABCD)

	switch buf {
	case [2]byte{0xCD, 0xAB}:
		nativeEndian = unicode.LittleEndian
	case [2]byte{0xAB, 0xCD}:
		nativeEndian = unicode.BigEndian
	default:
		panic("Could not determine native endianness.")
	}
}

// AuthConfig is used for interacting with some persistent configuration for azdo,
// with knowledge on how to access encrypted storage when neccesarry.
// Behavior is scoped to authentication specific tasks.
type authConfig struct {
	cfg Config
}

// Token will retrieve the auth token for the given organizationName,
// searching environment variables, plain text config, and
// lastly encrypted storage.
func (c *authConfig) GetToken(organizationName string) (token string, err error) {
	logger := zap.L().Sugar()

	organizationName = strings.ToLower(organizationName)

	logger.Debugf("getting token for organization %s", organizationName)
	token, err = c.GetTokenFromEnvOrConfig(organizationName)
	if err != nil {
		if errors.Is(err, new(KeyNotFoundError)) {
			logger.Debug("detected KeyNotFoundError trying to get token from keyring")
			token, err = c.GetTokenFromKeyring(organizationName)
		}
	}
	return
}

// TokenFromEnvOrConfig retrieves an authentication token from environment variables or the config
// file as fallback, but does not support reading the token from system keyring. Most consumers
// should use TokenForHost.
func (c *authConfig) GetTokenFromEnvOrConfig(organizationName string) (token string, err error) {
	organizationName = strings.ToLower(organizationName)

	if token, ok := os.LookupEnv(azdoToken); ok {
		return token, nil
	}
	token, err = c.cfg.Get([]string{Organizations, organizationName, "pat"})
	return
}

// TokenFromKeyring will retrieve the auth token for the given organizationName,
// only searching in encrypted storage.
func (c *authConfig) GetTokenFromKeyring(organizationName string) (token string, err error) {
	organizationName = strings.ToLower(organizationName)

	token, err = keyring.Get(keyringServiceName(organizationName), "")
	if err != nil {
		return
	}
	if runtime.GOOS == "windows" {
		// https://gist.github.com/bradleypeabody/185b1d7ed6c0c2ab6cec?permalink_comment_id=4318385#gistcomment-4318385
		decoder := unicode.UTF16(nativeEndian, unicode.IgnoreBOM).NewDecoder()
		utf8bytes, err := decoder.Bytes([]byte(token)) // token contains UTF16
		if err != nil {
			return "", err
		}
		token = string(utf8bytes)
	}
	return
}

// GetUrl will retrieve the url for the Azure DevOps organization
func (c *authConfig) GetURL(organizationName string) (string, error) {
	organizationName = strings.ToLower(organizationName)
	return c.cfg.Get([]string{Organizations, organizationName, "url"})
}

// GetGitProtocol will retrieve the git protocol for the logged in user at the given organizationName.
// If none is set it will return the default value.
func (c *authConfig) GetGitProtocol(organizationName string) (string, error) {
	organizationName = strings.ToLower(organizationName)
	key := "git_protocol"
	val, err := c.cfg.Get([]string{Organizations, organizationName, key})
	if err != nil {
		return defaultFor(key), nil
	}
	return val, nil
}

// GetDefaultOrganization will return the default organization for Azure DevOps
// If no default organization is set, the empty string will be returned
func (c *authConfig) GetDefaultOrganization() (organizationName string, err error) {
	organizationName = strings.ToLower(organizationName)
	if o, ok := os.LookupEnv(azdoOrganization); !ok {
		if organizations := c.GetOrganizations(); len(organizations) == 1 {
			organizationName = organizations[0]
		} else {
			key := "default_organization"
			organizationName, err = c.cfg.Get([]string{key})
			if err != nil {
				if !errors.Is(err, &KeyNotFoundError{}) {
					return
				}
			}
		}
	} else {
		organizationName = o
	}
	organizationName = strings.TrimSpace(organizationName)
	if organizationName == "" {
		return "", fmt.Errorf("no default organization defined")
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

// Login will set user, git protocol, and auth token for the given organizationName.
// If the encrypt option is specified it will first try to store the auth token
// in encrypted storage and will fall back to the plain text config file.
func (c *authConfig) Login(organizationName, organizationURL, token, gitProtocol string, secureStorage bool) error {
	var setErr error

	organizationName = strings.ToLower(organizationName)
	if secureStorage {
		if setErr = keyring.Set(keyringServiceName(organizationName), "", token); setErr == nil {
			// Clean up the previous oauth_token from the config file.
			_ = c.cfg.Remove([]string{Organizations, organizationName, Pat})
		}
	}
	c.cfg.Set([]string{Organizations, organizationName, "url"}, organizationURL)
	if !secureStorage || setErr != nil {
		c.cfg.Set([]string{Organizations, organizationName, Pat}, token)
	}
	if gitProtocol != "" {
		c.cfg.Set([]string{Organizations, organizationName, "git_protocol"}, gitProtocol)
	}
	return c.cfg.Write()
}

// Logout will remove user, git protocol, and auth token for the given organizationName.
// It will remove the auth token from the encrypted storage if it exists there.
func (c *authConfig) Logout(organizationName string) (err error) {
	if organizationName == "" {
		return nil
	}
	organizationName = strings.ToLower(organizationName)
	err = c.cfg.Remove([]string{Organizations, organizationName})
	if err != nil {
		return
	}
	_ = keyring.Delete(keyringServiceName(organizationName), "")
	return c.cfg.Write()
}

func keyringServiceName(organizationName string) string {
	return "azdo:" + organizationName
}
