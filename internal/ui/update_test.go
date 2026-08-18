package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/state"
)

// withVersion swaps the package-level Version for one test.
func withVersion(t *testing.T, v string) {
	t.Helper()
	prev := Version
	Version = v
	t.Cleanup(func() { Version = prev })
}

// recentlyChecked builds a state whose cached check is still fresh.
func recentlyChecked(latest string) *state.State {
	s := &state.State{}
	s.RecordUpdateCheck(time.Now(), latest, "https://example.invalid/"+latest)
	return s
}

func TestBackgroundCheckSkippedWhenAutoCheckOff(t *testing.T) {
	withVersion(t, "v1.2.0")
	s := &state.State{}
	s.SetAutoCheckUpdates(false)

	if cmd := backgroundUpdateCheck(&app{state: s}); cmd != nil {
		t.Error("no check should be scheduled when auto-check is off")
	}
}

// Within the 24h window zap must answer from state.json rather than calling
// the API again on every single launch.
func TestBackgroundCheckUsesCacheWhileFresh(t *testing.T) {
	withVersion(t, "v1.2.0")

	cmd := backgroundUpdateCheck(&app{state: recentlyChecked("v1.3.0")})
	if cmd == nil {
		t.Fatal("expected a cached result command")
	}
	msg, ok := cmd().(updateCheckedMsg)
	if !ok {
		t.Fatalf("expected updateCheckedMsg, got %T", cmd())
	}
	if !msg.cached {
		t.Error("result should be marked cached")
	}
	if !msg.available {
		t.Error("v1.3.0 should read as available to a v1.2.0 build")
	}
	if msg.latest != "v1.3.0" {
		t.Errorf("latest = %q, want v1.3.0", msg.latest)
	}
}

func TestBackgroundCheckCacheReportsUpToDate(t *testing.T) {
	withVersion(t, "v1.3.0")

	cmd := backgroundUpdateCheck(&app{state: recentlyChecked("v1.3.0")})
	if cmd == nil {
		t.Fatal("expected a cached result command")
	}
	msg := cmd().(updateCheckedMsg)
	if msg.available {
		t.Error("the running version should not report an update to itself")
	}
}

// A fresh check with nothing cached (the very first run inside the window)
// must not fabricate a result.
func TestBackgroundCheckNoCachedVersion(t *testing.T) {
	withVersion(t, "v1.2.0")
	s := &state.State{}
	s.RecordUpdateCheck(time.Now(), "", "")

	if cmd := backgroundUpdateCheck(&app{state: s}); cmd != nil {
		t.Error("no cached version means nothing to report")
	}
}

// A failed check must leave LastCheck untouched, so an offline launch retries
// next time instead of suppressing checks for a full day.
func TestFailedCheckDoesNotAdvanceLastCheck(t *testing.T) {
	withVersion(t, "v1.2.0")
	a := &app{state: &state.State{}}

	a.applyUpdateCheck(updateCheckedMsg{err: errFake{}})

	if !a.state.Updates.LastCheck.IsZero() {
		t.Error("a failed check must not record a check time")
	}
	if a.updateErr == nil {
		t.Error("the error should be retained on the app")
	}
}

// A cached result is already persisted; re-saving it would rewrite state.json
// on every launch for no reason.
func TestCachedResultIsNotRePersisted(t *testing.T) {
	withVersion(t, "v1.2.0")
	a := &app{state: &state.State{}}

	a.applyUpdateCheck(updateCheckedMsg{latest: "v1.3.0", available: true, cached: true})

	if !a.state.Updates.LastCheck.IsZero() {
		t.Error("a cached result should not rewrite LastCheck")
	}
	if !a.updateAvailable {
		t.Error("the banner should still be armed from a cached result")
	}
}

func TestUpdateBannerOnlyWhenAvailable(t *testing.T) {
	withVersion(t, "v1.2.0")

	a := &app{}
	if got := a.updateBanner(); got != "" {
		t.Errorf("expected no banner, got %q", got)
	}

	a.updateAvailable = true
	a.updateLatest = "v1.3.0"
	banner := a.updateBanner()
	if !strings.Contains(banner, "v1.3.0") || !strings.Contains(banner, "zap update") {
		t.Errorf("banner should name the version and the command, got %q", banner)
	}
}

func TestDescribeCheck(t *testing.T) {
	cases := []struct {
		msg  updateCheckedMsg
		want string
	}{
		{updateCheckedMsg{err: errFake{}}, "check failed"},
		{updateCheckedMsg{available: true, latest: "v1.3.0"}, "update available: v1.3.0"},
		{updateCheckedMsg{latest: "v1.2.0"}, "up to date (v1.2.0)"},
		{updateCheckedMsg{}, "up to date"},
	}
	for _, c := range cases {
		if got := describeCheck(c.msg); !strings.Contains(got, c.want) {
			t.Errorf("describeCheck(%+v) = %q, want it to contain %q", c.msg, got, c.want)
		}
	}
}

// The Settings screen must expose both the toggle and the manual check.
func TestSettingsHasUpdateRows(t *testing.T) {
	withVersion(t, "v1.2.0")
	a := &app{
		cfg:   &config.Config{},
		state: &state.State{},
	}
	m := newSettingsModel(a)

	var toggle, check *settingsRow
	for i := range m.rows {
		if m.rows[i].isUpdateToggle {
			toggle = &m.rows[i]
		}
		if m.rows[i].isUpdateCheck {
			check = &m.rows[i]
		}
	}
	if toggle == nil {
		t.Fatal("Settings is missing the auto-check toggle")
	}
	if check == nil {
		t.Fatal("Settings is missing the manual check action")
	}
	if !toggle.on {
		t.Error("the toggle should start on, matching the default")
	}
	if !strings.Contains(check.label, "never checked") {
		t.Errorf("a state with no checks should say so, got %q", check.label)
	}
}

// Toggling auto-check off in Settings must reach state, not just the row.
func TestSettingsTogglePersistsAutoCheck(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	withVersion(t, "v1.2.0")

	a := &app{cfg: &config.Config{}, state: &state.State{}}
	m := newSettingsModel(a)

	for i := range m.rows {
		if m.rows[i].isUpdateToggle {
			m.rows[i].on = false
			m.persistRow(&m.rows[i])
		}
	}

	if a.state.AutoCheckUpdates() {
		t.Error("toggling the row off did not persist to state")
	}
}

type errFake struct{}

func (errFake) Error() string { return "no network" }
