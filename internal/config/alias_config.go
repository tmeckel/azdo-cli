package config

import "fmt"

type AliasConfig interface {
	Get(string) (string, error)
	Add(string, string) error
	Delete(string) error
	All() map[string]string
}

type aliasConfig struct {
	cfg Config
}

func (a *aliasConfig) Get(alias string) (string, error) {
	value, err := a.cfg.Get([]string{Aliases, alias})
	if err != nil {
		return "", fmt.Errorf("unable to get alias %q: %w", alias, err)
	}
	return value, nil
}

func (a *aliasConfig) Add(alias, expansion string) error {
	a.cfg.Set([]string{Aliases, alias}, expansion)
	return nil
}

func (a *aliasConfig) Delete(alias string) error {
	err := a.cfg.Remove([]string{Aliases, alias})
	if err != nil {
		return fmt.Errorf("failed to remove alias: %w", err)
	}
	return nil
}

func (a *aliasConfig) All() map[string]string {
	out := map[string]string{}
	keys, err := a.cfg.Keys([]string{Aliases})
	if err != nil {
		return out
	}
	for _, key := range keys {
		val, _ := a.cfg.Get([]string{Aliases, key})
		out[key] = val
	}
	return out
}
