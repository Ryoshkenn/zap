package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/config"
)

// settingsModel lets the user view and toggle per-provider default flags.
// Toggling here writes to state.PreferredFlags so future launches inherit them.
type settingsModel struct {
	app    *app
	rows   []settingsRow
	cursor int
	dirty  bool
}

// settingsRow is one togglable flag (or a provider header row).
type settingsRow struct {
	isHeader   bool
	providerID string
	flag       config.Flag
	on         bool
	label      string
}

func newSettingsModel(a *app) *settingsModel {
	m := &settingsModel{app: a}
	m.rebuild()
	return m
}

func (m *settingsModel) rebuild() {
	m.rows = m.rows[:0]
	for _, p := range m.app.cfg.Providers {
		m.rows = append(m.rows, settingsRow{
			isHeader: true,
			label:    p.Icon + " " + p.Name + "  " + mutedStyle.Render("("+p.ID+")"),
		})
		if len(p.Flags) == 0 {
			m.rows = append(m.rows, settingsRow{
				isHeader: true,
				label:    mutedStyle.Render("  (no togglable flags)"),
			})
			continue
		}
		saved, hasSaved := m.app.state.PreferredFlagsFor(p.ID)
		for _, f := range p.Flags {
			on := f.Default
			for _, df := range p.DefaultFlags {
				if df == f.Flag {
					on = true
				}
			}
			if hasSaved {
				on = containsStr(saved, f.Flag)
			}
			m.rows = append(m.rows, settingsRow{
				providerID: p.ID,
				flag:       f,
				on:         on,
				label:      f.Label + "  " + mutedStyle.Render(f.Flag),
			})
		}
	}
	// Move cursor onto first non-header row if currently on a header.
	if len(m.rows) > 0 && m.rows[m.cursor].isHeader {
		m.advance(1)
	}
}

func (m *settingsModel) advance(dir int) {
	if len(m.rows) == 0 {
		return
	}
	for i := 0; i < len(m.rows); i++ {
		m.cursor = (m.cursor + dir + len(m.rows)) % len(m.rows)
		if !m.rows[m.cursor].isHeader {
			return
		}
	}
}

func (m *settingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			m.advance(-1)
		case "down", "j":
			m.advance(1)
		case " ", "x", "enter":
			r := &m.rows[m.cursor]
			if r.isHeader {
				return m.app, nil
			}
			r.on = !r.on
			m.persistRow(r)
			m.dirty = true
		case "esc", "q":
			m.app.screen = screenFolder
			m.app.folder = newFolderModel(m.app)
			return m.app, nil
		}
	}
	return m.app, nil
}

func (m *settingsModel) persistRow(r *settingsRow) {
	saved, _ := m.app.state.PreferredFlagsFor(r.providerID)
	// Collect current state of all rows for this provider so we save the
	// complete picture (so default_flags from config can be overridden).
	current := []string{}
	for _, row := range m.rows {
		if row.providerID == r.providerID && row.on {
			current = append(current, row.flag.Flag)
		}
	}
	_ = saved
	m.app.state.SetPreferredFlags(r.providerID, current)
	_ = m.app.state.Save()
}

func (m *settingsModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Settings — default flags per provider"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("  These persist across launches. Override per-run with `zap <provider> --safe`."))
	b.WriteString("\n\n")
	for i, r := range m.rows {
		if r.isHeader {
			b.WriteString("  " + r.label + "\n")
			continue
		}
		check := "[ ]"
		if r.on {
			check = highlightStyle.Render("[x]")
		}
		marker := "    "
		if i == m.cursor {
			marker = "  " + highlightStyle.Render("▸ ")
		}
		b.WriteString(marker + check + " " + r.label + "\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · space/enter toggle · esc back · q quit"))
	return b.String()
}
