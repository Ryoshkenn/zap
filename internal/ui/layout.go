package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Fallback terminal size used until the first tea.WindowSizeMsg arrives.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

// size returns the terminal size, falling back to defaults before the first
// WindowSizeMsg.
func (a *app) size() (w, h int) {
	w, h = a.width, a.height
	if w <= 0 {
		w = defaultWidth
	}
	if h <= 0 {
		h = defaultHeight
	}
	return w, h
}

// bodyHeight is how many rows a screen may use for its scrollable body after
// reserving overhead rows for its own title/help and the update banner.
func (a *app) bodyHeight(overhead int) int {
	_, h := a.size()
	h -= overhead + a.bannerHeight()
	if h < 3 {
		h = 3
	}
	return h
}

// listHeight is the height given to bubbles/list screens: everything except
// the help line below the list (1 row of padding + 1 row of text).
func (a *app) listHeight() int {
	return a.bodyHeight(2)
}

// resize re-applies the terminal size to every live list. Screens are built
// lazily, so a resize (or the update banner appearing) must reach lists that
// are not currently on screen too.
func (a *app) resize() {
	w, _ := a.size()
	if a.folder != nil {
		a.folder.list.SetSize(w, a.listHeight())
	}
	if a.provider != nil {
		a.provider.list.SetSize(w, a.listHeight())
	}
}

// scrollWindow returns the [start, end) slice of n rows to render in a window
// of height rows so that cursor stays visible. offset is the previous start;
// keeping it when possible stops the view from jumping on every keypress.
func scrollWindow(offset, cursor, n, height int) (start, end int) {
	if height <= 0 || n <= height {
		return 0, n
	}
	start = offset
	if cursor < start {
		start = cursor
	}
	if cursor >= start+height {
		start = cursor - height + 1
	}
	if start > n-height {
		start = n - height
	}
	if start < 0 {
		start = 0
	}
	return start, start + height
}

// scrollHint describes rows hidden above/below a scroll window, or "" if none.
func scrollHint(start, end, n int) string {
	var parts []string
	if start > 0 {
		parts = append(parts, fmt.Sprintf("↑ %d more", start))
	}
	if end < n {
		parts = append(parts, fmt.Sprintf("↓ %d more", n-end))
	}
	return strings.Join(parts, "  ")
}

// truncate cuts a single line s (which may contain ANSI styling) to at most
// width cells.
func truncate(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

// clipLines truncates every line of view to width so nothing wraps: a wrapped
// line pushes the whole screen down and scrolls the title out of view.
func clipLines(view string, width int) string {
	if width <= 0 {
		return view
	}
	lines := strings.Split(view, "\n")
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, width, "")
	}
	return strings.Join(lines, "\n")
}
