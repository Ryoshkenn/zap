package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed defaults.yaml
var defaultsYAML []byte

// ConfigDir returns the directory where zap's config file lives.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "zap"), nil
}

// ConfigPath returns the resolved path to the user config file (may not exist).
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads the embedded defaults and merges the user config (if any).
func Load() (*Config, error) {
	var defaults DefaultsFile
	if err := yaml.Unmarshal(defaultsYAML, &defaults); err != nil {
		return nil, fmt.Errorf("parse embedded defaults: %w", err)
	}

	user, err := loadUserConfig()
	if err != nil {
		return nil, err
	}

	return merge(defaults.Providers, user), nil
}

func loadUserConfig() (*UserConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return &UserConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UserConfig{}, nil
		}
		return nil, fmt.Errorf("read user config: %w", err)
	}
	var uc UserConfig
	if err := yaml.Unmarshal(data, &uc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &uc, nil
}

func merge(defaults []Provider, user *UserConfig) *Config {
	providers := make([]Provider, len(defaults))
	copy(providers, defaults)

	if user != nil {
		for id, override := range user.Providers {
			for i := range providers {
				if providers[i].ID == id {
					if override.DefaultFlags != nil {
						providers[i].DefaultFlags = override.DefaultFlags
					}
				}
			}
		}
		for _, custom := range user.CustomProviders {
			if findIndex(providers, custom.ID) == -1 {
				providers = append(providers, custom)
			}
		}
	}

	return &Config{Providers: providers}
}

func findIndex(ps []Provider, id string) int {
	for i, p := range ps {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
