package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Project  string    `toml:"project"`
	Services []Service `toml:"services"`
}

type Service struct {
	Name     string `toml:"name"`
	Critical bool   `toml:"critical"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Project == "" {
		return nil, fmt.Errorf("config: project name is required")
	}

	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("config: at least one service is required")
	}

	return &cfg, nil
}

func (c *Config) ServiceNames() []string {
	names := make([]string, len(c.Services))
	for i, s := range c.Services {
		names[i] = s.Name
	}
	return names
}

func (c *Config) IsCritical(service string) bool {
	for _, s := range c.Services {
		if s.Name == service {
			return s.Critical
		}
	}
	return false
}
