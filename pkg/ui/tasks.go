package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

// taskItem implements list.Item for tasks.
type taskItem struct{ task a2a.Task }

func (i taskItem) Title() string       { return i.task.ID }
func (i taskItem) Description() string { return string(i.task.Status.State) }
func (i taskItem) FilterValue() string { return i.task.ID }

// TaskList is a bubbletea component for displaying tasks.
type TaskList struct {
	list list.Model
}

func NewTaskList() TaskList {
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(3)
	delegate.SetSpacing(1)
	delegate.ShortHelpFunc = func() []key.Binding { return []key.Binding{} }
	delegate.FullHelpFunc = func() [][]key.Binding { return [][]key.Binding{} }

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Tasks"
	l.Styles.Title = titleStyle
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	l.KeyMap = newDelegateKeyMap()

	return TaskList{list: l}
}

func (tl TaskList) Init() tea.Cmd { return nil }

func (tl TaskList) Update(msg tea.Msg) (TaskList, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(m, defaultKeymap.enter) {
			if tl.list.SelectedItem() == nil {
				return tl, nil
			}
			if it, ok := tl.list.SelectedItem().(taskItem); ok {
				return tl, func() tea.Msg { return TaskSelectedMsg{Task: it.task} }
			}
			return tl, nil
		}
	}
	var cmd tea.Cmd
	tl.list, cmd = tl.list.Update(msg)
	return tl, cmd
}

func (tl TaskList) View() string { return tl.list.View() }

func (tl *TaskList) SetSize(w, h int) { tl.list.SetSize(w, h) }

func (tl *TaskList) SetItems(tasks []a2a.Task) {
	items := make([]list.Item, len(tasks))
	for i, task := range tasks {
		t := task
		items[i] = taskItem{task: t}
	}
	tl.list.SetItems(items)
}
