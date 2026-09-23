package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ryoshkenn/zap/internal/config"
)

func TestProviderStatusDetectsConfiguredAppBundle(t *testing.T) {
	dir := t.TempDir()
	oldAppDirs := appSearchDirs
	appSearchDirs = []string{dir}
	t.Cleanup(func() { appSearchDirs = oldAppDirs })

	appPath := filepath.Join(dir, "Example.app")
	if err := os.Mkdir(appPath, 0o755); err != nil {
		t.Fatal(err)
	}

	st := ProviderStatus(config.Provider{
		ID:        "example",
		Name:      "Example",
		Command:   "definitely-not-on-path-zap-test",
		AppBundle: "Example",
	})

	if !st.Installed {
		t.Fatal("expected app bundle provider to be detected as installed")
	}
	if st.AppBundlePath != appPath {
		t.Fatalf("expected app bundle path %q, got %q", appPath, st.AppBundlePath)
	}
}

func TestProviderStatusPrefersAppBundleOverPathShim(t *testing.T) {
	dir := t.TempDir()
	oldAppDirs := appSearchDirs
	appSearchDirs = []string{dir}
	t.Cleanup(func() { appSearchDirs = oldAppDirs })

	appPath := filepath.Join(dir, "Example.app")
	if err := os.Mkdir(appPath, 0o755); err != nil {
		t.Fatal(err)
	}
	// A CLI shim with the same name on PATH must not shadow the real app.
	binDir := t.TempDir()
	shim := filepath.Join(binDir, "example-shim")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	st := ProviderStatus(config.Provider{ID: "example", Command: "example-shim", AppBundle: "Example"})
	if st.AppBundlePath != appPath {
		t.Fatalf("expected app bundle %q to win over PATH shim, got %+v", appPath, st)
	}
}
