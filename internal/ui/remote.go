package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/sshconf"
	"github.com/Ryoshkenn/zap/internal/state"
)

// hostEntry is one row in the remote-host picker.
type hostEntry struct {
	label  string
	target string
	shell  string
	action string // "" => a host; "add" => the "add a computer" action
}

// buildHostEntries merges saved hosts with ~/.ssh/config aliases (deduped by
// target, saved hosts first) and appends the "add a computer" action. Pure and
// unit-tested — this is where a host-list bug would otherwise hide.
func buildHostEntries(saved []state.SSHHost, confHosts []string) []hostEntry {
	var out []hostEntry
	seen := map[string]bool{}
	for _, h := range saved {
		seen[h.Target] = true
		out = append(out, hostEntry{label: "💻 " + h.Label(), target: h.Target, shell: h.Shell})
	}
	for _, c := range confHosts {
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, hostEntry{label: "📡 " + c + "  (ssh config)", target: c})
	}
	out = append(out, hostEntry{label: "➕ Add a computer…", action: "add"})
	return out
}

// hostModel is the remote-host picker.
type hostModel struct {
	app     *app
	entries []hostEntry
	cursor  int
}

func newHostModel(a *app) *hostModel {
	return &hostModel{app: a, entries: buildHostEntries(a.state.SSHHosts, sshconf.Hosts())}
}

func (m *hostModel) Init() tea.Cmd { return nil }

func (m *hostModel) refresh() {
	m.entries = buildHostEntries(m.app.state.SSHHosts, sshconf.Hosts())
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// move advances the cursor by dir, wrapping around at either end.
func (m *hostModel) move(dir int) {
	if n := len(m.entries); n > 0 {
		m.cursor = (m.cursor + dir + n) % n
	}
}

func (m *hostModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "enter":
			e := m.entries[m.cursor]
			if e.action == "add" {
				return m.app, m.app.gotoAddHost()
			}
			return m.app, m.app.enterRemote(e.target, e.shell)
		case "d":
			// Forget a saved host (no-op for ssh-config-only or the add row).
			e := m.entries[m.cursor]
			if e.action == "" && m.app.state.FindSSHHost(e.target) != nil {
				m.app.state.RemoveSSHHost(e.target)
				_ = m.app.state.Save()
				m.refresh()
			}
		case "esc":
			m.app.screen = screenFolder
			return m.app, nil
		}
	}
	return m.app, nil
}

func (m *hostModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("SSH / Remote — pick a computer"))
	b.WriteString("\n\n")
	for i, e := range m.entries {
		marker := "  "
		label := e.label
		if i == m.cursor {
			marker = highlightStyle.Render("▸ ")
			label = highlightStyle.Render(label)
		}
		fmt.Fprintf(&b, "%s%s\n", marker, label)
		if e.action == "" && e.target != e.label {
			fmt.Fprintf(&b, "    %s\n", mutedStyle.Render(e.target))
		}
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · enter select · d forget host · esc back · ctrl+c quit"))
	return b.String()
}

// addHostModel is the "add a computer" text input.
type addHostModel struct {
	app *app
	ti  textinput.Model
}

func newAddHostModel(a *app) *addHostModel {
	ti := textinput.New()
	ti.Placeholder = "alias or user@host (e.g. devbox or me@10.0.0.5)"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 48
	return &addHostModel{app: a, ti: ti}
}

func (m *addHostModel) Init() tea.Cmd { return textinput.Blink }

func (m *addHostModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "enter":
			target := strings.TrimSpace(m.ti.Value())
			if target == "" {
				return m.app, nil
			}
			return m.app, m.app.enterRemote(target, "")
		case "esc":
			return m.app, m.app.gotoHost()
		}
	}
	var cmd tea.Cmd
	m.ti, cmd = m.ti.Update(msg)
	return m.app, cmd
}

func (m *addHostModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Add a computer"))
	b.WriteString("\n\n")
	b.WriteString("  SSH target (an alias from ~/.ssh/config, or user@host):\n\n  ")
	b.WriteString(m.ti.View())
	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("  zap connects with your normal ssh keys/config — no passwords stored."))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("enter connect · esc back"))
	return b.String()
}

// remoteFolderModel picks the remote working directory: recents for this host,
// home, or a typed path. Remote browsing isn't supported (v1) — paths are
// entered or reused rather than browsed like the local picker.
type remoteFolderModel struct {
	app       *app
	entries   []remoteFolderEntry
	cursor    int
	inputting bool
	ti        textinput.Model
}

type remoteFolderEntry struct {
	label  string
	path   string
	action string // "" => path; "input" => switch to text entry
}

func newRemoteFolderModel(a *app) *remoteFolderModel {
	var entries []remoteFolderEntry
	for _, r := range a.state.RemoteRecents(a.sshTarget, 8) {
		if r.Path == "~" {
			continue // home is always offered explicitly below
		}
		entries = append(entries, remoteFolderEntry{label: "🕘 " + r.Path, path: r.Path})
	}
	entries = append(entries, remoteFolderEntry{label: "🏠 Home (~)", path: "~"})
	entries = append(entries, remoteFolderEntry{label: "✏️  Enter a path…", action: "input"})

	ti := textinput.New()
	ti.Placeholder = "~/projects/api"
	ti.CharLimit = 1024
	ti.Width = 48
	return &remoteFolderModel{app: a, entries: entries, ti: ti}
}

// move advances the cursor by dir, wrapping around at either end.
func (m *remoteFolderModel) move(dir int) {
	if n := len(m.entries); n > 0 {
		m.cursor = (m.cursor + dir + n) % n
	}
}

func (m *remoteFolderModel) Init() tea.Cmd { return nil }

func (m *remoteFolderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m.app, nil
	}

	if m.inputting {
		switch km.String() {
		case "enter":
			path := strings.TrimSpace(m.ti.Value())
			if path == "" {
				path = "~"
			}
			return m.app, m.app.gotoRemoteProvider(path)
		case "esc":
			m.inputting = false
			m.ti.Blur()
			return m.app, nil
		}
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		return m.app, cmd
	}

	switch km.String() {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "enter":
		e := m.entries[m.cursor]
		if e.action == "input" {
			m.inputting = true
			m.ti.Focus()
			return m.app, textinput.Blink
		}
		return m.app, m.app.gotoRemoteProvider(e.path)
	case "esc":
		return m.app, m.app.gotoHost()
	}
	return m.app, nil
}

func (m *remoteFolderModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Remote folder — " + m.app.sshTarget))
	b.WriteString("\n\n")

	if m.inputting {
		b.WriteString("  Remote path:\n\n  ")
		b.WriteString(m.ti.View())
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("enter use path · esc cancel"))
		return b.String()
	}

	for i, e := range m.entries {
		marker := "  "
		label := e.label
		if i == m.cursor {
			marker = highlightStyle.Render("▸ ")
			label = highlightStyle.Render(label)
		}
		fmt.Fprintf(&b, "%s%s\n", marker, label)
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · enter select · esc back · ctrl+c quit"))
	return b.String()
}
