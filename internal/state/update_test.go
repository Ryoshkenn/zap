package state

import (
	"testing"
	"time"
)

// A state.json written before the update feature existed has no auto_check
// key. That must read as "on", not as an explicit opt-out.
func TestAutoCheckDefaultsOnForLegacyState(t *testing.T) {
	s := &State{}
	if !s.AutoCheckUpdates() {
		t.Error("AutoCheckUpdates should default to true when unset")
	}
}

func TestSetAutoCheckUpdates(t *testing.T) {
	s := &State{}
	s.SetAutoCheckUpdates(false)
	if s.AutoCheckUpdates() {
		t.Error("expected auto-check to be off after SetAutoCheckUpdates(false)")
	}
	s.SetAutoCheckUpdates(true)
	if !s.AutoCheckUpdates() {
		t.Error("expected auto-check to be on after SetAutoCheckUpdates(true)")
	}
}

func TestUpdateCheckDue(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	s := &State{}
	if !s.UpdateCheckDue(now) {
		t.Error("a state that has never checked is always due")
	}

	s.RecordUpdateCheck(now, "v1.2.0", "https://example.invalid/v1.2.0")
	if s.UpdateCheckDue(now.Add(23 * time.Hour)) {
		t.Error("a check 23h old is still fresh")
	}
	if !s.UpdateCheckDue(now.Add(24 * time.Hour)) {
		t.Error("a check 24h old is due")
	}
	if !s.UpdateCheckDue(now.Add(30 * time.Hour)) {
		t.Error("a check 30h old is due")
	}
}

func TestRecordUpdateCheckPersistsThroughSave(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	now := time.Now().Truncate(time.Second)
	s := &State{}
	s.SetAutoCheckUpdates(false)
	s.RecordUpdateCheck(now, "v1.3.0", "https://example.invalid/v1.3.0")
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AutoCheckUpdates() {
		t.Error("auto-check off did not survive a save/load round trip")
	}
	if got.Updates.LatestVersion != "v1.3.0" {
		t.Errorf("LatestVersion = %q, want v1.3.0", got.Updates.LatestVersion)
	}
	if !got.Updates.LastCheck.Equal(now) {
		t.Errorf("LastCheck = %v, want %v", got.Updates.LastCheck, now)
	}
}
