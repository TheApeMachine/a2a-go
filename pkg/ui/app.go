package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/sse"
	"github.com/theapemachine/a2a-go/pkg/stores/s3"
)

// panel identifiers used to track focus.
type panel int

const (
	agentListPanel panel = iota
	agentDetailPanel
	taskListPanel
	inputPanel
)

// App is the main application model combining all components together.
type App struct {
	help help.Model

	layout Layout

	agentList  AgentList
	detailView DetailView
	taskList   TaskList
	input      InputArea

	focusedPanel panel
	showHelp     bool

	catalogClient *catalog.CatalogClient
	selectedAgent *a2a.AgentCard
	selectedTask  *a2a.Task
	tasks         []a2a.Task
	sseClient     *sse.Client
	cancelSSE     context.CancelFunc

	statusMessage string
	errorMessage  string
}

// NewApp creates and initializes the application.
func NewApp(catalogURL string) *App {
	client := catalog.NewCatalogClient(catalogURL)
	return &App{
		help:          help.New(),
		catalogClient: client,
		agentList:     NewAgentList(),
		taskList:      NewTaskList(),
		detailView:    NewDetailView(),
		input:         NewInputArea(),
		focusedPanel:  agentListPanel,
	}
}

func (app *App) Init() tea.Cmd {
	return tea.Batch(app.fetchAgents, textarea.Blink)
}

// fetchAgents retrieves the list of agents from the catalog.
func (app *App) fetchAgents() tea.Msg {
	agents, err := app.catalogClient.GetAgents()
	if err != nil {
		return errorMsg{err}
	}
	app.agentList.SetItems(agents)
	return nil
}

// SafeUpdate is used by the cmd package to ensure panics do not crash the UI.
func (app *App) SafeUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("Recovered from panic", "error", r)
		}
	}()
	return app.Update(msg)
}

func (app *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, defaultKeymap.quit):
			return app, tea.Quit
		case key.Matches(km, defaultKeymap.help):
			app.showHelp = !app.showHelp
			return app, nil
		case key.Matches(km, defaultKeymap.tab):
			switch app.focusedPanel {
			case agentListPanel:
				app.focusedPanel = agentDetailPanel
			case agentDetailPanel:
				app.focusedPanel = inputPanel
			case inputPanel:
				app.focusedPanel = taskListPanel
			case taskListPanel:
				app.focusedPanel = agentListPanel
			}
			return app, nil
		}
	}

	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		app.layout = NewLayout(m)
		app.agentList.SetSize(app.layout.SidebarWidth-2, app.layout.DetailHeight+app.layout.InputHeight-2)
		app.taskList.SetSize(app.layout.SidebarWidth-2, app.layout.DetailHeight+app.layout.InputHeight-2)
		app.detailView.SetSize(app.layout.CenterWidth-4, app.layout.DetailHeight-4)
		app.input.SetSize(app.layout.CenterWidth-6, app.layout.InputHeight-2)
		return app, nil

	case AgentSelectedMsg:
		app.selectedAgent = &m.Agent
		detailText := fmt.Sprintf("Name: %s\nURL: %s\nVersion: %s", m.Agent.Name, m.Agent.URL, m.Agent.Version)
		app.detailView.SetBaseContent(detailText)
		return app, tea.Batch(
			app.subscribeToEvents(m.Agent.URL),
			func() tea.Msg { return app.getTasksByAgent(m.Agent.Name) },
		)

	case TaskSelectedMsg:
		app.selectedTask = &m.Task
		return app, nil

	case SendInstructionsMsg:
		if app.selectedAgent == nil {
			app.errorMessage = "select an agent first"
			return app, nil
		}
		return app, app.sendInstruction(m.Text)

	case AppendDetailMsg:
		app.detailView, cmd = app.detailView.Update(m)
		return app, cmd

	case errorMsg:
		app.errorMessage = m.err.Error()
		return app, nil
	}

	// Update focused component
	switch app.focusedPanel {
	case agentListPanel:
		app.agentList, cmd = app.agentList.Update(msg)
	case taskListPanel:
		app.taskList, cmd = app.taskList.Update(msg)
	case inputPanel:
		app.input, cmd = app.input.Update(msg)
	case agentDetailPanel:
		app.detailView, cmd = app.detailView.Update(msg)
	}
	return app, cmd
}

func (app App) View() string {
	left := activeStyle.Render(app.agentList.View())
	if app.focusedPanel != agentListPanel {
		left = inactiveStyle.Render(app.agentList.View())
	}

	right := activeStyle.Render(app.taskList.View())
	if app.focusedPanel != taskListPanel {
		right = inactiveStyle.Render(app.taskList.View())
	}

	detail := activeStyle.Render(app.detailView.View())
	if app.focusedPanel != agentDetailPanel {
		detail = inactiveStyle.Render(app.detailView.View())
	}

	input := activeStyle.Render(app.input.View())
	if app.focusedPanel != inputPanel {
		input = inactiveStyle.Render(app.input.View())
	}

	center := lipgloss.JoinVertical(lipgloss.Left, detail, input)
	horizontal := lipgloss.JoinHorizontal(0, left, center, right)
	hm, vm := app.layout.Margins()
	return lipgloss.NewStyle().Padding(vm, hm).Render(horizontal)
}

func (app *App) sendInstruction(text string) tea.Cmd {
	return func() tea.Msg {
		agentClient := a2a.NewClient(app.selectedAgent.URL)
		message := a2a.Message{
			Parts: []a2a.Part{{Type: a2a.PartTypeText, Text: text}},
		}
		resp, err := agentClient.SendTaskSubscribe(a2a.TaskSendParams{ID: uuid.New().String(), SessionID: uuid.New().String(), Message: message})
		if err != nil {
			return errorMsg{err}
		}
		if resp.Error != nil {
			return errorMsg{fmt.Errorf("server error: %s", resp.Error.Message)}
		}
		var task a2a.Task
		var ok bool
		if task, ok = resp.Result.(a2a.Task); !ok {
			data, err := json.Marshal(resp.Result)
			if err != nil {
				return errorMsg{fmt.Errorf("unable to marshal task result: %w", err)}
			}
			if err := json.Unmarshal(data, &task); err != nil {
				return errorMsg{fmt.Errorf("unable to decode task result: %w", err)}
			}
		}
		app.tasks = append(app.tasks, task)
		app.taskList.SetItems(app.tasks)
		return AppendDetailMsg{Text: fmt.Sprintf("Task %s submitted", task.ID)}
	}
}

func (app *App) getStore() (*s3.Store, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "minio:9000"
	}
	minioClient, err := minio.New(endpoint, &minio.Options{
		Region: "us-east-1",
		Creds:  credentials.NewStaticV4(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}
	return s3.NewStore(s3.NewConn(s3.WithClient(minioClient))), nil
}

func (app *App) getTasksByAgent(agentName string) tea.Msg {
	store, err := app.getStore()
	if err != nil {
		return errorMsg{err}
	}
	tasks, rpcErr := store.Get(context.Background(), agentName, 100)
	if rpcErr != nil {
		return errorMsg{rpcErr}
	}
	if tasks != nil {
		app.tasks = tasks
	}
	app.taskList.SetItems(app.tasks)
	return nil
}

func (app *App) subscribeToEvents(url string) tea.Cmd {
	return func() tea.Msg {
		if app.cancelSSE != nil {
			app.cancelSSE()
		}
		ctx, cancel := context.WithCancel(context.Background())
		app.cancelSSE = cancel
		client := sse.NewClient(url + "/events")
		app.sseClient = client
		go func() {
			_ = client.SubscribeWithContext(ctx, "", func(e *sse.Event) {
				app.SafeUpdate(AppendDetailMsg{Text: string(e.Data)})
			})
		}()
		return nil
	}
}
