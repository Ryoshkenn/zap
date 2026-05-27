package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Ryoshkenn/zap/internal/state"
)

// folderItem implements list.Item.
type folderItem struct {
	label   string
	path    string // empty => action sentinel
	section string // "starred", "recent", "current", "browse"
}

func (i folderItem) Title() string       { return i.label }
func (i folderItem) Description() string { return i.path }
func (i folderItem) FilterValue() string { return i.label + " " + i.path }

type folderModel struct {
	app  *app
	list list.Model
}

func newFolderModel(a *app) *folderModel {
	items := buildFolderItems(a.state)

	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(0)
	delegate.ShowDescription = true

	l := list.New(items, delegate, 80, 22)
	l.Title = "Pick a folder"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return &folderModel{app: a, list: l}
}

func buildFolderItems(s *state.State) []list.Item {
	cwd, _ := os.Getwd()
	items := []list.Item{}

	if len(s.FavoriteFolders) > 0 {
		for _, f := range s.FavoriteFolders {
			items = append(items, folderItem{label: "⭐ " + abbrev(f), path: f, section: "starred"})
		}
	}
	for _, r := range s.RecentsSorted(4) {
		// Don't double-list starred folders.
		if s.IsFavoriteFolder(r.Path) {
			continue
		}
		items = append(items, folderItem{label: "🕘 " + abbrev(r.Path), path: r.Path, section: "recent"})
	}
	items = append(items, folderItem{label: "📁 Current: " + abbrev(cwd), path: cwd, section: "current"})
	items = append(items, folderItem{label: "➜  Browse folders…", path: "", section: "browse"})
	return items
}

func (m *folderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-2, msg.Height-4)
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			sel, ok := m.list.SelectedItem().(folderItem)
			if !ok {
				return m.app, nil
			}
			if sel.section == "browse" {
				return m.app, m.app.gotoBrowse()
			}
			return m.app, m.app.gotoProvider(sel.path)
		case "f":
			sel, ok := m.list.SelectedItem().(folderItem)
			if ok && sel.path != "" && sel.section != "browse" {
				if m.app.state.IsFavoriteFolder(sel.path) {
					m.app.state.RemoveFavoriteFolder(sel.path)
				} else {
					m.app.state.AddFavoriteFolder(sel.path)
				}
				_ = m.app.state.Save()
				m.list.SetItems(buildFolderItems(m.app.state))
			}
			return m.app, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m.app, cmd
}

func (m *folderModel) View() string {
	help := helpStyle.Render("↑/↓ move · enter select · / filter · f star/unstar · q quit")
	return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), help)
}

func abbrev(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// browse screen using bubbles/filepicker

type browseDoneMsg struct {
	path string
}

type browseModel struct {
	app     *app
	cwd     string
	entries []os.DirEntry
	cursor  int
	err     error
}

func newBrowseModel(a *app) *browseModel {
	cwd, _ := os.Getwd()
	bm := &browseModel{app: a, cwd: cwd}
	bm.refresh()
	return bm
}

func (m *browseModel) init() tea.Cmd { return nil }

func (m *browseModel) refresh() {
	entries, err := os.ReadDir(m.cwd)
	if err != nil {
		m.err = err
		return
	}
	dirs := entries[:0]
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, e)
		}
	}
	m.entries = dirs
	if m.cursor >= len(m.entries) {
		m.cursor = 0
	}
}

func (m *browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "left", "h", "backspace":
			parent := parentOf(m.cwd)
			if parent != m.cwd {
				m.cwd = parent
				m.cursor = 0
				m.refresh()
			}
		case "right", "l":
			if len(m.entries) > 0 {
				m.cwd = m.cwd + string(os.PathSeparator) + m.entries[m.cursor].Name()
				m.cursor = 0
				m.refresh()
			}
		case "enter":
			// Pick this directory (cwd).
			return m.app, m.app.gotoProvider(m.cwd)
		case "esc":
			m.app.screen = screenFolder
			return m.app, nil
		}
	}
	return m.app, nil
}

func (m *browseModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Browse — " + abbrev(m.cwd)))
	b.WriteString("\n\n")
	if m.err != nil {
		b.WriteString(errorStyle.Render(m.err.Error()) + "\n")
	}
	if len(m.entries) == 0 {
		b.WriteString(mutedStyle.Render("  (no subdirectories)") + "\n")
	}
	for i, e := range m.entries {
		marker := "  "
		name := e.Name()
		if i == m.cursor {
			marker = highlightStyle.Render("▸ ")
			name = highlightStyle.Render(name)
		}
		fmt.Fprintf(&b, "%s%s/\n", marker, name)
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · →/l enter dir · ←/h up · enter pick this dir · esc back"))
	return b.String()
}

func parentOf(p string) string {
	if p == "/" || p == "" {
		return p
	}
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == os.PathSeparator {
			if i == 0 {
				return "/"
			}
			return p[:i]
		}
	}
	return p
}
