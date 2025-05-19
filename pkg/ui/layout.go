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
	const inputVerticalPadding = 6 // borders and padding for textarea
	l.DetailHeight = (availableHeight - headerHeight) * 3 / 4
	l.InputHeight = availableHeight - headerHeight - l.DetailHeight - inputVerticalPadding

	const minInputHeight = 3
	if l.InputHeight < minInputHeight {
		l.InputHeight = minInputHeight
	}
	return l
}

func (l Layout) Margins() (int, int) {
	return l.horizontalMargin, l.verticalMargin
}
