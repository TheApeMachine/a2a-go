package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// InputArea is a textarea used for sending instructions to the agent.
type InputArea struct {
	textarea textarea.Model
}

func NewInputArea() InputArea {
	ta := textarea.New()
	ta.Placeholder = "Type instructions and press Ctrl+S or Shift+Enter..."
	ta.ShowLineNumbers = false
	taKeyMap := ta.KeyMap
	taKeyMap.InsertNewline.SetKeys("enter")
	ta.KeyMap = taKeyMap
	ta.Blur()
	return InputArea{textarea: ta}
}

func (ia InputArea) Init() tea.Cmd { return nil }

func (ia InputArea) Update(msg tea.Msg) (InputArea, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(m, defaultKeymap.send) || key.Matches(m, defaultKeymap.shiftEnter) {
			text := strings.TrimSpace(ia.textarea.Value())
			ia.textarea.Reset()
			if text != "" {
				return ia, func() tea.Msg { return SendInstructionsMsg{Text: text} }
			}
		}
	}
	var cmd tea.Cmd
	ia.textarea, cmd = ia.textarea.Update(msg)
	return ia, cmd
}

func (ia InputArea) View() string { return ia.textarea.View() }

func (ia *InputArea) Focus() { ia.textarea.Focus() }
func (ia *InputArea) Blur()  { ia.textarea.Blur() }
func (ia *InputArea) SetSize(w, h int) {
	ia.textarea.SetWidth(w)
	ia.textarea.SetHeight(h)
}
