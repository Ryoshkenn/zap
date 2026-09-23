package config

import "strings"

// SplitCommand splits a provider's Command field ("opencode", "opencode run")
// into its binary and any baked-in base args. The Command field is documented
// as a single binary name, but custom providers (and historically the built-in
// opencode entry) may include args — detection and launch must LookPath only
// the binary, not the whole string.
func SplitCommand(command string) (bin string, baseArgs []string) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

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
	// Aliases are older spellings of Flag. Preferences saved under an alias
	// (e.g. before the upstream CLI renamed the flag) migrate to Flag.
	Aliases []string `yaml:"aliases,omitempty"`
}

// DefaultFlagSet is the flag set used when the user has saved no preference:
// the configured default_flags plus every declared flag marked default.
func (p Provider) DefaultFlagSet() []string {
	out := append([]string{}, p.DefaultFlags...)
	for _, f := range p.Flags {
		if !f.Default {
			continue
		}
		dup := false
		for _, existing := range out {
			if existing == f.Flag {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, f.Flag)
		}
	}
	return out
}

// NormalizeFlags cleans a saved flag set before it reaches the CLI: aliases
// are rewritten to the current flag, and flags the provider no longer declares
// are dropped (a removed flag makes most CLIs refuse to start). Flags listed
// in DefaultFlags are always kept since the user configured them explicitly.
// Providers that declare no Flags pass through untouched.
func (p Provider) NormalizeFlags(saved []string) []string {
	if len(p.Flags) == 0 || saved == nil {
		return saved
	}
	canonical := map[string]string{}
	for _, f := range p.Flags {
		canonical[f.Flag] = f.Flag
		for _, a := range f.Aliases {
			canonical[a] = f.Flag
		}
	}
	for _, df := range p.DefaultFlags {
		canonical[df] = df
	}
	out := []string{}
	seen := map[string]bool{}
	for _, s := range saved {
		c, ok := canonical[s]
		if !ok || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
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
