package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/selfupdate"
)

// Version is the running zap version. cmd sets it before starting the TUI so
// the update check knows what to compare against.
var Version = "dev"

// checkTimeout bounds the background check. The TUI is already interactive by
// the time it fires, so a slow or captive network must never make zap feel
// stuck — it just silently stays quiet.
const checkTimeout = 5 * time.Second

// updateCheckedMsg carries the outcome of a release check back into the TUI.
type updateCheckedMsg struct {
	latest    string
	url       string
	available bool
	cached    bool // served from state.json, so nothing needs re-persisting
	manual    bool // user asked from Settings, so report even a null result
	err       error
}

// backgroundUpdateCheck is the Init-time check. It returns nil when the user
// has turned auto-checking off, replays the cached answer when the last check
// is still fresh, and only hits the network once every state.UpdateCheckInterval.
func backgroundUpdateCheck(a *app) tea.Cmd {
	if !a.state.AutoCheckUpdates() {
		return nil
	}
	if !a.state.UpdateCheckDue(time.Now()) {
		cachedLatest := a.state.Updates.LatestVersion
		if cachedLatest == "" {
			return nil
		}
		cachedURL := a.state.Updates.ReleaseURL
		available := selfupdate.ParseVersion(cachedLatest).
			IsNewerThan(selfupdate.ParseVersion(Version))
		return func() tea.Msg {
			return updateCheckedMsg{latest: cachedLatest, url: cachedURL, available: available, cached: true}
		}
	}
	return checkUpdateCmd(false)
}

// checkUpdateCmd queries the releases API off the UI goroutine.
func checkUpdateCmd(manual bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
		defer cancel()

		res, err := selfupdate.Check(ctx, Version)
		if err != nil {
			return updateCheckedMsg{err: err, manual: manual}
		}
		return updateCheckedMsg{
			latest:    res.Release.TagName,
			url:       res.Release.HTMLURL,
			available: res.Available,
			manual:    manual,
		}
	}
}

// applyUpdateCheck records a check result on the app and, for a live network
// result, caches it in state.json.
//
// A failed check deliberately does not touch LastCheck: leaving it stale means
// an offline run retries next time instead of going quiet for a full day.
func (a *app) applyUpdateCheck(msg updateCheckedMsg) {
	if msg.err != nil {
		a.updateErr = msg.err
		return
	}
	a.updateErr = nil
	a.updateLatest = msg.latest
	a.updateURL = msg.url
	a.updateAvailable = msg.available

	if msg.cached {
		return
	}
	a.state.RecordUpdateCheck(time.Now(), msg.latest, msg.url)
	_ = a.state.Save()
}

// updateBanner is the one-line "update available" notice shown under every
// screen. It returns "" when there is nothing to say, so screens that are
// already full do not lose a row to an empty banner.
func (a *app) updateBanner() string {
	if !a.updateAvailable || a.updateLatest == "" {
		return ""
	}
	return "\n" + hintStyle.Render("  ▲ zap "+a.updateLatest+" is available (you have "+Version+") — run `zap update`") + "\n"
}

// describeCheck turns a check result into a short status line for Settings.
// A manual check always says something — silence would read as a broken button.
func describeCheck(msg updateCheckedMsg) string {
	switch {
	case msg.err != nil:
		return "check failed: " + msg.err.Error()
	case msg.available:
		return "update available: " + msg.latest
	case msg.latest != "":
		return "up to date (" + msg.latest + ")"
	default:
		return "up to date"
	}
}
