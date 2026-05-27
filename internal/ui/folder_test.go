package ui

import "testing"

func TestChildOfUsesPlatformPathJoin(t *testing.T) {
	got := childOf("/tmp/project", "src")
	if got != "/tmp/project/src" {
		t.Fatalf("expected joined path, got %q", got)
	}
}

func TestParentOfRootStaysRoot(t *testing.T) {
	if got := parentOf("/"); got != "/" {
		t.Fatalf("expected root parent to stay root, got %q", got)
	}
}
