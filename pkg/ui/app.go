package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/sse"
	"github.com/theapemachine/a2a-go/pkg/stores/s3"
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

// Panel identifiers
type panel int

const (
	agentListPanel panel = iota
	agentDetailPanel
	taskListPanel
	inputPanel
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

// Message types for internal events
type fetchAgentsMsg struct{ agents []a2a.AgentCard }
type fetchAgentDetailMsg struct{ agent a2a.AgentCard }
type fetchTasksMsg struct{ tasks []a2a.Task }
type fetchTaskDetailMsg struct{ task a2a.Task }
type errorMsg struct{ err error }
type streamEventMsg struct{ event any }
type LogMsg struct{ Log string }

// Item implementations for the lists
type agentItem struct {
	agent a2a.AgentCard
}

func (i agentItem) Title() string {
	return i.agent.Name
}

func (i agentItem) Description() string {
	if i.agent.Description != nil {
		return *i.agent.Description
	}
	return "No description available"
}

func (i agentItem) FilterValue() string {
	return i.agent.Name
}

type taskItem struct {
	task a2a.Task
}

func (i taskItem) Title() string {
	return i.task.ID
}

func (i taskItem) Description() string {
	// Safe access to task fields
	state := string(i.task.Status.State)
	if state == "" {
		state = "unknown"
	}
	return fmt.Sprintf("Status: %s", state)
}

func (i taskItem) FilterValue() string {
	return i.task.ID
}

// Keymap for the application
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
		tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch focus"),
		),
		enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select item"),
		),
		send: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "send instructions"),
		),
		shiftEnter: key.NewBinding(
			key.WithKeys("shift+enter"),
			key.WithHelp("shift+enter", "send instructions"),
		),
		refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh data"),
		),
		help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q", "esc"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}
}

// App represents the main application state
type App struct {
	// UI components
	keymap             keymap
	help               help.Model
	width              int
	height             int
	agentList          list.Model
	taskList           list.Model
	agentDetail        viewport.Model
	agentDetailContent string // Stores the raw content for the agent detail viewport
	textarea           textarea.Model
	focusedPanel       panel
	showHelp           bool

	// Application state
	catalogClient *catalog.CatalogClient
	selectedAgent *a2a.AgentCard
	selectedTask  *a2a.Task
	tasks         []a2a.Task
	agents        []a2a.AgentCard
	statusMessage string
	errorMessage  string
	sseClient     *sse.Client
	cancelSSE     context.CancelFunc
}

// NewApp creates a new application with default state
func NewApp(catalogURL string) *App {
	// Set up keybindings
	keys := newKeymap()

	// Create delegate styles for list items
	delegateKeys := newDelegateKeyMap()
	agentDelegate := list.NewDefaultDelegate()
	agentDelegate.Styles.SelectedTitle = agentDelegate.Styles.SelectedTitle.Foreground(lipgloss.Color("231")).Background(indigo)
	agentDelegate.Styles.SelectedDesc = agentDelegate.Styles.SelectedDesc.Foreground(lipgloss.Color("231")).Background(indigo).Faint(false)
	agentDelegate.SetHeight(3)
	agentDelegate.SetSpacing(1)
	agentDelegate.ShortHelpFunc = func() []key.Binding { return []key.Binding{} }
	agentDelegate.FullHelpFunc = func() [][]key.Binding { return [][]key.Binding{} }

	taskDelegate := list.NewDefaultDelegate()
	taskDelegate.Styles.SelectedTitle = taskDelegate.Styles.SelectedTitle.Foreground(lipgloss.Color("231")).Background(indigo)
	taskDelegate.Styles.SelectedDesc = taskDelegate.Styles.SelectedDesc.Foreground(lipgloss.Color("231")).Background(indigo).Faint(false)
	taskDelegate.SetHeight(3)
	taskDelegate.SetSpacing(1)
	taskDelegate.ShortHelpFunc = func() []key.Binding { return []key.Binding{} }
	taskDelegate.FullHelpFunc = func() [][]key.Binding { return [][]key.Binding{} }

	// Create the agent list
	agentList := list.New([]list.Item{}, agentDelegate, 0, 0)
	agentList.Title = "Agents"
	agentList.Styles.Title = titleStyle
	agentList.SetShowHelp(false)
	agentList.SetFilteringEnabled(false)
	agentList.DisableQuitKeybindings()
	agentList.KeyMap = delegateKeys

	// Create the task list
	taskList := list.New([]list.Item{}, taskDelegate, 0, 0)
	taskList.Title = "Tasks"
	taskList.Styles.Title = titleStyle
	taskList.SetShowHelp(false)
	taskList.SetFilteringEnabled(false)
	taskList.DisableQuitKeybindings()
	taskList.KeyMap = delegateKeys

	// Create the agent detail viewport
	agentDetail := viewport.New(0, 0)
	agentDetail.Style = noborderStyle
	// Enable word wrapping for the viewport
	agentDetail.YPosition = 0

	// Create the textarea for new instructions
	ta := textarea.New()
	ta.Placeholder = "Type instructions to the agent and press Ctrl+S or Shift+Enter to send..."
	ta.ShowLineNumbers = false

	// Customize the textarea KeyMap to prevent Shift+Enter from adding a newline
	taKeyMap := ta.KeyMap
	taKeyMap.InsertNewline.SetKeys("enter") // Only regular enter adds a newline
	ta.KeyMap = taKeyMap

	ta.Blur()

	// Create and configure the catalog client
	client := catalog.NewCatalogClient(catalogURL)

	// Return the app
	return &App{
		keymap:             keys,
		agentList:          agentList,
		taskList:           taskList,
		agentDetail:        agentDetail,
		textarea:           ta,
		focusedPanel:       agentListPanel,
		catalogClient:      client,
		showHelp:           false,
		help:               help.New(),
		agentDetailContent: "Select an agent to see details and output.", // Initial content
	}
}

// Init initializes the application and returns the first commands to execute
func (app *App) Init() tea.Cmd {
	// Add a defer/recover to prevent any initialization panic from crashing the app
	defer func() {
		if r := recover(); r != nil {
			log.Error("Recovered from panic in Init", "error", r)
		}
	}()

	return tea.Batch(
		app.fetchAgents,
		textarea.Blink,
	)
}

// Custom delegate keymap that disables unwanted keys
func newDelegateKeyMap() list.KeyMap {
	return list.KeyMap{
		CursorUp:      key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
		CursorDown:    key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
		PrevPage:      key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "prev page")),
		NextPage:      key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", "next page")),
		GoToStart:     key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "go to start")),
		GoToEnd:       key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "go to end")),
		Filter:        key.NewBinding(key.WithDisabled()),
		Quit:          key.NewBinding(key.WithDisabled()),
		ShowFullHelp:  key.NewBinding(key.WithDisabled()),
		CloseFullHelp: key.NewBinding(key.WithDisabled()),
	}
}

// Set focus to a specific panel
func (app *App) setFocus(p panel) {
	// First, blur everything
	app.agentDetail.Style = noborderStyle
	app.textarea.Blur()

	// Then focus the selected panel
	app.focusedPanel = p

	switch p {
	case agentListPanel:
		app.agentList.SetDelegate(newActiveDelegate())
		app.taskList.SetDelegate(newInactiveDelegate())
		app.agentDetail.Style = noborderStyle
	case agentDetailPanel:
		app.agentList.SetDelegate(newInactiveDelegate())
		app.taskList.SetDelegate(newInactiveDelegate())
		app.agentDetail.Style = noborderStyle
	case taskListPanel:
		app.agentList.SetDelegate(newInactiveDelegate())
		app.taskList.SetDelegate(newActiveDelegate())
		app.agentDetail.Style = noborderStyle
	case inputPanel:
		app.agentList.SetDelegate(newInactiveDelegate())
		app.taskList.SetDelegate(newInactiveDelegate())
		app.agentDetail.Style = noborderStyle
		app.textarea.Focus()
	}
}

// Create active delegate for lists
func newActiveDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("231")).
		Background(indigo).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("231")).
		Background(indigo).
		Faint(false)
	delegate.SetHeight(3)
	delegate.SetSpacing(1)

	return delegate
}

// Create inactive delegate for lists
func newInactiveDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("231")).
		Background(gray).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("231")).
		Background(gray).
		Faint(false)
	delegate.SetHeight(3)
	delegate.SetSpacing(1)

	return delegate
}

// fetchAgents retrieves the list of agents from the catalog
func (app *App) fetchAgents() tea.Msg {
	agents, err := app.catalogClient.GetAgents()
	if err != nil {
		return errorMsg{err}
	}

	app.agents = agents
	return fetchAgentsMsg{agents}
}

// Update handles all state transitions based on messages
func (app *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Clear any previous error message when new actions are taken
	if _, isErrorMsg := msg.(errorMsg); !isErrorMsg {
		app.errorMessage = ""
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, app.keymap.quit):
			return app, tea.Quit

		case key.Matches(msg, app.keymap.help):
			app.showHelp = !app.showHelp

		case key.Matches(msg, app.keymap.tab):
			// Cycle through panels
			switch app.focusedPanel {
			case agentListPanel:
				if app.selectedAgent != nil {
					app.setFocus(agentDetailPanel)
				} else {
					app.setFocus(taskListPanel)
				}
			case agentDetailPanel:
				app.setFocus(inputPanel)
			case inputPanel:
				app.setFocus(taskListPanel)
			case taskListPanel:
				app.setFocus(agentListPanel)
			}

		case key.Matches(msg, app.keymap.enter):
			switch app.focusedPanel {
			case agentListPanel:
				if i, ok := app.agentList.SelectedItem().(agentItem); ok {
					app.selectedAgent = &i.agent

					// If this is the UI agent and the TUI is running on the host,
					// change its URL to localhost and the exposed port (3212 for ui agent).
					if app.selectedAgent.Name == "User Interface Agent" && app.selectedAgent.URL == "http://ui:3210" {
						app.selectedAgent.URL = "http://localhost:3212"
					}

					// Use raw content instead of styled String() content
					agentDetailText := fmt.Sprintf("Name: %s\n", app.selectedAgent.Name)
					if app.selectedAgent.Description != nil {
						agentDetailText += fmt.Sprintf("Description: %s\n", *app.selectedAgent.Description)
					}
					agentDetailText += fmt.Sprintf("URL: %s\n", app.selectedAgent.URL)
					agentDetailText += fmt.Sprintf("Version: %s\n", app.selectedAgent.Version)

					if app.selectedAgent.Provider != nil {
						agentDetailText += fmt.Sprintf("\nProvider: %s\n", app.selectedAgent.Provider.Organization)
						if app.selectedAgent.Provider.URL != nil {
							agentDetailText += fmt.Sprintf("Provider URL: %s\n", *app.selectedAgent.Provider.URL)
						}
					}

					agentDetailText += fmt.Sprintf("\nCapabilities:\n")
					agentDetailText += fmt.Sprintf("- Streaming: %v\n", app.selectedAgent.Capabilities.Streaming)
					agentDetailText += fmt.Sprintf("- Push Notifications: %v\n", app.selectedAgent.Capabilities.PushNotifications)
					agentDetailText += fmt.Sprintf("- State Transition History: %v\n", app.selectedAgent.Capabilities.StateTransitionHistory)

					if app.selectedAgent.Authentication != nil && app.selectedAgent.Authentication.Schemes != nil && len(app.selectedAgent.Authentication.Schemes) > 0 {
						agentDetailText += fmt.Sprintf("\nAuthentication Schemes: %s\n", strings.Join(app.selectedAgent.Authentication.Schemes, ", "))
					}

					if len(app.selectedAgent.Skills) > 0 {
						agentDetailText += fmt.Sprintf("\nSkills:\n")
						for _, skill := range app.selectedAgent.Skills {
							agentDetailText += fmt.Sprintf("- %s (%s)\n", skill.Name, skill.ID)
							if skill.Description != nil {
								agentDetailText += fmt.Sprintf("  %s\n", *skill.Description)
							}
						}
					}

					app.agentDetailContent = agentDetailText // Set base content
					app.agentDetail.SetContent(app.agentDetailContent)
					app.agentDetail.YPosition = 0 // Reset scroll to top
					app.setFocus(agentDetailPanel)

					// Fetch tasks for this agent when selected
					cmds = append(cmds, func() tea.Msg {
						return app.getTasksByAgent(app.selectedAgent.Name)
					})
					cmds = append(cmds, app.subscribeToEvents(app.selectedAgent.URL))

					app.statusMessage = fmt.Sprintf("Selected agent: %s", i.agent.Name)

					// Ensure word wrap works by resetting content at the current size
					// When window resizes, re-apply the stored content to the viewport
					// to ensure it reflows correctly.
					app.agentDetail.SetContent(app.agentDetailContent) // Re-set content after resize, using the stored raw content
				}
			case taskListPanel:
				selected := app.taskList.SelectedItem()
				if selected == nil {
					app.statusMessage = "No task selected"
					return app, nil
				}

				if i, ok := selected.(taskItem); ok {
					// Make a copy of the task to avoid memory issues
					taskCopy := i.task
					app.selectedTask = &taskCopy
					app.statusMessage = fmt.Sprintf("Selected task: %s", i.task.ID)
				}
			}

		case key.Matches(msg, app.keymap.refresh):
			switch app.focusedPanel {
			case agentListPanel:
				cmds = append(cmds, app.fetchAgents)
				app.statusMessage = "Refreshing agent list..."
			case taskListPanel:
				if app.selectedAgent != nil {
					cmds = append(cmds, func() tea.Msg {
						return app.getTasksByAgent(app.selectedAgent.Name)
					})
					app.statusMessage = "Refreshing task list..."
				}
			}

		case key.Matches(msg, app.keymap.send) || (key.Matches(msg, app.keymap.shiftEnter) && app.focusedPanel == inputPanel):
			if app.selectedAgent == nil || app.textarea.Value() == "" {
				app.statusMessage = "Please select an agent and enter instructions"
				return app, nil
			}

			// Create a cmd function to handle the sending in the background
			cmds = append(cmds, func() tea.Msg {
				agentClient := a2a.NewClient(app.selectedAgent.URL)
				message := a2a.Message{
					Role: "user",
					Parts: []a2a.Part{
						{
							Type: a2a.PartTypeText,
							Text: app.textarea.Value(),
						},
					},
				}
				response, err := agentClient.SendTaskSubscribe(a2a.TaskSendParams{
					ID:        uuid.New().String(),
					SessionID: uuid.New().String(),
					Message:   message,
				})

				if err != nil {
					return errorMsg{err: fmt.Errorf("error sending instructions: %w", err)}
				}

				// Check for JSON-RPC level error returned by the server
				if response.Error != nil {
					return errorMsg{err: fmt.Errorf("server error: %s (code: %d)", response.Error.Message, response.Error.Code)}
				}

				// If response.Error is nil, then response.Result should contain the actual result.
				if response.Result == nil {
					app.appendToAgentDetail(errorStyle.Render("Server returned success but with a nil result for send operation."))
					return errorMsg{err: fmt.Errorf("server returned success but with a nil result")} // Propagate as error to log it
				}

				var resultBytes []byte
				var marshalErr error

				// Attempt to convert response.Result to []byte for unmarshalling
				// Original code expected []byte, so try type assertion first.
				resultBytes, ok := response.Result.([]byte)
				if !ok {
					// If not already []byte, assume it might be a map[string]interface{} or similar
					// and try to marshal it to JSON bytes.
					log.Warn("response.Result is not []byte; attempting to marshal for unmarshalling", "type", fmt.Sprintf("%T", response.Result))
					resultBytes, marshalErr = json.Marshal(response.Result)
					if marshalErr != nil {
						return errorMsg{err: fmt.Errorf("cannot process result of type %T: %w", response.Result, marshalErr)}
					}
				}

				// Convert the response result to a Task
				task := &a2a.Task{}
				if err := json.Unmarshal(resultBytes, task); err != nil {
					app.appendToAgentDetail(errorStyle.Render(fmt.Sprintf("Failed to unmarshal task from send response: %s", err.Error())))
					return errorMsg{err: fmt.Errorf("failed to unmarshal task response: %w", err)}
				}

				// Add/Update task in local list and refresh UI task list
				app.upsertTaskInList(*task)
				app.appendToAgentDetail(fmt.Sprintf("Task %s submitted. Initial response received.", task.ID))

				// Reset the textarea after successful send
				app.textarea.Reset()

				// After sending, return both commands to refresh task list and subscribe to events
				return tea.Batch(
					app.subscribeToEvents(app.selectedAgent.URL),
					func() tea.Msg {
						return app.getTasksByAgent(app.selectedAgent.Name)
					},
				)
			})
		}

	case tea.WindowSizeMsg:
		app.width = msg.Width
		app.height = msg.Height

		// Add margins to prevent content from being cut off
		horizontalMargin := 2
		verticalMargin := 2
		availableWidth := app.width - (horizontalMargin * 2)
		availableHeight := app.height - (verticalMargin * 2)

		// Calculate panel dimensions
		sidebarWidth := availableWidth / 4 // 25% of width for each sidebar
		centerWidth := availableWidth - (2 * sidebarWidth)

		// Determine heights for the center panels
		headerHeight := 1
		detailHeight := (availableHeight - headerHeight) * 3 / 4         // Changed from 2/3 to 3/4
		inputHeight := availableHeight - headerHeight - detailHeight - 6 // account for borders and padding

		// Size the components, accounting for borders
		app.agentList.SetSize(sidebarWidth-2, availableHeight-2)
		app.taskList.SetSize(sidebarWidth-2, availableHeight-2)

		app.agentDetail.Width = centerWidth - 4 // Account for borders
		app.agentDetail.Height = detailHeight - 4
		// Ensure word wrap works by resetting content at the current size
		// When window resizes, re-apply the stored content to the viewport
		// to ensure it reflows correctly.
		app.agentDetail.SetContent(app.agentDetailContent) // Re-set content after resize, using the stored raw content

		app.textarea.SetWidth(centerWidth - 6) // Account for borders and inner padding
		app.textarea.SetHeight(inputHeight - 2)

		// Re-focus the active panel to refresh the styling
		app.setFocus(app.focusedPanel)

		return app, nil

	case LogMsg:
		app.appendToAgentDetail(msg.Log)
		return app, nil

	case fetchAgentsMsg:
		items := make([]list.Item, len(msg.agents))
		for i, agent := range msg.agents {
			items[i] = agentItem{agent: agent}
		}
		app.agentList.SetItems(items)
		app.statusMessage = fmt.Sprintf("Loaded %d agents", len(items))
		return app, nil

	case fetchTasksMsg:
		items := make([]list.Item, len(msg.tasks))
		for i, task := range msg.tasks {
			items[i] = taskItem{task: task}
		}
		app.taskList.SetItems(items)
		app.statusMessage = fmt.Sprintf("Loaded %d tasks", len(items))
		return app, nil

	case TaskMessage:
		// Safely handle empty task lists
		if len(msg.Tasks) == 0 {
			app.taskList.SetItems([]list.Item{})
			app.tasks = []a2a.Task{}
			app.statusMessage = "No tasks found"
			return app, nil
		}

		items := make([]list.Item, len(msg.Tasks))
		for i, task := range msg.Tasks {
			// Create a copy of the task to avoid pointer issues
			taskCopy := task
			items[i] = taskItem{task: taskCopy}
		}
		app.taskList.SetItems(items)
		app.tasks = msg.Tasks
		app.statusMessage = fmt.Sprintf("Loaded %d tasks", len(items))
		return app, nil

	case streamEventMsg:
		var taskUpdate a2a.Task
		// msg.event is string, convert to []byte for unmarshalling
		eventData := []byte(msg.event.(string))

		// Attempt 1: Try to unmarshal directly into a2a.Task
		if err := json.Unmarshal(eventData, &taskUpdate); err == nil {
			app.upsertTaskInList(taskUpdate)
			app.appendToAgentDetail(fmt.Sprintf("Task Event (%s): Status: %s, Message: %s",
				taskUpdate.ID, taskUpdate.Status.State, taskUpdate.Status.Message))
			return app, nil
		}

		// Attempt 2: Try to unmarshal into jsonrpc.Response
		var rpcResp jsonrpc.Response
		if err := json.Unmarshal(eventData, &rpcResp); err == nil {
			if rpcResp.Error != nil {
				app.appendToAgentDetail(errorStyle.Render(
					fmt.Sprintf("Stream Error: %s (Code: %d)", rpcResp.Error.Message, rpcResp.Error.Code),
				))
				return app, nil
			}

			if rpcResp.Result != nil {
				resultBytes, marshalErr := json.Marshal(rpcResp.Result)
				if marshalErr != nil {
					app.appendToAgentDetail(errorStyle.Render(
						fmt.Sprintf("Stream Event: Could not marshal rpcResp.Result: %v", marshalErr),
					))
					return app, nil
				}

				// Try to unmarshal rpcResp.Result into a2a.Task
				if err := json.Unmarshal(resultBytes, &taskUpdate); err == nil {
					app.upsertTaskInList(taskUpdate)
					app.appendToAgentDetail(fmt.Sprintf("Task Update via RPC Event (%s): Status: %s, Message: %s",
						taskUpdate.ID, taskUpdate.Status.State, taskUpdate.Status.Message))
					return app, nil
				}

				// If rpcResp.Result is not a task, append its string representation
				app.appendToAgentDetail(fmt.Sprintf("Stream Event (RPC Result): %s", string(resultBytes)))
				return app, nil
			}

			app.appendToAgentDetail(fmt.Sprintf("Stream Event (RPC): %s", msg.event)) // Should have Result or Error
			return app, nil
		}

		// If all unmarshalling fails, append the raw event string
		app.appendToAgentDetail(fmt.Sprintf("Stream Event (Raw): %s", msg.event))
		return app, nil

	case errorMsg:
		// app.errorMessage = msg.err.Error()
		app.appendToAgentDetail(errorStyle.Render(fmt.Sprintf("Error: %s", msg.err.Error())))
		log.Error("UI error", "error", msg.err)
		// Return with no commands - this prevents crashing on error
		return app, nil
	}

	// Handle component-specific updates
	switch app.focusedPanel {
	case agentListPanel:
		app.agentList, cmd = app.agentList.Update(msg)
		cmds = append(cmds, cmd)

	case taskListPanel:
		app.taskList, cmd = app.taskList.Update(msg)
		cmds = append(cmds, cmd)

	case agentDetailPanel:
		app.agentDetail, cmd = app.agentDetail.Update(msg)
		cmds = append(cmds, cmd)

	case inputPanel:
		app.textarea, cmd = app.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return app, tea.Batch(cmds...)
}

// Wrap Update with a recover function to prevent panics from crashing
func (app *App) SafeUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("Recovered from panic in Update", "error", r)
			app.errorMessage = fmt.Sprintf("Internal error: %v", r)
		}
	}()

	return app.Update(msg)
}

// View renders the UI based on current state
func (app *App) View() string {
	// Recover from any panics in View rendering
	defer func() {
		if r := recover(); r != nil {
			log.Error("Recovered from panic in View", "error", r)
			// Return a simple error view in case of panic
			app.errorMessage = fmt.Sprintf("Internal error in UI rendering: %v", r)
		}
	}()

	// If there's an error, show it in a non-blocking way
	if app.errorMessage != "" {
		// Create a styled error bar at the bottom of the screen
		// errorBar := errorStyle.Render(fmt.Sprintf("Error: %s", app.errorMessage))

		// Return normal UI with error bar at bottom
		// return lipgloss.JoinVertical(
		// 	lipgloss.Left,
		// 	app.renderMainUI(),
		// 	errorBar,
		// )
		// Error is now handled by appending to agentDetailContent, so this explicit block can be simplified or removed.
		// For now, we'll just fall through to renderMainUI which will include the error in agentDetail.
		app.errorMessage = "" // Clear it as it's now in agentDetailContent
	}

	return app.renderMainUI()
}

// renderMainUI generates the main UI layout without the error bar
func (app *App) renderMainUI() string {
	// Add margins to prevent content from being cut off
	horizontalMargin := 2
	verticalMargin := 2
	availableWidth := app.width - (horizontalMargin * 2)
	availableHeight := app.height - (verticalMargin * 2)

	// Calculate dimensions with margins
	sidebarWidth := availableWidth / 4
	centerWidth := availableWidth - (2 * sidebarWidth)
	headerHeight := 1
	detailHeight := (availableHeight - headerHeight) - 7
	inputHeight := 10

	// Define panel styles based on focus
	agentListStyle := inactiveStyle.Width(sidebarWidth - 2).Height(availableHeight - 2)
	taskListStyle := inactiveStyle.Width(sidebarWidth - 2).Height(availableHeight - 2)
	agentDetailStyle := inactiveStyle.Width(centerWidth - 4).Height(detailHeight - 4)
	inputStyle := inactiveStyle.Width(centerWidth - 4).Height(inputHeight - 2)

	// Highlight the focused panel
	switch app.focusedPanel {
	case agentListPanel:
		agentListStyle = activeStyle.Width(sidebarWidth - 2).Height(availableHeight - 2)
	case taskListPanel:
		taskListStyle = activeStyle.Width(sidebarWidth - 2).Height(availableHeight - 2)
	case agentDetailPanel:
		agentDetailStyle = activeStyle.Width(centerWidth - 4).Height(detailHeight - 4)
	case inputPanel:
		inputStyle = activeStyle.Width(centerWidth - 4).Height(inputHeight - 2)
	}

	// Render panels
	agentListView := agentListStyle.Render(app.agentList.View())

	// Agent detail view
	var agentDetailRenderedBox string
	if app.selectedAgent != nil {
		header := headerStyle.Render(fmt.Sprintf("AGENT: %s", app.selectedAgent.Name))
		// app.agentDetail already contains app.agentDetailContent due to SetContent calls
		// So, app.agentDetail.View() will render the scrollable content.
		contentForPanel := lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			app.agentDetail.View(), // This renders the viewport's current content
		)
		agentDetailRenderedBox = agentDetailStyle.Render(contentForPanel)
	} else {
		// If no agent is selected, render the placeholder content (already in app.agentDetail viewport)
		// inside the styled box.
		agentDetailRenderedBox = agentDetailStyle.Render(app.agentDetail.View())
	}

	// Input panel view
	var inputTitle string
	if app.selectedAgent != nil {
		inputTitle = headerStyle.Render(fmt.Sprintf("INSTRUCTIONS FOR: %s", app.selectedAgent.Name))
	} else {
		inputTitle = headerStyle.Render("SELECT AN AGENT FIRST")
	}
	inputView := inputStyle.Render(fmt.Sprintf("%s\n\n%s", inputTitle, app.textarea.View()))

	// Task list view
	taskListView := taskListStyle.Render(app.taskList.View())

	// Combine center panels vertically
	centerView := lipgloss.JoinVertical(
		lipgloss.Left,
		agentDetailRenderedBox, // Use the fully rendered box for the agent detail
		inputView,
	)

	// Combine columns horizontally
	mainView := lipgloss.JoinHorizontal(
		0,
		agentListView,
		centerView,
		taskListView,
	)

	// Add padding around the entire UI
	paddedView := lipgloss.NewStyle().
		Padding(verticalMargin, horizontalMargin).
		Render(mainView)

	return paddedView
}

func (app *App) getStore() (*s3.Store, error) {
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Region: "us-east-1",
		Creds: credentials.NewStaticV4(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			"",
		),
		Secure: false,
	})

	if err != nil {
		log.Error("failed to create minio client", "error", err)
		return nil, err
	}

	store := s3.NewStore(
		s3.NewConn(
			s3.WithClient(minioClient),
		),
	)

	return store, nil
}

/*
getTasksByAgent retrieves all tasks for a given agent.
Tasks are stored as prefixes in the s3 (compatible) bucket as:

<agentname>/<state>/<sessionid>/<taskid>/<timestamp>

Where state is the state of the task, sessionid is the session id of the task,
and taskid is the id of the task.

The timestamp is the unix nano timestamp of the task.

The task is stored as a json object in the s3 bucket.
*/
func (app *App) getTasksByAgent(agentName string) tea.Msg {
	store, err := app.getStore()
	if err != nil {
		return errorMsg{err: fmt.Errorf("failed to initialize store: %w", err)}
	}

	fetchedTasks, rpcErr := store.Get(context.Background(), agentName, 100)
	if rpcErr != nil {
		// Log the actual error, and then return it as a UI error message.
		log.Error("failed to get tasks from store", "agent", agentName, "error", rpcErr)
		return errorMsg{err: fmt.Errorf("failed to get tasks for agent %s: %w", agentName, rpcErr)}
	}

	// Use fetched tasks or empty slice
	var tasksToUpdate []a2a.Task
	if fetchedTasks != nil && len(fetchedTasks) > 0 {
		tasksToUpdate = fetchedTasks
	} else {
		tasksToUpdate = []a2a.Task{}
	}

	return TaskMessage{Tasks: tasksToUpdate}
}

/*
getTasksByID retrieves a single task, with all possible update events.
Tasks are stored as prefixes in the s3 (compatible) bucket as:

<agentname>/<state>/<sessionid>/<taskid>/<timestamp>

Where timestamp is the unix nano timestamp of the task.
*/
func (app *App) getTasksByID(
	agentName, sessionID, taskID string,
) tea.Msg {
	store, err := app.getStore()
	if err != nil {
		return errorMsg{err: fmt.Errorf("failed to initialize store: %w", err)}
	}

	var tasks []a2a.Task
	fetchedTasks, err := store.Get(context.Background(), agentName+"/"+sessionID+"/"+taskID, 100)
	if err.Error() != "RPC error: (nil RpcError)" {
		log.Error("failed to get tasks", "error", err)
		return errorMsg{err: fmt.Errorf("failed to get tasks: %w", err)}
	}

	// Use fetched tasks or empty slice
	if fetchedTasks != nil && len(fetchedTasks) > 0 {
		tasks = fetchedTasks
	} else {
		tasks = []a2a.Task{}
	}

	return TaskMessage{Tasks: tasks}
}

// subscribeToEvents connects to the agent's /events SSE endpoint and forwards
// events to the application.
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
				app.SafeUpdate(streamEventMsg{event: string(e.Data)})
			})
		}()
		return nil
	}
}

type TaskMessage struct {
	Tasks []a2a.Task
}

// appendToAgentDetail adds new text to the agent detail viewport and scrolls to the bottom.
func (app *App) appendToAgentDetail(newText string) {
	if strings.TrimSpace(app.agentDetailContent) == strings.TrimSpace("Select an agent to see details and output.") || app.agentDetailContent == "" {
		// If it's the initial message or empty, replace it
		app.agentDetailContent = newText
	} else {
		// Otherwise, append
		app.agentDetailContent += "\n" + newText
	}
	app.agentDetail.SetContent(app.agentDetailContent)
	app.agentDetail.GotoBottom()
}

// upsertTaskInList adds or updates a task in the app.tasks slice and refreshes the list.Model
func (app *App) upsertTaskInList(taskToUpdate a2a.Task) {
	found := false
	for i, t := range app.tasks {
		if t.ID == taskToUpdate.ID {
			app.tasks[i] = taskToUpdate
			found = true
			break
		}
	}
	if !found {
		app.tasks = append(app.tasks, taskToUpdate)
	}

	// Rebuild items for the list model
	items := make([]list.Item, len(app.tasks))
	for i, t := range app.tasks {
		taskCopy := t // Avoid pointer issues with loop variable
		items[i] = taskItem{task: taskCopy}
	}
	app.taskList.SetItems(items)
}
