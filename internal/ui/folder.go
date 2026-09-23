package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Ryoshkenn/zap/internal/state"
)

// folderItem implements list.Item.
type folderItem struct {
	icon    string
	label   string
	tag     string // dim suffix, e.g. "current"
	path    string // empty => action sentinel
	section string // "current", "starred", "recent", "browse", "ssh", "settings"
}

func (i folderItem) FilterValue() string { return i.label + " " + i.path }

// separatorItem renders as a horizontal rule between sections.
type separatorItem struct{}

func (separatorItem) FilterValue() string { return "" }

// folderDelegate renders one compact row per item, so favorites, recents and
// the actions all fit on one page instead of paginating the actions away.
type folderDelegate struct{}

func (folderDelegate) Height() int                         { return 1 }
func (folderDelegate) Spacing() int                        { return 0 }
func (folderDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (folderDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	width := m.Width()
	if _, ok := item.(separatorItem); ok {
		n := width - 4
		if n > 60 {
			n = 60
		}
		if n < 4 {
			n = 4
		}
		fmt.Fprint(w, "  "+mutedStyle.Render(strings.Repeat("─", n)))
		return
	}
	it, ok := item.(folderItem)
	if !ok {
		return
	}
	marker, label := "  ", it.label
	if index == m.Index() {
		marker = highlightStyle.Render("▸ ")
		label = highlightStyle.Render(label)
	}
	row := marker + it.icon + " " + label
	if it.tag != "" {
		row += "  " + hintStyle.Render(it.tag)
	}
	fmt.Fprint(w, truncate(row, width))
}

type folderModel struct {
	app  *app
	list list.Model
}

func newFolderModel(a *app) *folderModel {
	w, _ := a.size()
	l := list.New(buildFolderItems(a.state), folderDelegate{}, w, a.listHeight())
	l.Title = "Pick a folder"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // zap renders its own help line below the list
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.KeyMap.Quit.SetEnabled(false) // esc/q handling lives in app.Update
	return &folderModel{app: a, list: l}
}

// maxFolderRecents caps how many recent folders the picker offers.
const maxFolderRecents = 4

// buildFolderItems lists the current directory first, then favorites and
// recents (each folder appears once), then the actions.
func buildFolderItems(s *state.State) []list.Item {
	cwd, _ := os.Getwd()
	shown := map[string]bool{}

	items := []list.Item{}
	if cwd != "" {
		tag := "current"
		if s.IsFavoriteFolder(cwd) {
			tag = "current · ⭐"
		}
		items = append(items, folderItem{icon: "📁", label: abbrev(cwd), tag: tag, path: cwd, section: "current"})
		shown[cwd] = true
	}

	var saved []list.Item
	for _, f := range s.FavoriteFolders {
		if !shown[f] {
			shown[f] = true
			saved = append(saved, folderItem{icon: "⭐", label: abbrev(f), path: f, section: "starred"})
		}
	}
	recents := 0
	for _, r := range s.RecentsSorted(0) {
		if recents == maxFolderRecents {
			break
		}
		if !shown[r.Path] {
			shown[r.Path] = true
			recents++
			saved = append(saved, folderItem{icon: "🕘", label: abbrev(r.Path), path: r.Path, section: "recent"})
		}
	}
	if len(saved) > 0 {
		items = append(items, separatorItem{})
		items = append(items, saved...)
	}

	items = append(items,
		separatorItem{},
		folderItem{icon: "🔎", label: "Browse folders…", section: "browse"},
		folderItem{icon: "🌐", label: "SSH / Remote…", section: "ssh"},
		folderItem{icon: "🔧", label: "Settings…", section: "settings"},
	)
	return items
}

// skipSep moves the selection by dir to the next non-separator row, wrapping
// around at either end so up from the top lands on the bottom and vice versa.
func (m *folderModel) skipSep(from, dir int) {
	items := m.list.Items()
	n := len(items)
	if n == 0 {
		return
	}
	idx := from
	for i := 0; i < n; i++ {
		idx = (idx + dir + n) % n
		if _, isSep := items[idx].(separatorItem); !isSep {
			m.list.Select(idx)
			return
		}
	}
}

// rebuild refreshes the items after a favorite toggle, keeping the cursor on
// the same folder.
func (m *folderModel) rebuild(keep folderItem) {
	items := buildFolderItems(m.app.state)
	m.list.SetItems(items)
	for i, it := range items {
		if fi, ok := it.(folderItem); ok && fi.path == keep.path && fi.section == keep.section {
			m.list.Select(i)
			return
		}
	}
	for i, it := range items {
		if fi, ok := it.(folderItem); ok && fi.path == keep.path {
			m.list.Select(i)
			return
		}
	}
}

func (m *folderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		// While typing a filter every key belongs to the filter input, except
		// enter, which picks the highlighted match straight away.
		if m.list.FilterState() == list.Filtering && km.String() != "enter" {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m.app, cmd
		}
		switch km.String() {
		case "up", "k":
			m.skipSep(m.list.Index(), -1)
			return m.app, nil
		case "down", "j":
			m.skipSep(m.list.Index(), 1)
			return m.app, nil
		case "enter":
			sel, ok := m.list.SelectedItem().(folderItem)
			if !ok {
				return m.app, nil
			}
			switch sel.section {
			case "browse":
				return m.app, m.app.gotoBrowse()
			case "ssh":
				return m.app, m.app.gotoHost()
			case "settings":
				return m.app, m.app.gotoSettings()
			}
			return m.app, m.app.gotoProvider(sel.path)
		case "i":
			return m.app, m.app.gotoSettings()
		case "esc":
			// esc clears an applied filter first; otherwise it quits like q.
			if m.list.FilterState() == list.Unfiltered {
				return m.app, tea.Quit
			}
		case "f":
			sel, ok := m.list.SelectedItem().(folderItem)
			if ok && sel.path != "" {
				if m.app.state.IsFavoriteFolder(sel.path) {
					m.app.state.RemoveFavoriteFolder(sel.path)
				} else {
					m.app.state.AddFavoriteFolder(sel.path)
				}
				_ = m.app.state.Save()
				m.rebuild(sel)
			}
			return m.app, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m.app, cmd
}

func (m *folderModel) View() string {
	text := "↑/↓ move · enter select · / filter · f star/unstar · i settings · q quit"
	if m.list.FilterState() == list.Filtering {
		text = "type to filter · enter select · esc cancel"
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), helpStyle.Render(text))
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
	entries []string // subdirectory names
	cursor  int
	offset  int
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
	m.entries = nil
	m.offset = 0
	entries, err := os.ReadDir(m.cwd)
	m.err = err
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			// Follow symlinks so linked project folders are browsable too.
			if info, err := os.Stat(filepath.Join(m.cwd, e.Name())); err == nil {
				isDir = info.IsDir()
			}
		}
		if isDir {
			m.entries = append(m.entries, e.Name())
		}
	}
	if m.cursor >= len(m.entries) {
		m.cursor = 0
	}
}

func (m *browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if n := len(m.entries); n > 0 {
				m.cursor = (m.cursor - 1 + n) % n
			}
		case "down", "j":
			if n := len(m.entries); n > 0 {
				m.cursor = (m.cursor + 1) % n
			}
		case "left", "h", "backspace":
			parent := parentOf(m.cwd)
			if parent != m.cwd {
				from := filepath.Base(m.cwd)
				m.cwd = parent
				m.cursor = 0
				m.refresh()
				// Land on the folder we just came out of.
				for i, name := range m.entries {
					if name == from {
						m.cursor = i
						break
					}
				}
			}
		case "enter", "right", "l":
			// Descend into highlighted subdirectory.
			if len(m.entries) > 0 {
				m.cwd = childOf(m.cwd, m.entries[m.cursor])
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
	w, _ := m.app.size()
	b.WriteString(truncate(titleStyle.Render("Browse — "+abbrev(m.cwd)+star), w))
	b.WriteString("\n\n")
	if m.err != nil {
		b.WriteString(truncate("  "+errorStyle.Render(m.err.Error()), w) + "\n")
	} else if len(m.entries) == 0 {
		b.WriteString(mutedStyle.Render("  (no subdirectories)") + "\n")
	}
	// Overhead: title, blank, scroll hint, help (2), error line.
	start, end := scrollWindow(m.offset, m.cursor, len(m.entries), m.app.bodyHeight(6))
	m.offset = start
	for i := start; i < end; i++ {
		marker, name := "  ", m.entries[i]+"/"
		if i == m.cursor {
			marker = highlightStyle.Render("▸ ")
			name = highlightStyle.Render(name)
		}
		b.WriteString(truncate(marker+name, w) + "\n")
	}
	if hint := scrollHint(start, end, len(m.entries)); hint != "" {
		b.WriteString("  " + hintStyle.Render(hint) + "\n")
	}
	b.WriteString(helpStyle.Render("↑/↓ move · enter/→ descend · ←/h up · space pick this dir · f favorite · esc back"))
	return b.String()
}

func parentOf(p string) string {
	if p == "" {
		return p
	}
	clean := filepath.Clean(p)
	parent := filepath.Dir(clean)
	if parent == "." {
		return clean
	}
	return parent
}

func childOf(parent, child string) string {
	return filepath.Join(parent, child)
}
