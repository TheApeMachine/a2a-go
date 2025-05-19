package ui

import tea "github.com/charmbracelet/bubbletea"

// Layout contains the computed dimensions for all panels.
type Layout struct {
	Width  int
	Height int

	SidebarWidth int
	CenterWidth  int
	DetailHeight int
	InputHeight  int

	horizontalMargin int
	verticalMargin   int
}

// NewLayout calculates sizes for the different UI components based on the
// terminal window dimensions.
func NewLayout(msg tea.WindowSizeMsg) Layout {
	l := Layout{Width: msg.Width, Height: msg.Height, horizontalMargin: 2, verticalMargin: 2}
	availableWidth := msg.Width - (l.horizontalMargin * 2)
	availableHeight := msg.Height - (l.verticalMargin * 2)

	l.SidebarWidth = availableWidth / 4
	l.CenterWidth = availableWidth - (2 * l.SidebarWidth)

	headerHeight := 1
	l.DetailHeight = (availableHeight - headerHeight) * 3 / 4
	l.InputHeight = availableHeight - headerHeight - l.DetailHeight - 6
	return l
}

func (l Layout) Margins() (int, int) {
	return l.horizontalMargin, l.verticalMargin
}
