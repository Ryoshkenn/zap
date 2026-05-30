package ui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD580")).
			Bold(true).
			Padding(0, 1)

	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD580"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(1, 0, 0, 2)

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Faint(true)

	starStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD580"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B"))

	// hintStyle is for secondary info (download hints, sizes) — dim but readable.
	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	// sectionStyle is for section headers in lists.
	sectionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(true)
)
