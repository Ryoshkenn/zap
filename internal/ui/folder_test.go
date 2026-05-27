package ui

import (
	"path/filepath"
	"testing"
)

func TestChildOfUsesPlatformPathJoin(t *testing.T) {
	got := childOf("/tmp/project", "src")
	want := filepath.Join("/tmp/project", "src")
	if got != want {
		t.Fatalf("expected joined path %q, got %q", want, got)
	}
}

func TestParentOfRootStaysRoot(t *testing.T) {
	root := filepath.Clean("/")
	if got := parentOf(root); got != root {
		t.Fatalf("expected root parent to stay root as %q, got %q", root, got)
	}
}
