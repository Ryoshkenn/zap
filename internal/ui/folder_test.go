package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/state"
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

func TestBuildFolderItemsPutsCurrentFirstAndDedupes(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	s := &state.State{FavoriteFolders: []string{cwd, "/fav"}}
	s.TouchRecent("/fav")
	s.TouchRecent(cwd)
	s.TouchRecent("/recent")

	items := buildFolderItems(s)
	first, ok := items[0].(folderItem)
	if !ok || first.section != "current" || first.path != cwd {
		t.Fatalf("first row must be the current directory, got %+v", items[0])
	}
	counts := map[string]int{}
	for _, it := range items {
		if fi, ok := it.(folderItem); ok && fi.path != "" {
			counts[fi.path]++
		}
	}
	for path, n := range counts {
		if n != 1 {
			t.Errorf("%s listed %d times, want once", path, n)
		}
	}
	if counts["/fav"] != 1 || counts["/recent"] != 1 {
		t.Errorf("favorite and recent folders should both be listed, got %v", counts)
	}
	last, ok := items[len(items)-1].(folderItem)
	if !ok || last.section != "settings" {
		t.Errorf("actions should come last, got %+v", items[len(items)-1])
	}
}

// Typing 'q' into the filter must filter, not quit.
func TestQDoesNotQuitWhileFiltering(t *testing.T) {
	a := testApp(t)
	a.folder = newFolderModel(a)
	a.Update(key("/"))
	if a.folder.list.FilterState() != list.Filtering {
		t.Fatalf("expected filtering state, got %v", a.folder.list.FilterState())
	}
	_, cmd := a.Update(key("q"))
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("q quit the app while typing a filter")
		}
	}
}

func TestScrollWindowKeepsCursorVisible(t *testing.T) {
	cases := []struct{ offset, cursor, n, height, start, end int }{
		{0, 0, 5, 10, 0, 5},    // fits: everything shown
		{0, 12, 20, 10, 3, 13}, // cursor below window scrolls down
		{8, 2, 20, 10, 2, 12},  // cursor above window scrolls up
		{4, 6, 20, 10, 4, 14},  // cursor inside: offset kept
		{15, 19, 20, 10, 10, 20},
	}
	for _, c := range cases {
		start, end := scrollWindow(c.offset, c.cursor, c.n, c.height)
		if start != c.start || end != c.end {
			t.Errorf("scrollWindow(%d,%d,%d,%d) = %d,%d want %d,%d",
				c.offset, c.cursor, c.n, c.height, start, end, c.start, c.end)
		}
	}
}
