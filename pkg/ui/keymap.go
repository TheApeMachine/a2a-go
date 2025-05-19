package ui

import "github.com/charmbracelet/bubbles/key"

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
