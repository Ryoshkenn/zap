package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
)

// settingsModel lets the user view and toggle per-provider default flags.
// Toggling here writes to state.PreferredFlags so future launches inherit them.
type settingsModel struct {
	app    *app
	rows   []settingsRow
	cursor int
	dirty  bool
}

// settingsRow is one togglable flag (or a provider header row, launch mode toggle, or model selector).
type settingsRow struct {
	isHeader      bool
	isLaunchMode  bool
	isModelSelect bool
	providerID    string
	flag          config.Flag
	on            bool
	label         string
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

		// Launch mode toggle — always shown so users can override the default.
		defaultMode := p.LaunchMode
		if defaultMode == "" {
			defaultMode = "terminal"
		}
		modeOn := defaultMode == "app"
		if saved, ok := m.app.state.LaunchModeFor(p.ID); ok {
			modeOn = saved == "app"
		}
		m.rows = append(m.rows, settingsRow{
			isLaunchMode: true,
			providerID:   p.ID,
			on:           modeOn,
			label:        "Open as app  " + mutedStyle.Render("(default: "+defaultMode+")"),
		})

		if p.ModelSelector {
			modelLabel := mutedStyle.Render("(none selected)")
			if model, ok := m.app.state.PreferredModelFor(p.ID); ok {
				modelLabel = highlightStyle.Render(model)
			}
			m.rows = append(m.rows, settingsRow{
				isModelSelect: true,
				providerID:    p.ID,
				label:         "Default model  " + modelLabel,
			})
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

func (m *settingsModel) isInteractive(r settingsRow) bool {
	return !r.isHeader
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
			if r.isModelSelect {
				st := m.app.cfg.FindProvider(r.providerID)
				if st == nil {
					return m.app, nil
				}
				fakeStatus := &detect.Status{Provider: *st}
				return m.app, m.app.gotoModelPicker(fakeStatus, screenSettings)
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
	if r.isLaunchMode {
		mode := "terminal"
		if r.on {
			mode = "app"
		}
		m.app.state.SetLaunchMode(r.providerID, mode)
		_ = m.app.state.Save()
		return
	}
	// Collect current state of all flag rows for this provider.
	current := []string{}
	for _, row := range m.rows {
		if row.providerID == r.providerID && !row.isLaunchMode && row.on {
			current = append(current, row.flag.Flag)
		}
	}
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
		marker := "    "
		if i == m.cursor {
			marker = "  " + highlightStyle.Render("▸ ")
		}
		if r.isModelSelect {
			b.WriteString(marker + highlightStyle.Render("[→]") + " " + r.label + "\n")
			continue
		}
		check := "[ ]"
		if r.on {
			check = highlightStyle.Render("[x]")
		}
		b.WriteString(marker + check + " " + r.label + "\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · space/enter toggle · esc back · q quit"))
	return b.String()
}
