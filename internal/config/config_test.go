package config

import (
	"testing"
)

func TestEmbeddedDefaultsParse(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	wantIDs := []string{"claude", "codex", "gemini", "opencode"}
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
