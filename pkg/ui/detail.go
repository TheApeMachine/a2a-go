package ui

import tea "github.com/charmbracelet/bubbletea"
import "github.com/charmbracelet/bubbles/viewport"

// DetailView shows information about the selected agent and streaming output.
type DetailView struct {
	viewport viewport.Model
	content  string
}

func NewDetailView() DetailView {
	vp := viewport.New(0, 0)
	vp.Style = noborderStyle
	return DetailView{viewport: vp}
}

func (dv DetailView) Init() tea.Cmd { return nil }

func (dv DetailView) Update(msg tea.Msg) (DetailView, tea.Cmd) {
	switch m := msg.(type) {
	case AppendDetailMsg:
		if dv.content == "" {
			dv.content = m.Text
		} else {
			dv.content += "\n" + m.Text
		}
		dv.viewport.SetContent(dv.content)
		dv.viewport.GotoBottom()
		return dv, nil
	}
	var cmd tea.Cmd
	dv.viewport, cmd = dv.viewport.Update(msg)
	return dv, cmd
}

func (dv DetailView) View() string { return dv.viewport.View() }

func (dv *DetailView) SetSize(w, h int) {
	dv.viewport.Width = w
	dv.viewport.Height = h
	dv.viewport.SetContent(dv.content)
}

func (dv *DetailView) SetBaseContent(text string) {
	dv.content = text
	dv.viewport.SetContent(text)
	dv.viewport.GotoTop()
}
