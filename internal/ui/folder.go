package ui

import (
	"fmt"
	"io"
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

// separatorItem renders as a horizontal rule between sections.
type separatorItem struct{}

func (separatorItem) Title() string       { return "" }
func (separatorItem) Description() string { return "" }
func (separatorItem) FilterValue() string { return "" }

// folderDelegate wraps DefaultDelegate and renders separatorItems as horizontal rules.
type folderDelegate struct {
	base list.DefaultDelegate
}

func (d folderDelegate) Height() int  { return d.base.Height() }
func (d folderDelegate) Spacing() int { return d.base.Spacing() }
func (d folderDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return d.base.Update(msg, m)
}
func (d folderDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if _, ok := item.(separatorItem); ok {
		width := m.Width()
		if width <= 6 {
			width = 40
		}
		line := strings.Repeat("─", width-6)
		fmt.Fprintf(w, "  %s\n", mutedStyle.Render(line))
		return
	}
	d.base.Render(w, m, index, item)
}

type folderModel struct {
	app  *app
	list list.Model
}

func newFolderModel(a *app) *folderModel {
	items := buildFolderItems(a.state)

	base := list.NewDefaultDelegate()
	base.SetSpacing(0)
	base.ShowDescription = true

	l := list.New(items, folderDelegate{base: base}, 80, 22)
	l.Title = "Pick a folder"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return &folderModel{app: a, list: l}
}

func buildFolderItems(s *state.State) []list.Item {
	cwd, _ := os.Getwd()
	items := []list.Item{}

	hasFavorites := len(s.FavoriteFolders) > 0
	if hasFavorites {
		for _, f := range s.FavoriteFolders {
			items = append(items, folderItem{label: "⭐ " + abbrev(f), path: f, section: "starred"})
		}
	}

	var recentItems []list.Item
	for _, r := range s.RecentsSorted(4) {
		// Don't double-list starred folders.
		if s.IsFavoriteFolder(r.Path) {
			continue
		}
		recentItems = append(recentItems, folderItem{label: "🕘 " + abbrev(r.Path), path: r.Path, section: "recent"})
	}

	if len(recentItems) > 0 {
		if hasFavorites {
			items = append(items, separatorItem{})
		}
		items = append(items, recentItems...)
		items = append(items, separatorItem{})
	} else if hasFavorites {
		items = append(items, separatorItem{})
	}

	items = append(items, folderItem{label: "📁 Current: " + abbrev(cwd), path: cwd, section: "current"})
	items = append(items, folderItem{label: "➜  Browse folders…", path: "", section: "browse"})
	items = append(items, folderItem{label: "⚙  Settings…", path: "", section: "settings"})
	return items
}

func (m *folderModel) skipSep(from, dir int) {
	items := m.list.Items()
	idx := from
	for {
		next := idx + dir
		if next < 0 || next >= len(items) {
			break
		}
		idx = next
		if _, isSep := items[idx].(separatorItem); !isSep {
			m.list.Select(idx)
			break
		}
	}
}

func (m *folderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-2, msg.Height-4)
	case tea.KeyMsg:
		// Intercept up/down when not filtering so the cursor skips separator rows.
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "up", "k":
				m.skipSep(m.list.Index(), -1)
				return m.app, nil
			case "down", "j":
				m.skipSep(m.list.Index(), 1)
				return m.app, nil
			}
		}
		switch msg.String() {
		case "enter":
			sel, ok := m.list.SelectedItem().(folderItem)
			if !ok {
				return m.app, nil
			}
			switch sel.section {
			case "browse":
				return m.app, m.app.gotoBrowse()
			case "settings":
				return m.app, m.app.gotoSettings()
			}
			return m.app, m.app.gotoProvider(sel.path)
		case "i":
			return m.app, m.app.gotoSettings()
		case "f":
			sel, ok := m.list.SelectedItem().(folderItem)
			if ok && sel.path != "" && sel.section != "browse" && sel.section != "settings" {
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
	help := helpStyle.Render("↑/↓ move · enter select · / filter · f star/unstar · i settings · q quit")
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
		case "enter", "right", "l":
			// Descend into highlighted subdirectory.
			if len(m.entries) > 0 {
				m.cwd = m.cwd + string(os.PathSeparator) + m.entries[m.cursor].Name()
				m.cursor = 0
				m.refresh()
			}
		case " ":
			// Pick the directory we're currently viewing.
			return m.app, m.app.gotoProvider(m.cwd)
		case "f":
			// Toggle favorite on the directory we're currently viewing.
			if m.app.state.IsFavoriteFolder(m.cwd) {
				m.app.state.RemoveFavoriteFolder(m.cwd)
			} else {
				m.app.state.AddFavoriteFolder(m.cwd)
			}
			_ = m.app.state.Save()
		case "esc":
			m.app.screen = screenFolder
			m.app.folder = newFolderModel(m.app)
			return m.app, nil
		}
	}
	return m.app, nil
}

func (m *browseModel) View() string {
	var b strings.Builder
	star := ""
	if m.app.state.IsFavoriteFolder(m.cwd) {
		star = starStyle.Render(" ⭐")
	}
	b.WriteString(titleStyle.Render("Browse — " + abbrev(m.cwd) + star))
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
	b.WriteString(helpStyle.Render("↑/↓ move · enter/→ descend · ←/h up · space pick this dir · f favorite · esc back"))
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
