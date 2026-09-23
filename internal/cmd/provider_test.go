package cmd

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
	"github.com/Ryoshkenn/zap/internal/state"
)

func TestLaunchProviderUsesOpenForAppMode(t *testing.T) {
	oldExec := launchExec
	oldOpen := launchOpen
	t.Cleanup(func() {
		launchExec = oldExec
		launchOpen = oldOpen
	})

	execCalled := false
	openCalled := false
	var gotDir, gotCommand, gotBundle string
	var gotArgs []string

	launchExec = func(string, string, []string, []string) error {
		execCalled = true
		return nil
	}
	launchOpen = func(dir, command string, args []string, appBundlePath string) error {
		openCalled = true
		gotDir = dir
		gotCommand = command
		gotArgs = args
		gotBundle = appBundlePath
		return nil
	}

	st := detect.Status{
		Provider: config.Provider{
			ID:         "cursor",
			Name:       "Cursor",
			Command:    "cursor",
			LaunchMode: "app",
		},
		Installed:     true,
		AppBundlePath: "/Applications/Cursor.app",
	}

	err := launchProvider("/tmp/project", st, []string{"--reuse-window"}, &state.State{})
	if err != nil {
		t.Fatal(err)
	}

	if execCalled {
		t.Fatal("terminal exec was called for app-mode provider")
	}
	if !openCalled {
		t.Fatal("open was not called for app-mode provider")
	}
	if gotDir != "/tmp/project" || gotCommand != "cursor" || gotBundle != "/Applications/Cursor.app" {
		t.Fatalf("unexpected launch call dir=%q command=%q bundle=%q", gotDir, gotCommand, gotBundle)
	}
	if !reflect.DeepEqual(gotArgs, []string{"--reuse-window"}) {
		t.Fatalf("unexpected args: %v", gotArgs)
	}
}

func TestLaunchProviderHonorsSavedAppMode(t *testing.T) {
	oldExec := launchExec
	oldOpen := launchOpen
	t.Cleanup(func() {
		launchExec = oldExec
		launchOpen = oldOpen
	})

	launchExec = func(string, string, []string, []string) error {
		t.Fatal("terminal exec was called despite saved app mode")
		return nil
	}
	openCalled := false
	launchOpen = func(string, string, []string, string) error {
		openCalled = true
		return nil
	}

	st := detect.Status{
		Provider:  config.Provider{ID: "codex", Command: "codex"},
		Installed: true,
	}
	s := &state.State{}
	s.SetLaunchMode("codex", "app")

	if err := launchProvider("/tmp/project", st, nil, s); err != nil {
		t.Fatal(err)
	}
	if !openCalled {
		t.Fatal("open was not called for saved app mode")
	}
}

func TestLaunchProviderUsesExecForTerminalMode(t *testing.T) {
	oldExec := launchExec
	oldOpen := launchOpen
	t.Cleanup(func() {
		launchExec = oldExec
		launchOpen = oldOpen
	})

	execCalled := false
	launchExec = func(dir, command string, args []string, env []string) error {
		execCalled = true
		if len(env) == 0 {
			t.Fatal("expected environment to be forwarded")
		}
		return nil
	}
	launchOpen = func(string, string, []string, string) error {
		t.Fatal("open was called for terminal-mode provider")
		return nil
	}

	st := detect.Status{
		Provider:  config.Provider{ID: "claude", Command: "claude"},
		Installed: true,
	}

	if err := launchProvider("/tmp/project", st, nil, &state.State{}); err != nil {
		t.Fatal(err)
	}
	if !execCalled {
		t.Fatal("exec was not called for terminal-mode provider")
	}
}

// An explicitly saved empty flag set ("all off") must not fall back to the
// provider's DefaultFlags.
func TestResolveFlagsHonorsSavedEmptySet(t *testing.T) {
	p := config.Provider{
		ID:           "opencode",
		Flags:        []config.Flag{{ID: "yolo", Flag: "--auto"}},
		DefaultFlags: []string{"--auto"},
	}
	s := &state.State{}
	s.SetPreferredFlags("opencode", []string{})
	if got := resolveFlags(p, s, false, false); len(got) != 0 {
		t.Fatalf("saved empty set should win over defaults, got %v", got)
	}
}

// Flags saved under a CLI's old spelling are migrated, and flags the CLI no
// longer accepts are dropped instead of crashing the launch.
func TestResolveFlagsMigratesStaleSavedFlags(t *testing.T) {
	p := config.Provider{
		ID: "codex",
		Flags: []config.Flag{
			{ID: "full_auto", Flag: "--approve-for-me", Aliases: []string{"--full-auto"}},
		},
	}
	s := &state.State{}
	s.SetPreferredFlags("codex", []string{"--full-auto", "--removed-flag"})
	got := resolveFlags(p, s, false, false)
	if !reflect.DeepEqual(got, []string{"--approve-for-me"}) {
		t.Fatalf("got %v", got)
	}
}

// Direct launches must match the interactive picker: flags declared with
// `default: true` are on unless the user saved a different choice.
func TestResolveFlagsIncludesDeclaredDefaults(t *testing.T) {
	p := config.Provider{
		ID:    "opencode",
		Flags: []config.Flag{{ID: "yolo", Flag: "--auto", Default: true}},
	}
	if got := resolveFlags(p, &state.State{}, false, false); !reflect.DeepEqual(got, []string{"--auto"}) {
		t.Fatalf("got %v, want [--auto]", got)
	}
	if got := resolveFlags(p, &state.State{}, false, true); len(got) != 0 {
		t.Fatalf("--safe should drop the default yolo flag, got %v", got)
	}
}

func TestUnexpandHome(t *testing.T) {
	home := filepath.Clean(t.TempDir())
	old := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = old })

	sep := string(filepath.Separator)
	cases := map[string]string{
		home:                           "~",
		home + "/api":                  "~/api",
		home + sep + "api" + sep + "x": "~/api/x",
		"/srv/app":                     "/srv/app",
		"~/already":                    "~/already",
		home + "sibling/api":           home + "sibling/api",
	}
	for in, want := range cases {
		if got := unexpandHome(in); got != want {
			t.Errorf("unexpandHome(%q) = %q, want %q", in, got, want)
		}
	}
}
