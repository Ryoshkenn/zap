package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Ryoshkenn/zap/internal/config"
	"github.com/Ryoshkenn/zap/internal/detect"
)

type providerItem struct {
	st            detect.Status
	starred       bool
	modelSelector bool
	defaultModel  string
	remoteTarget  string // set for ssh launches: installation is unknown locally
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
	if i.remoteTarget != "" {
		bin, _ := config.SplitCommand(i.st.Provider.Command)
		return mutedStyle.Render("runs `" + bin + "` on " + i.remoteTarget)
	}
	if i.st.Installed {
		if i.modelSelector {
			if i.defaultModel != "" {
				return mutedStyle.Render("model: ") + highlightStyle.Render(i.defaultModel)
			}
			return mutedStyle.Render("no default model — press enter to choose")
		}
		if i.st.AppBundlePath != "" {
			return mutedStyle.Render(i.st.AppBundlePath)
		}
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
	w, _ := a.size()
	l := list.New(items, delegate, w, a.listHeight())
	title := "Pick a provider — " + abbrev(a.chosenFolder)
	if a.remote {
		title = "Pick a provider — " + a.sshTarget + ":" + a.chosenFolder
	}
	l.Title = title
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // zap renders its own help line below the list
	l.KeyMap.Quit.SetEnabled(false)
	return &providerModel{app: a, list: l}
}

func buildProviderItems(a *app) []list.Item {
	if a.remote {
		return buildRemoteProviderItems(a)
	}
	starred := []list.Item{}
	installed := []list.Item{}
	missing := []list.Item{}
	for _, st := range a.statuses {
		item := providerItem{st: st, starred: a.state.IsFavoriteProvider(st.Provider.ID)}
		if st.Provider.ModelSelector && st.Installed {
			item.modelSelector = true
			item.defaultModel, _ = a.state.PreferredModelFor(st.Provider.ID)
		}
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

// buildRemoteProviderItems lists providers suitable for an ssh launch. We can't
// detect installation on the remote host, so every provider is shown as
// available. GUI app-mode providers (Cursor, VS Code…) and model-selector
// providers (whose model defaults are local-only) are omitted — they don't
// make sense over ssh.
func buildRemoteProviderItems(a *app) []list.Item {
	starred := []list.Item{}
	rest := []list.Item{}
	for _, st := range a.statuses {
		if st.Provider.LaunchMode == "app" || st.Provider.ModelSelector {
			continue
		}
		st.Installed = true // assume present on the remote; we can't LookPath there
		item := providerItem{st: st, starred: a.state.IsFavoriteProvider(st.Provider.ID), remoteTarget: a.sshTarget}
		if item.starred {
			starred = append(starred, item)
		} else {
			rest = append(rest, item)
		}
	}
	return append(starred, rest...)
}

func (m *providerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// While typing a filter every key belongs to the filter input, except
		// enter, which picks the highlighted match straight away.
		if m.list.FilterState() == list.Filtering && msg.String() != "enter" {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m.app, cmd
		}
		switch msg.String() {
		case "up", "k":
			// Wrap to the bottom when pressing up at the top.
			if m.list.Index() == 0 {
				if n := len(m.list.VisibleItems()); n > 0 {
					m.list.Select(n - 1)
				}
				return m.app, nil
			}
		case "down", "j":
			// Wrap to the top when pressing down at the bottom.
			if m.list.Index() == len(m.list.VisibleItems())-1 {
				m.list.Select(0)
				return m.app, nil
			}
		case "enter":
			sel, ok := m.list.SelectedItem().(providerItem)
			if !ok {
				return m.app, nil
			}
			if !sel.st.Installed {
				return m.app, nil // ignore — not installed
			}
			return m.app, m.app.launchSelected(&sel.st)
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
				m.selectProvider(sel.st.Provider.ID)
			}
			return m.app, nil
		case "esc":
			if m.list.FilterState() == list.FilterApplied {
				break // let the list clear the filter
			}
			if m.app.remote {
				m.app.screen = screenRemoteFolder
			} else {
				m.app.screen = screenFolder
			}
			return m.app, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m.app, cmd
}

// selectProvider moves the cursor to the provider with id (after a re-sort).
func (m *providerModel) selectProvider(id string) {
	for i, it := range m.list.Items() {
		if pi, ok := it.(providerItem); ok && pi.st.Provider.ID == id {
			m.list.Select(i)
			return
		}
	}
}

func (m *providerModel) View() string {
	text := "↑/↓ move · enter select · / filter · f star · esc back · q quit"
	if m.list.FilterState() == list.Filtering {
		text = "type to filter · enter select · esc cancel"
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), helpStyle.Render(text))
}
