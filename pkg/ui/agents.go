package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

// agentItem implements list.Item for the agent list.
type agentItem struct{ agent a2a.AgentCard }

func (i agentItem) Title() string { return i.agent.Name }
func (i agentItem) Description() string {
	if i.agent.Description != nil {
		return *i.agent.Description
	}
	return "No description available"
}
func (i agentItem) FilterValue() string { return i.agent.Name }

// AgentList is a bubbletea component displaying available agents.
type AgentList struct {
	list list.Model
}

func NewAgentList() AgentList {
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(3)
	delegate.SetSpacing(1)
	delegate.ShortHelpFunc = func() []key.Binding { return []key.Binding{} }
	delegate.FullHelpFunc = func() [][]key.Binding { return [][]key.Binding{} }

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Agents"
	l.Styles.Title = titleStyle
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	l.KeyMap = newDelegateKeyMap()

	return AgentList{list: l}
}

func (al AgentList) Init() tea.Cmd { return nil }

func (al AgentList) Update(msg tea.Msg) (AgentList, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(m, defaultKeymap.enter) {
			if al.list.SelectedItem() == nil {
				return al, nil
			}
			if it, ok := al.list.SelectedItem().(agentItem); ok {
				return al, func() tea.Msg { return AgentSelectedMsg{Agent: it.agent} }
			}
			return al, nil
		}
	}
	var cmd tea.Cmd
	al.list, cmd = al.list.Update(msg)
	return al, cmd
}

func (al AgentList) View() string { return al.list.View() }

func (al *AgentList) SetSize(w, h int) { al.list.SetSize(w, h) }

func (al *AgentList) SetItems(agents []a2a.AgentCard) {
	items := make([]list.Item, len(agents))
	for i, agent := range agents {
		items[i] = agentItem{agent: agent}
	}
	al.list.SetItems(items)
}
