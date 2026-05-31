package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
	"github.com/Ryoshkenn/zap/internal/state"
)

func TestBuildHostEntries(t *testing.T) {
	saved := []state.SSHHost{
		{Target: "devbox", Alias: "dev"},
		{Target: "prod"},
	}
	conf := []string{"prod", "staging", "devbox"} // prod & devbox dup the saved ones

	got := buildHostEntries(saved, conf)

	// saved (2) + unique conf (staging) + add row = 4
	if len(got) != 4 {
		t.Fatalf("expected 4 entries, got %d: %+v", len(got), got)
	}
	if got[0].target != "devbox" || !strings.Contains(got[0].label, "dev") {
		t.Errorf("first entry should be saved devbox with alias, got %+v", got[0])
	}
	if got[2].target != "staging" {
		t.Errorf("third entry should be the unique ssh-config host, got %+v", got[2])
	}
	last := got[len(got)-1]
	if last.action != "add" {
		t.Errorf("last entry must be the add action, got %+v", last)
	}
}

func TestBuildHostEntriesEmpty(t *testing.T) {
	got := buildHostEntries(nil, nil)
	if len(got) != 1 || got[0].action != "add" {
		t.Errorf("with no hosts, only the add action should appear, got %+v", got)
	}
}

func TestBuildRemoteProviderItemsExcludesAppAndModel(t *testing.T) {
	a := &app{
		remote:    true,
		sshTarget: "devbox",
		state:     &state.State{},
		statuses: []detect.Status{
			{Provider: config.Provider{ID: "claude", Command: "claude"}},                    // terminal, not installed locally
			{Provider: config.Provider{ID: "cursor", LaunchMode: "app"}, Installed: true},   // GUI app
			{Provider: config.Provider{ID: "ollama", ModelSelector: true}, Installed: true}, // model selector
			{Provider: config.Provider{ID: "codex", Command: "codex"}, Installed: true},     // terminal
		},
	}
	items := buildRemoteProviderItems(a)
	for _, it := range items {
		pi := it.(providerItem)
		if pi.st.Provider.LaunchMode == "app" {
			t.Errorf("app-mode provider %q should be excluded from remote list", pi.st.Provider.ID)
		}
		if pi.st.Provider.ModelSelector {
			t.Errorf("model-selector provider %q should be excluded from remote list", pi.st.Provider.ID)
		}
		if !pi.st.Installed {
			t.Errorf("remote providers should be presented as installed, got %q not installed", pi.st.Provider.ID)
		}
	}
	if len(items) == 0 {
		t.Fatal("expected at least one terminal provider in remote list")
	}
}

// testApp builds a minimal app with a temp state dir for driving transitions.
func testApp(t *testing.T) *app {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("APPDATA", dir)
	a := &app{
		state:  &state.State{},
		screen: screenFolder,
		statuses: []detect.Status{
			{Provider: config.Provider{ID: "claude", Command: "claude"}, Installed: true},
		},
	}
	return a
}

func key(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// Drives the full interactive SSH path: host picker -> add a computer ->
// remote folder (Home) -> provider, then launch. Asserts remote wiring.
func TestInteractiveSSHFlow(t *testing.T) {
	a := testApp(t)

	a.gotoHost()
	if a.screen != screenHost {
		t.Fatalf("expected screenHost, got %v", a.screen)
	}

	// Pick "Add a computer…" (last entry).
	a.host.cursor = len(a.host.entries) - 1
	a.host.Update(key("enter"))
	if a.screen != screenAddHost {
		t.Fatalf("expected screenAddHost, got %v", a.screen)
	}

	// Type a target and connect.
	a.addHost.ti.SetValue("devbox")
	a.addHost.Update(key("enter"))
	if !a.remote || a.sshTarget != "devbox" {
		t.Fatalf("enterRemote did not set remote state: remote=%v target=%q", a.remote, a.sshTarget)
	}
	if a.screen != screenRemoteFolder {
		t.Fatalf("expected screenRemoteFolder, got %v", a.screen)
	}
	if a.state.FindSSHHost("devbox") == nil {
		t.Error("host should have been persisted on connect")
	}

	// Pick "Home (~)" — it's the second-to-last entry (last is "Enter a path").
	a.remoteFolder.cursor = len(a.remoteFolder.entries) - 2
	a.remoteFolder.Update(key("enter"))
	if a.screen != screenProvider {
		t.Fatalf("expected screenProvider, got %v", a.screen)
	}
	if !a.remote || a.chosenFolder != "~" {
		t.Fatalf("remote folder selection lost: remote=%v folder=%q", a.remote, a.chosenFolder)
	}

	// Launch a terminal provider.
	st := &detect.Status{Provider: config.Provider{ID: "claude", Command: "claude"}, Installed: true}
	a.launch(st, nil)
	if a.finalLaunch == nil {
		t.Fatal("launch produced no result")
	}
	if a.finalLaunch.SSHTarget != "devbox" {
		t.Errorf("remote launch lost target: %q", a.finalLaunch.SSHTarget)
	}
	if a.finalLaunch.LaunchMode != "terminal" {
		t.Errorf("remote launch must be terminal mode, got %q", a.finalLaunch.LaunchMode)
	}
}

// Regression: the sticky-remote-flag bug. After backing out of the SSH flow and
// choosing a LOCAL folder, the launch must NOT go over ssh.
func TestRemoteFlagDoesNotLeakIntoLocalLaunch(t *testing.T) {
	a := testApp(t)
	a.enterRemote("devbox", "")
	if !a.remote {
		t.Fatal("precondition: enterRemote should set remote")
	}

	// User hits esc back to the folder list and picks a local folder.
	a.gotoProvider("/local/path")

	if a.remote || a.sshTarget != "" || a.sshShell != "" {
		t.Fatalf("remote state leaked into local path: remote=%v target=%q shell=%q",
			a.remote, a.sshTarget, a.sshShell)
	}

	a.launch(&detect.Status{Provider: config.Provider{ID: "claude", Command: "claude"}}, nil)
	if a.finalLaunch.SSHTarget != "" {
		t.Errorf("local launch must not have an ssh target, got %q", a.finalLaunch.SSHTarget)
	}
}

func TestHostCursorWrapsAround(t *testing.T) {
	m := &hostModel{entries: make([]hostEntry, 3)} // indices 0,1,2
	m.cursor = 0
	m.move(-1)
	if m.cursor != 2 {
		t.Errorf("up at top should wrap to bottom, got %d", m.cursor)
	}
	m.move(1)
	if m.cursor != 0 {
		t.Errorf("down at bottom should wrap to top, got %d", m.cursor)
	}
}

func TestRemoteFolderCursorWrapsAround(t *testing.T) {
	m := &remoteFolderModel{entries: make([]remoteFolderEntry, 2)} // indices 0,1
	m.cursor = 0
	m.move(-1)
	if m.cursor != 1 {
		t.Errorf("up at top should wrap to bottom, got %d", m.cursor)
	}
	m.move(1)
	if m.cursor != 0 {
		t.Errorf("down at bottom should wrap to top, got %d", m.cursor)
	}
}
