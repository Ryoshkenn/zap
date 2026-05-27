package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
)

type flagsModel struct {
	app    *app
	st     *detect.Status
	flags  []config.Flag
	on     []bool
	cursor int
}

func newFlagsModel(a *app, st *detect.Status) *flagsModel {
	m := &flagsModel{app: a, st: st, flags: st.Provider.Flags}
	m.on = make([]bool, len(m.flags))
	for i, f := range m.flags {
		m.on[i] = f.Default
		// If the flag is already in default_flags from config, pre-toggle on.
		for _, df := range st.Provider.DefaultFlags {
			if df == f.Flag {
				m.on[i] = true
			}
		}
	}
	return m
}

func (m *flagsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.flags)-1 {
				m.cursor++
			}
		case " ", "x":
			if len(m.flags) > 0 {
				m.on[m.cursor] = !m.on[m.cursor]
			}
		case "enter":
			extra := []string{}
			for i, f := range m.flags {
				if m.on[i] {
					extra = append(extra, f.Flag)
				}
			}
			// Don't double-apply DefaultFlags; app.launch merges.
			// Replace DefaultFlags semantics with explicit flags from this screen.
			// We pass the user-toggled set; app.launch will dedupe.
			st := *m.st
			st.Provider.DefaultFlags = nil // suppress duplicate apply
			return m.app, m.app.launch(&st, extra)
		case "esc":
			m.app.screen = screenProvider
			return m.app, nil
		}
	}
	return m.app, nil
}

func (m *flagsModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Flags for " + m.st.Provider.Name))
	b.WriteString("\n\n")
	if len(m.flags) == 0 {
		b.WriteString(mutedStyle.Render("  (no togglable flags — press enter to launch)") + "\n")
	}
	for i, f := range m.flags {
		check := "[ ]"
		if m.on[i] {
			check = highlightStyle.Render("[x]")
		}
		line := check + " " + f.Label + "  " + mutedStyle.Render(f.Flag)
		if i == m.cursor {
			line = highlightStyle.Render("▸ ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · space toggle · enter launch · esc back · q quit"))
	return b.String()
}
