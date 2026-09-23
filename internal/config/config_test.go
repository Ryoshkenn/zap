package config

import (
	"strings"
	"testing"
)

func TestEmbeddedDefaultsParse(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	wantIDs := []string{"claude", "codex", "opencode", "kimi", "commandcode", "gemini"}
	for _, id := range wantIDs {
		if cfg.FindProvider(id) == nil {
			t.Errorf("expected provider %q in defaults", id)
		}
	}
}

func TestMergeOverridesDefaultFlags(t *testing.T) {
	defaults := []Provider{
		{ID: "claude", Command: "claude"},
	}
	user := &UserConfig{
		Providers: map[string]ProviderOverride{
			"claude": {DefaultFlags: []string{"--dangerously-skip-permissions"}},
		},
	}
	cfg := merge(defaults, user)
	p := cfg.FindProvider("claude")
	if p == nil {
		t.Fatal("claude missing")
	}
	if len(p.DefaultFlags) != 1 || p.DefaultFlags[0] != "--dangerously-skip-permissions" {
		t.Errorf("default flags not applied, got %v", p.DefaultFlags)
	}
}

func TestMergeAppendsCustomProviders(t *testing.T) {
	defaults := []Provider{{ID: "claude", Command: "claude"}}
	user := &UserConfig{
		CustomProviders: []Provider{{ID: "mine", Command: "mycli"}},
	}
	cfg := merge(defaults, user)
	if cfg.FindProvider("mine") == nil {
		t.Errorf("custom provider not appended")
	}
	if len(cfg.Providers) != 2 {
		t.Errorf("want 2 providers, got %d", len(cfg.Providers))
	}
}

func TestMergeDoesNotDuplicateCustomWithSameID(t *testing.T) {
	defaults := []Provider{{ID: "claude", Command: "claude"}}
	user := &UserConfig{
		CustomProviders: []Provider{{ID: "claude", Command: "should-not-replace"}},
	}
	cfg := merge(defaults, user)
	if len(cfg.Providers) != 1 {
		t.Errorf("want 1 provider, got %d", len(cfg.Providers))
	}
	if cfg.Providers[0].Command != "claude" {
		t.Errorf("custom provider should not override default by ID collision; got command=%q", cfg.Providers[0].Command)
	}
}

func TestNormalizeFlagsMigratesAliasesAndDropsUnknown(t *testing.T) {
	p := Provider{
		ID: "opencode",
		Flags: []Flag{
			{ID: "yolo", Flag: "--auto", Aliases: []string{"--dangerously-skip-permissions"}},
		},
		DefaultFlags: []string{"--pure"},
	}
	got := p.NormalizeFlags([]string{"--dangerously-skip-permissions", "--auto", "--full-auto", "--pure"})
	want := []string{"--auto", "--pure"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNormalizeFlagsKeepsExplicitEmptyChoice(t *testing.T) {
	p := Provider{Flags: []Flag{{ID: "yolo", Flag: "--yolo"}}}
	got := p.NormalizeFlags([]string{"--gone"})
	if got == nil || len(got) != 0 {
		t.Fatalf("an explicit (now empty) choice must stay non-nil and empty, got %#v", got)
	}
}

func TestNormalizeFlagsPassesThroughUndeclaredProviders(t *testing.T) {
	p := Provider{ID: "custom"}
	in := []string{"--anything"}
	if got := p.NormalizeFlags(in); len(got) != 1 || got[0] != "--anything" {
		t.Fatalf("custom providers without declared flags must pass through, got %v", got)
	}
}

// Every built-in flag must be a real single argv token; a flag with a space
// would reach the CLI as one malformed argument.
func TestDefaultFlagsAreSingleTokens(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range cfg.Providers {
		for _, f := range p.Flags {
			if f.Flag == "" || strings.ContainsAny(f.Flag, " \t") {
				t.Errorf("%s: flag %q must be a single token", p.ID, f.Flag)
			}
		}
	}
}

// The picker lists installed providers in config order, so defaults.yaml
// order is the order users see.
func TestDefaultProviderOrder(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"claude", "codex", "opencode", "kimi", "commandcode", "gemini", "cursor", "vscode", "ollama"}
	if len(cfg.Providers) < len(want) {
		t.Fatalf("expected at least %d providers, got %d", len(want), len(cfg.Providers))
	}
	for i, id := range want {
		if cfg.Providers[i].ID != id {
			t.Errorf("provider %d = %q, want %q", i, cfg.Providers[i].ID, id)
		}
	}
}
