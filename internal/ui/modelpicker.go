package ui

import (
	"fmt"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Ryoshkenn/zap/internal/ollama"
)

// pickerState tracks which phase the model picker is in.
type pickerState int

const (
	pickerStarting     pickerState = iota // ensuring ollama is running
	pickerLoading                         // fetching model list
	pickerWarning                         // loaded-model RAM warning
	pickerStopping                        // stopping loaded models
	pickerReady                           // showing model list + search
	pickerPullConfirm                     // confirm download of a pull model
	pickerError                           // unrecoverable error
)

// modelItem is one row in the model list (section header or selectable model).
type modelItem struct {
	name      string
	isSection bool
	section   string
	kind      itemKind
}

type itemKind int

const (
	kindDownloaded itemKind = iota
	kindCloud
	kindPull
)

// modelPickerModel is the TUI screen for selecting an Ollama model.
type modelPickerModel struct {
	app           *app
	providerID    string
	pstate        pickerState
	allItems      []modelItem // full unfiltered list
	visible       []modelItem // filtered by search (rebuilt on every keystroke)
	cursor        int         // index into visible
	scrollOffset  int         // first visible item index in visible slice
	searchQuery   string
	pendingModel  string // model awaiting pull confirmation
	runningModels []ollama.RunningModel
	warnCursor    int // 0=stop&continue, 1=continue anyway, 2=go back
	pullCursor    int // 0=download&run, 1=cancel
	err           error
	onSelect      func(string) tea.Cmd
	returnScreen  screen
	settingsMode  bool // true = only show downloaded+cloud, no pull section
}

// --- messages ---

type ollamaEnsureMsg struct{ err error }

type ollamaFetchMsg struct {
	downloaded []string
	running    []ollama.RunningModel
	err        error
}

type ollamaStopMsg struct{ err error }

// --- constructor ---

func newModelPickerModel(a *app, providerID string, onSelect func(string) tea.Cmd, returnTo screen, settingsMode bool) *modelPickerModel {
	return &modelPickerModel{
		app:          a,
		providerID:   providerID,
		pstate:       pickerStarting,
		warnCursor:   0,
		onSelect:     onSelect,
		returnScreen: returnTo,
		settingsMode: settingsMode,
	}
}

// --- cmds ---

func ensureOllamaRunning() tea.Cmd {
	return func() tea.Msg { return ollamaEnsureMsg{err: ollama.EnsureRunning()} }
}

func fetchAllModels() tea.Cmd {
	return func() tea.Msg {
		downloaded, err := ollama.FetchModels()
		if err != nil {
			return ollamaFetchMsg{err: err}
		}
		running, _ := ollama.FetchRunningModels() // non-fatal
		return ollamaFetchMsg{downloaded: downloaded, running: running}
	}
}

func stopAllModels(models []ollama.RunningModel) tea.Cmd {
	return func() tea.Msg {
		for _, m := range models {
			if err := ollama.StopModel(m.Name); err != nil {
				return ollamaStopMsg{err: err}
			}
		}
		return ollamaStopMsg{}
	}
}

// --- lifecycle ---

func (m *modelPickerModel) Init() tea.Cmd { return ensureOllamaRunning() }

// buildAllItems builds the full unfiltered list from downloaded + catalogue.
func (m *modelPickerModel) buildAllItems(downloaded []string) {
	m.allItems = nil

	// Section 1: Downloaded
	if len(downloaded) > 0 {
		m.allItems = append(m.allItems, modelItem{isSection: true, section: "Downloaded"})
		for _, name := range downloaded {
			m.allItems = append(m.allItems, modelItem{name: name, kind: kindDownloaded})
		}
	}

	// Build a set of downloaded base names (strip :tag) to de-dup the catalogue.
	dlSet := make(map[string]bool, len(downloaded))
	for _, d := range downloaded {
		dlSet[d] = true
		if idx := strings.Index(d, ":"); idx >= 0 {
			dlSet[d[:idx]] = true
		}
	}

	// Section 2: Cloud
	var cloudItems []modelItem
	for _, km := range ollama.KnownModels() {
		if km.Kind == ollama.KindCloud && !dlSet[km.Name] {
			cloudItems = append(cloudItems, modelItem{name: km.Name, kind: kindCloud})
		}
	}
	if len(cloudItems) > 0 {
		m.allItems = append(m.allItems, modelItem{isSection: true, section: "Cloud"})
		m.allItems = append(m.allItems, cloudItems...)
	}

	// Section 3: Available to pull — omitted in settings mode.
	if !m.settingsMode {
		var pullItems []modelItem
		for _, km := range ollama.KnownModels() {
			if km.Kind == ollama.KindPull && !dlSet[km.Name] {
				pullItems = append(pullItems, modelItem{name: km.Name, kind: kindPull})
			}
		}
		if len(pullItems) > 0 {
			m.allItems = append(m.allItems, modelItem{isSection: true, section: "Available to pull"})
			m.allItems = append(m.allItems, pullItems...)
		}
	}

	m.rebuildVisible()
	m.resetCursorToSaved()
}

// rebuildVisible filters allItems by searchQuery and populates visible.
func (m *modelPickerModel) rebuildVisible() {
	m.scrollOffset = 0
	if m.searchQuery == "" {
		m.visible = m.allItems
		return
	}
	q := strings.ToLower(m.searchQuery)
	result := make([]modelItem, 0, len(m.allItems))
	var pendingSection *modelItem
	for i := range m.allItems {
		item := m.allItems[i]
		if item.isSection {
			pendingSection = &m.allItems[i]
			continue
		}
		if strings.Contains(strings.ToLower(item.name), q) {
			if pendingSection != nil {
				result = append(result, *pendingSection)
				pendingSection = nil
			}
			result = append(result, item)
		}
	}
	m.visible = result

	// Clamp cursor onto a valid non-section row.
	if m.cursor >= len(m.visible) || (m.cursor < len(m.visible) && m.visible[m.cursor].isSection) {
		m.moveCursorToFirst()
	}
}

func (m *modelPickerModel) resetCursorToSaved() {
	if saved, ok := m.app.state.PreferredModelFor(m.providerID); ok {
		for i, item := range m.visible {
			if !item.isSection && item.name == saved {
				m.cursor = i
				return
			}
		}
	}
	m.moveCursorToFirst()
}

func (m *modelPickerModel) moveCursorToFirst() {
	for i, item := range m.visible {
		if !item.isSection {
			m.cursor = i
			return
		}
	}
	m.cursor = 0
}

func (m *modelPickerModel) advanceCursor(dir int) {
	if len(m.visible) == 0 {
		return
	}
	for range m.visible {
		m.cursor = (m.cursor + dir + len(m.visible)) % len(m.visible)
		if !m.visible[m.cursor].isSection {
			return
		}
	}
}

// listViewportHeight returns how many item rows fit on screen for the list.
func (m *modelPickerModel) listViewportHeight() int {
	// Fixed overhead: title(1) + blank(1) + search(1) + blank(1) + help(1) = 5
	h := m.app.height - 5
	if h < 4 {
		h = 4
	}
	return h
}

// clampScroll adjusts scrollOffset so cursor stays within the viewport.
func (m *modelPickerModel) clampScroll() {
	vp := m.listViewportHeight()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+vp {
		m.scrollOffset = m.cursor - vp + 1
	}
	maxOffset := len(m.visible) - vp
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// --- Update ---

func (m *modelPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ollamaEnsureMsg:
		if msg.err != nil {
			m.pstate = pickerError
			m.err = msg.err
			return m.app, nil
		}
		m.pstate = pickerLoading
		return m.app, fetchAllModels()

	case ollamaFetchMsg:
		if msg.err != nil {
			m.pstate = pickerError
			m.err = msg.err
			return m.app, nil
		}
		m.runningModels = msg.running
		m.buildAllItems(msg.downloaded)
		// Only warn about RAM if a running model is NOT the one the user is
		// most likely to select (i.e. their saved preferred model). If every
		// running model is already the preferred model, loading it again costs
		// no new RAM, so skip straight to ready.
		preferred, hasPref := m.app.state.PreferredModelFor(m.providerID)
		shouldWarn := false
		if len(msg.running) > 0 {
			if !hasPref {
				shouldWarn = true
			} else {
				for _, rm := range msg.running {
					if rm.Name != preferred {
						shouldWarn = true
						break
					}
				}
			}
		}
		if shouldWarn {
			m.pstate = pickerWarning
		} else {
			m.pstate = pickerReady
		}
		return m.app, nil

	case ollamaStopMsg:
		m.runningModels = nil
		m.pstate = pickerReady
		return m.app, nil

	case tea.KeyMsg:
		// Esc always cancels (except while a stop is in progress).
		if msg.String() == "esc" && m.pstate != pickerStopping {
			if m.pstate == pickerReady && m.searchQuery != "" {
				m.searchQuery = ""
				m.rebuildVisible()
				m.resetCursorToSaved()
				return m.app, nil
			}
			m.app.screen = m.returnScreen
			return m.app, nil
		}

		switch m.pstate {
		case pickerWarning:
			return m.updateWarning(msg)
		case pickerReady:
			return m.updateReady(msg)
		case pickerPullConfirm:
			return m.updatePullConfirm(msg)
		case pickerError:
			if msg.String() == "r" {
				m.pstate = pickerStarting
				m.err = nil
				return m.app, ensureOllamaRunning()
			}
		}
	}
	return m.app, nil
}

func (m *modelPickerModel) updateWarning(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.warnCursor > 0 {
			m.warnCursor--
		}
	case "down", "j":
		if m.warnCursor < 2 {
			m.warnCursor++
		}
	case "enter", " ":
		switch m.warnCursor {
		case 0:
			m.pstate = pickerStopping
			return m.app, stopAllModels(m.runningModels)
		case 1:
			m.pstate = pickerReady
		case 2:
			m.app.screen = m.returnScreen
		}
	}
	return m.app, nil
}

func (m *modelPickerModel) updateReady(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.advanceCursor(-1)
	case "down", "j":
		m.advanceCursor(1)
	case "enter":
		if len(m.visible) == 0 || m.visible[m.cursor].isSection {
			return m.app, nil
		}
		item := m.visible[m.cursor]
		if item.kind == kindPull {
			m.pendingModel = item.name
			m.pullCursor = 0
			m.pstate = pickerPullConfirm
			return m.app, nil
		}
		return m.app, m.onSelect(item.name)
	case "backspace":
		if len(m.searchQuery) > 0 {
			r := []rune(m.searchQuery)
			m.searchQuery = string(r[:len(r)-1])
			m.rebuildVisible()
		}
	default:
		// Any printable single rune goes into the search query.
		runes := []rune(msg.String())
		if len(runes) == 1 && unicode.IsPrint(runes[0]) {
			m.searchQuery += string(runes[0])
			m.rebuildVisible()
		}
	}
	return m.app, nil
}

func (m *modelPickerModel) updatePullConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.pullCursor > 0 {
			m.pullCursor--
		}
	case "down", "j":
		if m.pullCursor < 1 {
			m.pullCursor++
		}
	case "enter", " ":
		if m.pullCursor == 0 {
			model := m.pendingModel
			m.pendingModel = ""
			return m.app, m.onSelect(model)
		}
		// Cancel — back to list.
		m.pendingModel = ""
		m.pstate = pickerReady
	}
	return m.app, nil
}

// --- View ---

func (m *modelPickerModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select Ollama model"))
	b.WriteString("\n\n")

	switch m.pstate {
	case pickerStarting:
		b.WriteString("  " + hintStyle.Render("Starting Ollama…") + "\n")
		b.WriteString(helpStyle.Render("esc back · ctrl+c quit"))
	case pickerLoading:
		b.WriteString("  " + hintStyle.Render("Fetching available models…") + "\n")
		b.WriteString(helpStyle.Render("esc back · ctrl+c quit"))
	case pickerWarning:
		m.renderWarning(&b)
	case pickerStopping:
		b.WriteString("  " + hintStyle.Render("Stopping loaded model(s)…") + "\n")
		b.WriteString(helpStyle.Render("ctrl+c quit"))
	case pickerReady:
		m.renderSearch(&b)
		m.renderList(&b)
	case pickerPullConfirm:
		m.renderPullConfirm(&b)
	case pickerError:
		b.WriteString("  " + errorStyle.Render("✗  "+m.err.Error()) + "\n\n")
		b.WriteString("  " + hintStyle.Render("Make sure Ollama is installed: https://ollama.com") + "\n")
		b.WriteString(helpStyle.Render("r retry · esc back · ctrl+c quit"))
	}
	return b.String()
}

func (m *modelPickerModel) renderSearch(b *strings.Builder) {
	cursor := "█"
	b.WriteString("  " + hintStyle.Render("Search:") + " " + m.searchQuery + cursor + "\n\n")
}

func (m *modelPickerModel) renderWarning(b *strings.Builder) {
	b.WriteString("  " + errorStyle.Render("⚠  Model already loaded in memory") + "\n\n")
	for _, rm := range m.runningModels {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			highlightStyle.Render(rm.Name),
			hintStyle.Render(fmt.Sprintf("(%.1f GB in RAM)", rm.SizeGB)),
		))
	}
	b.WriteString("\n  Loading another model may strain your system.\n\n")

	opts := []string{"Stop loaded model(s) and continue", "Continue anyway", "Go back"}
	for i, opt := range opts {
		if i == m.warnCursor {
			b.WriteString("  " + highlightStyle.Render("▸ "+opt) + "\n")
		} else {
			b.WriteString("    " + opt + "\n")
		}
	}
	b.WriteString(helpStyle.Render("↑/↓ move · enter select · esc back · ctrl+c quit"))
}

func (m *modelPickerModel) renderPullConfirm(b *strings.Builder) {
	b.WriteString("  " + errorStyle.Render("⬇  Not downloaded yet") + "\n\n")
	b.WriteString("  " + m.pendingModel + "\n\n")
	b.WriteString("  Running this model will download it first.\n")
	b.WriteString("  " + hintStyle.Render("This may take several minutes depending on your connection.") + "\n\n")

	opts := []string{"Download and run", "Cancel"}
	for i, opt := range opts {
		if i == m.pullCursor {
			b.WriteString("  " + highlightStyle.Render("▸ "+opt) + "\n")
		} else {
			b.WriteString("    " + opt + "\n")
		}
	}
	b.WriteString(helpStyle.Render("↑/↓ move · enter select · esc back · ctrl+c quit"))
}

func (m *modelPickerModel) renderList(b *strings.Builder) {
	if len(m.visible) == 0 {
		if m.searchQuery != "" {
			b.WriteString("  " + hintStyle.Render("No models match \""+m.searchQuery+"\"") + "\n")
		} else {
			b.WriteString("  " + hintStyle.Render("No models found. Pull one with `ollama pull <model>`.") + "\n")
		}
		b.WriteString(helpStyle.Render("backspace clear · esc back · ctrl+c quit"))
		return
	}

	m.clampScroll()
	vp := m.listViewportHeight()
	end := m.scrollOffset + vp
	if end > len(m.visible) {
		end = len(m.visible)
	}

	for i := m.scrollOffset; i < end; i++ {
		item := m.visible[i]
		if item.isSection {
			if i > m.scrollOffset {
				b.WriteString("\n")
			}
			b.WriteString("  " + sectionStyle.Render(item.section) + "\n")
			continue
		}
		selected := i == m.cursor
		marker := "    "
		if selected {
			marker = "  " + highlightStyle.Render("▸ ")
		}
		name := item.name
		if selected {
			name = highlightStyle.Render(item.name)
		}
		suffix := ""
		switch item.kind {
		case kindCloud:
			suffix = "  " + hintStyle.Render("☁ cloud")
		case kindPull:
			suffix = "  " + hintStyle.Render("↓ pull")
		}
		b.WriteString(marker + name + suffix + "\n")
	}

	// Scroll indicator when there are hidden items.
	if m.scrollOffset > 0 || end < len(m.visible) {
		above := m.scrollOffset
		below := len(m.visible) - end
		indicator := ""
		if above > 0 && below > 0 {
			indicator = fmt.Sprintf("↑ %d more  ↓ %d more", above, below)
		} else if above > 0 {
			indicator = fmt.Sprintf("↑ %d more", above)
		} else {
			indicator = fmt.Sprintf("↓ %d more", below)
		}
		b.WriteString("  " + hintStyle.Render(indicator) + "\n")
	}

	b.WriteString(helpStyle.Render("↑/↓ move · enter select · type to search · esc back"))
}
