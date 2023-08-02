// Package config is a set of types for interacting with the azdo configuration files.
// Note: This package is intended for use only in azdo, any other use cases are subject
// to breakage and non-backwards compatible updates.
package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/tmeckel/azdo-cli/internal/yamlmap"
)

const (
	appData       = "AppData"
	ghConfigDir   = "AZDO_CONFIG_DIR"
	localAppData  = "LocalAppData"
	xdgConfigHome = "XDG_CONFIG_HOME"
	xdgDataHome   = "XDG_DATA_HOME"
	xdgStateHome  = "XDG_STATE_HOME"
)

var (
	instance *configData
	once     sync.Once
	loadErr  error
)

// configData is a in memory representation of the azdo configuration files.
// It can be thought of as map where entries consist of a key that
// correspond to either a string value or a map value, allowing for
// multi-level maps.
type configData struct {
	entries *yamlmap.Map
	mu      sync.RWMutex
}

// Get a string value from a configData.
// The keys argument is a sequence of key values so that nested
// entries can be retrieved. A undefined string will be returned
// if trying to retrieve a key that corresponds to a map value.
// Returns "", KeyNotFoundError if any of the keys can not be found.
func (c *configData) Get(keys []string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := c.entries
	for _, key := range keys {
		var err error
		m, err = m.FindEntry(key)
		if err != nil {
			return "", &KeyNotFoundError{key}
		}
	}
	return m.Value, nil
}

func (c *configData) GetOrDefault(keys []string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := c.entries
	for _, key := range keys {
		var err error
		m, err = m.FindEntry(key)
		if err != nil {
			return defaultFor(key), nil
		}
	}
	return m.Value, nil
}

// Keys enumerates a configData's keys.
// The keys argument is a sequence of key values so that nested
// map values can be have their keys enumerated.
// Returns nil, KeyNotFoundError if any of the keys can not be found.
func (c *configData) Keys(keys []string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := c.entries
	for _, key := range keys {
		var err error
		m, err = m.FindEntry(key)
		if err != nil {
			return nil, &KeyNotFoundError{key}
		}
	}
	return m.Keys(), nil
}

// Remove an entry from a configData.
// The keys argument is a sequence of key values so that nested
// entries can be removed. Removing an entry that has nested
// entries removes those also.
// Returns KeyNotFoundError if any of the keys can not be found.
func (c *configData) Remove(keys []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.entries
	for i := 0; i < len(keys)-1; i++ {
		var err error
		key := keys[i]
		m, err = m.FindEntry(key)
		if err != nil {
			return &KeyNotFoundError{key}
		}
	}
	err := m.RemoveEntry(keys[len(keys)-1])
	if err != nil {
		return &KeyNotFoundError{keys[len(keys)-1]}
	}
	return nil
}

// Set a string value in a configData.
// The keys argument is a sequence of key values so that nested
// entries can be set. If any of the keys do not exist they will
// be created.
func (c *configData) Set(keys []string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.entries
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		entry, err := m.FindEntry(key)
		if err != nil {
			entry = yamlmap.MapValue()
			m.AddEntry(key, entry)
		}
		m = entry
	}
	m.SetEntry(keys[len(keys)-1], yamlmap.StringValue(value))
}

// Read azdo configuration files from the local file system and
// return a configData.
var Read = func() (*configData, error) {
	once.Do(func() {
		instance, loadErr = load(generalConfigFile(), organizationsConfigFile())
	})
	return instance, loadErr
}

// ReadFromString takes a yaml string and returns a configData.
// Note: This is only used for testing, and should not be
// relied upon in production.
func ReadFromString(str string) *configData {
	m, _ := mapFromString(str)
	if m == nil {
		m = yamlmap.MapValue()
	}
	return &configData{entries: m}
}

// Write azdo configuration files to the local file system.
// It will only write azdo configuration files that have been modified
// since last being read.
func Write(c *configData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	hosts, err := c.entries.FindEntry(Organizations)
	if err == nil && hosts.IsModified() {
		err := writeFile(organizationsConfigFile(), []byte(hosts.String()))
		if err != nil {
			return err
		}
		hosts.SetUnmodified()
	}

	if c.entries.IsModified() {
		// Hosts gets written to a different file above so remove it
		// before writing and add it back in after writing.
		organizationsMap, hostsErr := c.entries.FindEntry(Organizations)
		if hostsErr == nil {
			_ = c.entries.RemoveEntry(Organizations)
		}
		err := writeFile(generalConfigFile(), []byte(c.entries.String()))
		if err != nil {
			return err
		}
		c.entries.SetUnmodified()
		if hostsErr == nil {
			c.entries.AddEntry(Organizations, organizationsMap)
		}
	}

	return nil
}

func load(generalFilePath, organizationsFilePath string) (*configData, error) {
	generalMap, err := mapFromFile(generalFilePath)
	if err != nil && !os.IsNotExist(err) {
		if errors.Is(err, yamlmap.ErrInvalidYaml) ||
			errors.Is(err, yamlmap.ErrInvalidFormat) {
			return nil, &InvalidConfigFileError{Path: generalFilePath, Err: err}
		}
		return nil, err
	}

	if generalMap == nil || generalMap.Empty() {
		generalMap, _ = mapFromString(defaultGeneralEntries)
	}

	organizationsMap, err := mapFromFile(organizationsFilePath)
	if err != nil && !os.IsNotExist(err) {
		if errors.Is(err, yamlmap.ErrInvalidYaml) ||
			errors.Is(err, yamlmap.ErrInvalidFormat) {
			return nil, &InvalidConfigFileError{Path: organizationsFilePath, Err: err}
		}
		return nil, err
	}

	if organizationsMap != nil && !organizationsMap.Empty() {
		generalMap.AddEntry(Organizations, organizationsMap)
	}

	return &configData{entries: generalMap}, nil
}

func generalConfigFile() string {
	return filepath.Join(ConfigDir(), "config.yml")
}

func organizationsConfigFile() string {
	return filepath.Join(ConfigDir(), "organizations.yml")
}

func mapFromFile(filename string) (*yamlmap.Map, error) {
	data, err := readFile(filename)
	if err != nil {
		return nil, err
	}
	return yamlmap.Unmarshal(data)
}

func mapFromString(str string) (*yamlmap.Map, error) {
	return yamlmap.Unmarshal([]byte(str))
}

// configData path precedence: AZDO_CONFIG_DIR, XDG_CONFIG_HOME, AppData (windows only), HOME.
func ConfigDir() string {
	var path string
	if a := os.Getenv(ghConfigDir); a != "" {
		path = a
	} else if b := os.Getenv(xdgConfigHome); b != "" {
		path = filepath.Join(b, "azdo")
	} else if c := os.Getenv(appData); runtime.GOOS == "windows" && c != "" {
		path = filepath.Join(c, "AzDO CLI")
	} else {
		d, _ := os.UserHomeDir()
		path = filepath.Join(d, ".config", "azdo")
	}
	return path
}

// State path precedence: XDG_STATE_HOME, LocalAppData (windows only), HOME.
func StateDir() string {
	var path string
	if a := os.Getenv(xdgStateHome); a != "" {
		path = filepath.Join(a, "azdo")
	} else if b := os.Getenv(localAppData); runtime.GOOS == "windows" && b != "" {
		path = filepath.Join(b, "GitHub CLI")
	} else {
		c, _ := os.UserHomeDir()
		path = filepath.Join(c, ".local", "state", "azdo")
	}
	return path
}

// Data path precedence: XDG_DATA_HOME, LocalAppData (windows only), HOME.
func DataDir() string {
	var path string
	if a := os.Getenv(xdgDataHome); a != "" {
		path = filepath.Join(a, "azdo")
	} else if b := os.Getenv(localAppData); runtime.GOOS == "windows" && b != "" {
		path = filepath.Join(b, "AzDO CLI")
	} else {
		c, _ := os.UserHomeDir()
		path = filepath.Join(c, ".local", "share", "azdo")
	}
	return path
}

func readFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func writeFile(filename string, data []byte) (writeErr error) {
	if writeErr = os.MkdirAll(filepath.Dir(filename), 0o771); writeErr != nil {
		return
	}
	var file *os.File
	if file, writeErr = os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600); writeErr != nil {
		return
	}
	defer func() {
		if err := file.Close(); writeErr == nil && err != nil {
			writeErr = err
		}
	}()
	_, writeErr = file.Write(data)
	return
}

var defaultGeneralEntries = `
# What protocol to use when performing git operations. Supported values: ssh, https
git_protocol: https
# What editor azdo should run when creating issues, pull requests, etc. If blank, will refer to environment.
editor:
# When to interactively prompt. This is a global config that cannot be overridden by hostname. Supported values: enabled, disabled
prompt: enabled
# A pager program to send command output to, e.g. "less". Set the value to "cat" to disable the pager.
pager:
# Aliases allow you to create nicknames for azdo commands
aliases:
  co: pr checkout
# The path to a unix socket through which send HTTP connections. If blank, HTTP traffic will be handled by net/http.DefaultTransport.
http_unix_socket:
# What web browser azdo should use when opening URLs. If blank, will refer to environment.
browser:
`
