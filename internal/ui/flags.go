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

	// Priority for pre-check: state.PreferredFlags > config DefaultFlags > flag.Default.
	saved, hasSaved := a.state.PreferredFlagsFor(st.Provider.ID)
	for i, f := range m.flags {
		switch {
		case hasSaved:
			m.on[i] = containsStr(saved, f.Flag)
		default:
			m.on[i] = f.Default
			for _, df := range st.Provider.DefaultFlags {
				if df == f.Flag {
					m.on[i] = true
				}
			}
		}
	}
	return m
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
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
			// Persist this choice as the provider's preferred flag set.
			m.app.state.SetPreferredFlags(m.st.Provider.ID, extra)
			_ = m.app.state.Save()
			// The toggled set is the source of truth; suppress DefaultFlags merge.
			st := *m.st
			st.Provider.DefaultFlags = nil
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
