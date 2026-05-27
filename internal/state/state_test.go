package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withTempCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	// On darwin, UserCacheDir uses ~/Library/Caches and ignores XDG_CACHE_HOME.
	// Override HOME so the macOS path also redirects to tempdir.
	t.Setenv("HOME", dir)
}

func TestSaveLoadRoundtrip(t *testing.T) {
	withTempCache(t)
	s := &State{
		FavoriteFolders:   []string{"/a", "/b"},
		FavoriteProviders: []string{"claude"},
	}
	s.TouchRecent("/a")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.FavoriteFolders) != 2 || got.FavoriteFolders[0] != "/a" {
		t.Errorf("favorites lost: %v", got.FavoriteFolders)
	}
	if len(got.RecentFolders) != 1 || got.RecentFolders[0].Path != "/a" {
		t.Errorf("recents lost: %v", got.RecentFolders)
	}
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	withTempCache(t)
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.FavoriteFolders) != 0 || len(s.RecentFolders) != 0 {
		t.Errorf("expected empty state, got %+v", s)
	}
}

func TestTouchRecentDedupsAndCaps(t *testing.T) {
	s := &State{}
	for i := 0; i < 25; i++ {
		s.TouchRecent("/path/" + string(rune('a'+i%5)))
	}
	if len(s.RecentFolders) > maxRecents {
		t.Errorf("cap broken: %d", len(s.RecentFolders))
	}
	// 5 distinct paths only
	if len(s.RecentFolders) != 5 {
		t.Errorf("expected 5 dedup'd entries, got %d", len(s.RecentFolders))
	}
}

func TestTouchRecentMovesToFront(t *testing.T) {
	s := &State{}
	s.TouchRecent("/a")
	time.Sleep(time.Millisecond)
	s.TouchRecent("/b")
	time.Sleep(time.Millisecond)
	s.TouchRecent("/a")
	if s.RecentFolders[0].Path != "/a" {
		t.Errorf("most recent should be /a, got %s", s.RecentFolders[0].Path)
	}
}

func TestAddRemoveFavorites(t *testing.T) {
	s := &State{}
	if !s.AddFavoriteFolder("/x") {
		t.Error("first add should report new")
	}
	if s.AddFavoriteFolder("/x") {
		t.Error("duplicate add should report not-new")
	}
	if !s.IsFavoriteFolder("/x") {
		t.Error("expected /x to be favorite")
	}
	if !s.RemoveFavoriteFolder("/x") {
		t.Error("remove should succeed")
	}
	if s.IsFavoriteFolder("/x") {
		t.Error("/x should be gone")
	}
}

// Ensure StateDir returns a usable path under test override.
func TestStateDirUsesOverride(t *testing.T) {
	withTempCache(t)
	dir, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("StateDir should be absolute, got %s", dir)
	}
	// sanity: ensure we can mkdir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}
