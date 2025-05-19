package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
)

// keymap defines the global key bindings used by the application and components.
type keymap struct {
	tab        key.Binding
	enter      key.Binding
	send       key.Binding
	shiftEnter key.Binding
	refresh    key.Binding
	help       key.Binding
	quit       key.Binding
}

func newKeymap() keymap {
	return keymap{
		tab:        key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch focus")),
		enter:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		send:       key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "send")),
		shiftEnter: key.NewBinding(key.WithKeys("shift+enter"), key.WithHelp("shift+enter", "send")),
		refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
		quit:       key.NewBinding(key.WithKeys("ctrl+c", "q", "esc"), key.WithHelp("ctrl+c", "quit")),
	}
}

// defaultKeymap provides a convenient globally accessible set of bindings.
var defaultKeymap = newKeymap()

// newDelegateKeyMap disables filtering and quit keys for list delegates and only
// exposes navigation shortcuts.
func newDelegateKeyMap() list.KeyMap {
	return list.KeyMap{
		CursorUp:      key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		CursorDown:    key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		PrevPage:      key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "prev page")),
		NextPage:      key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", "next page")),
		GoToStart:     key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "start")),
		GoToEnd:       key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "end")),
		Filter:        key.NewBinding(key.WithDisabled()),
		Quit:          key.NewBinding(key.WithDisabled()),
		ShowFullHelp:  key.NewBinding(key.WithDisabled()),
		CloseFullHelp: key.NewBinding(key.WithDisabled()),
	}
}
