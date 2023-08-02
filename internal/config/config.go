package config

const (
	Aliases       = "aliases"
	Organizations = "organizations"
	Defaults      = "defaults"
	Pat           = "pat"
)

// This interface describes interacting with some persistent configuration for azdo.
//
//go:generate moq -rm -out config_mock.go . Config
type Config interface {
	Keys([]string) ([]string, error)
	Get([]string) (string, error)
	GetOrDefault([]string) (string, error)
	Set([]string, string)
	Remove([]string) error
	Write() error
	Authentication() AuthConfig
	Aliases() AliasConfig
}

// Implements Config interface
type cfg struct {
	cfg      *configData
	authCfg  *authConfig
	aliasCfg *aliasConfig
}

func NewConfig() (Config, error) {
	c, err := Read()
	if err != nil {
		return nil, err
	}
	cfg := &cfg{
		cfg: c,
	}
	cfg.authCfg = &authConfig{
		cfg: cfg,
	}
	cfg.aliasCfg = &aliasConfig{
		cfg: cfg,
	}
	return cfg, nil
}

func (c *cfg) Keys(keys []string) (values []string, err error) {
	values, err = c.cfg.Keys(keys)
	return
}

func (c *cfg) Get(keys []string) (string, error) {
	return c.cfg.Get(keys)
}

func (c *cfg) GetOrDefault(keys []string) (val string, err error) {
	return c.cfg.GetOrDefault(keys)
}

func (c *cfg) Set(keys []string, value string) {
	c.cfg.Set(keys, value)
}

func (c *cfg) Remove(keys []string) error {
	return c.cfg.Remove(keys)
}

func (c *cfg) Write() error {
	return Write(c.cfg)
}

func (c *cfg) Authentication() AuthConfig {
	return c.authCfg
}

func (c *cfg) Aliases() AliasConfig {
	return c.aliasCfg
}
