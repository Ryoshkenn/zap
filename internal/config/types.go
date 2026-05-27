package config

// Provider describes a launchable AI CLI.
type Provider struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	Command      string   `yaml:"command"`
	Icon         string   `yaml:"icon,omitempty"`
	InstallHint  string   `yaml:"install_hint,omitempty"`
	Flags        []Flag   `yaml:"flags,omitempty"`
	DefaultFlags []string `yaml:"default_flags,omitempty"`
}

// Flag is a togglable command-line flag exposed in the interactive picker.
type Flag struct {
	ID      string `yaml:"id"`
	Label   string `yaml:"label,omitempty"`
	Flag    string `yaml:"flag"`
	Default bool   `yaml:"default,omitempty"`
}

// DefaultsFile is the schema for the embedded defaults.yaml.
type DefaultsFile struct {
	Providers []Provider `yaml:"providers"`
}

// UserConfig is the schema for ~/.config/zap/config.yaml.
type UserConfig struct {
	Providers       map[string]ProviderOverride `yaml:"providers,omitempty"`
	CustomProviders []Provider                  `yaml:"custom_providers,omitempty"`
}

// ProviderOverride overrides a built-in provider's settings.
type ProviderOverride struct {
	DefaultFlags []string `yaml:"default_flags,omitempty"`
}

// Config is the resolved view used at runtime (embedded defaults ⊕ user overrides).
type Config struct {
	Providers []Provider
}

// FindProvider returns the provider with the given ID, or nil if not found.
func (c *Config) FindProvider(id string) *Provider {
	for i := range c.Providers {
		if c.Providers[i].ID == id {
			return &c.Providers[i]
		}
	}
	return nil
}
