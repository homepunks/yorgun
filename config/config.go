package config

import (
	"strings"
	"bufio"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Project  string    `toml:"project"`
	Telegram Telegram  `toml:"telegram"`
	Services []Service `toml:"services"`
}

type Telegram struct {
	BotToken string // to be loaded from .env
	ChatIDs []string `toml:"chat_ids"`
}

type Service struct {
	Name     string `toml:"name"`
	Critical bool   `toml:"critical"`
}

func Load(configPath, envPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", configPath, err)
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

	if len(cfg.Telegram.ChatIDs) == 0 {
		return nil, fmt.Errorf("config: at least one telegram chat_id is required")
	}

	token, err := loadEnvValue(envPath, "TG_BOT_TOKEN")
	if err != nil {
		return nil, fmt.Errorf("loading bot token: %w", err)
	}
	if token == "" {
		return nil, fmt.Errorf("config: TG_BOT_TOKEN is not set in %s", envPath)
	}
	cfg.Telegram.BotToken = token

	return &cfg, nil
}

func (c *Config) WatchedServices() map[string]bool {
	m := make(map[string]bool, len(c.Services))
	for _, s := range c.Services {
		m[s.Name] = true
	}

	return m
}

func (c *Config) IsCritical(service string) bool {
	for _, s := range c.Services {
		if s.Name == service {
			return s.Critical
		}
	}
	return false
}

func loadEnvValue(path, key string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1]), nil
		}
	}

	return "", fmt.Errorf("%s not found in %s", key, path)
}
