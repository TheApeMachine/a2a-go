package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// UI color scheme
var (
	red      = lipgloss.AdaptiveColor{Light: "#FE5F86", Dark: "#FE5F86"}
	indigo   = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	green    = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
	blue     = lipgloss.AdaptiveColor{Light: "#1E88E5", Dark: "#42A5F5"}
	yellow   = lipgloss.AdaptiveColor{Light: "#FFC107", Dark: "#FFD54F"}
	gray     = lipgloss.AdaptiveColor{Light: "#9E9E9E", Dark: "#BDBDBD"}
	darkGray = lipgloss.AdaptiveColor{Light: "#424242", Dark: "#757575"}
)

// UI styles
var (
	// Base styles
	activeStyle = lipgloss.NewStyle().
			BorderForeground(indigo).
			BorderStyle(lipgloss.RoundedBorder())

	inactiveStyle = lipgloss.NewStyle().
			BorderForeground(gray).
			BorderStyle(lipgloss.RoundedBorder())

	noborderStyle = lipgloss.NewStyle()

	titleStyle = lipgloss.NewStyle().
			Foreground(indigo).
			Bold(true).
			Padding(0, 1)

	// Error and status styles
	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(gray).
			Padding(0, 1)

	// Panel styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("231")).
			Background(indigo).
			Padding(0, 1)
)

var (
	bgTextStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("239")).
			Padding(1, 2)

	dialogWordStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E7E1CC"))

	specialWordLightColor = lipgloss.Color("#43BF6D")
	specialWordDarkColor  = lipgloss.Color("#73F59F")
)
