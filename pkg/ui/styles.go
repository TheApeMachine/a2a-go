package ui

import "github.com/charmbracelet/lipgloss"

// Color palette used across the UI.
var (
	red      = lipgloss.AdaptiveColor{Light: "#FE5F86", Dark: "#FE5F86"}
	indigo   = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	green    = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
	blue     = lipgloss.AdaptiveColor{Light: "#1E88E5", Dark: "#42A5F5"}
	yellow   = lipgloss.AdaptiveColor{Light: "#FFC107", Dark: "#FFD54F"}
	gray     = lipgloss.AdaptiveColor{Light: "#9E9E9E", Dark: "#BDBDBD"}
	darkGray = lipgloss.AdaptiveColor{Light: "#424242", Dark: "#757575"}
)

// Base styles that components can reuse.
var (
	activeStyle   = lipgloss.NewStyle().BorderForeground(indigo).BorderStyle(lipgloss.RoundedBorder())
	inactiveStyle = lipgloss.NewStyle().BorderForeground(gray).BorderStyle(lipgloss.RoundedBorder())
	noborderStyle = lipgloss.NewStyle()

	titleStyle     = lipgloss.NewStyle().Foreground(indigo).Bold(true).Padding(0, 1)
	errorStyle     = lipgloss.NewStyle().Foreground(red).Bold(true)
	statusBarStyle = lipgloss.NewStyle().Foreground(gray).Padding(0, 1)
	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(indigo).Padding(0, 1)
)
