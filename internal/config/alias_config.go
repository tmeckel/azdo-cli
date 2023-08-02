package config

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
	return a.cfg.Get([]string{Aliases, alias})
}

func (a *aliasConfig) Add(alias, expansion string) error {
	a.cfg.Set([]string{Aliases, alias}, expansion)
	return nil
}

func (a *aliasConfig) Delete(alias string) error {
	return a.cfg.Remove([]string{Aliases, alias})
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
