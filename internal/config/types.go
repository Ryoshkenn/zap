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
	// LaunchMode controls how the provider is started: "terminal" (default) replaces
	// the zap process via exec; "app" fires-and-forgets so a GUI window opens.
	LaunchMode string `yaml:"launch_mode,omitempty"`
	// AppBundle is the macOS .app name (without the .app suffix) used to detect
	// installation via /Applications and to open the app via `open -a`.
	AppBundle string `yaml:"app_bundle,omitempty"`
	// ModelSelector indicates the provider requires a model to be chosen before
	// launch (e.g. Ollama). The model name is passed as an arg at run time.
	ModelSelector bool `yaml:"model_selector,omitempty"`
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
	LaunchMode   string   `yaml:"launch_mode,omitempty"`
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
