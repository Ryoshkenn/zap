package cmd

import (
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
