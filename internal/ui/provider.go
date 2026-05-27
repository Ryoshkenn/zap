package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Ryoshkenn/zap/internal/detect"
)

type providerItem struct {
	st       detect.Status
	starred  bool
}

func (i providerItem) Title() string {
	star := "  "
	if i.starred {
		star = "⭐"
	}
	icon := i.st.Provider.Icon
	if icon == "" {
		icon = "  "
	}
	name := i.st.Provider.Name
	if !i.st.Installed {
		name = mutedStyle.Render(name + "  (not installed)")
	}
	return star + " " + icon + " " + name
}

func (i providerItem) Description() string {
	if i.st.Installed {
		return mutedStyle.Render(i.st.Path)
	}
	if i.st.Provider.InstallHint != "" {
		return mutedStyle.Render("install: " + i.st.Provider.InstallHint)
	}
	return ""
}

func (i providerItem) FilterValue() string { return i.st.Provider.ID + " " + i.st.Provider.Name }

type providerModel struct {
	app  *app
	list list.Model
}

func newProviderModel(a *app) *providerModel {
	items := buildProviderItems(a)
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	l := list.New(items, delegate, 80, 22)
	l.Title = "Pick a provider — " + abbrev(a.chosenFolder)
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	return &providerModel{app: a, list: l}
}

func buildProviderItems(a *app) []list.Item {
	starred := []list.Item{}
	installed := []list.Item{}
	missing := []list.Item{}
	for _, st := range a.statuses {
		item := providerItem{st: st, starred: a.state.IsFavoriteProvider(st.Provider.ID)}
		switch {
		case item.starred:
			starred = append(starred, item)
		case st.Installed:
			installed = append(installed, item)
		default:
			missing = append(missing, item)
		}
	}
	out := append(starred, installed...)
	return append(out, missing...)
}

func (m *providerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-2, msg.Height-4)
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			sel, ok := m.list.SelectedItem().(providerItem)
			if !ok {
				return m.app, nil
			}
			if !sel.st.Installed {
				return m.app, nil // ignore — not installed
			}
			return m.app, m.app.gotoFlags(&sel.st)
		case "f":
			sel, ok := m.list.SelectedItem().(providerItem)
			if ok {
				if m.app.state.IsFavoriteProvider(sel.st.Provider.ID) {
					m.app.state.RemoveFavoriteProvider(sel.st.Provider.ID)
				} else {
					m.app.state.AddFavoriteProvider(sel.st.Provider.ID)
				}
				_ = m.app.state.Save()
				m.list.SetItems(buildProviderItems(m.app))
			}
			return m.app, nil
		case "esc":
			m.app.screen = screenFolder
			return m.app, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m.app, cmd
}

func (m *providerModel) View() string {
	help := helpStyle.Render("↑/↓ move · enter select · / filter · f star · esc back · q quit")
	return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), help)
}
