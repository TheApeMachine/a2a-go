package ui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/catalog"
)

const gap = "\n\n"

type model struct {
	viewport      viewport.Model
	messages      []string
	textarea      textarea.Model
	senderStyle   lipgloss.Style
	agentStyle    lipgloss.Style
	errorStyle    lipgloss.Style
	catalogClient *catalog.CatalogClient
	sessionID     string
	err           error
}

func New() tea.Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message to the UI agent..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 500

	ta.SetWidth(80)
	ta.SetHeight(3)

	// Remove cursor line styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent(`Welcome to the A2A Agent Chat!
Type a message and press Enter to send to the UI agent.
The UI agent will relay your message to the appropriate agent.
Press Ctrl+C or Esc to quit.`)

	ta.KeyMap.InsertNewline.SetEnabled(false)

	// Initialize catalog client
	v := viper.GetViper()
	catalogURL := v.GetString("endpoints.catalog")

	// Convert Docker internal URLs to localhost when running locally
	if catalogURL == "" || strings.Contains(catalogURL, "catalog:3210") {
		catalogURL = "http://localhost:3210"
	}

	return model{
		textarea:      ta,
		messages:      []string{},
		viewport:      vp,
		senderStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true), // Blue for user
		agentStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true), // Green for agent
		errorStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),  // Red for errors
		catalogClient: catalog.NewCatalogClient(catalogURL),
		sessionID:     uuid.New().String(),
		err:           nil,
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.Height = msg.Height - m.textarea.Height() - lipgloss.Height(gap) - 2

		if len(m.messages) > 0 {
			m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
		}
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			userMessage := strings.TrimSpace(m.textarea.Value())
			if userMessage != "" {
				// Add user message
				m.messages = append(m.messages, m.senderStyle.Render("You: ")+userMessage)

				// Get agent response
				agentResponse := m.sendToUIAgent(userMessage)
				m.messages = append(m.messages, agentResponse)

				// Update viewport
				m.viewport.SetContent(lipgloss.NewStyle().Width(m.viewport.Width).Render(strings.Join(m.messages, "\n")))
				m.textarea.Reset()
				m.viewport.GotoBottom()
			}
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m model) View() string {
	return fmt.Sprintf(
		"%s%s%s",
		m.viewport.View(),
		gap,
		m.textarea.View(),
	)
}

// sendToUIAgent communicates with the actual UI agent via A2A protocol
func (m model) sendToUIAgent(userMessage string) string {
	// Get UI agent from catalog
	agents, err := m.catalogClient.GetAgents()
	if err != nil {
		return m.errorStyle.Render("Error: ") + "Failed to connect to catalog: " + err.Error()
	}

	var uiAgent *a2a.AgentCard
	for _, agent := range agents {
		if agent.Name == "User Interface Agent" {
			uiAgent = &agent
			break
		}
	}

	if uiAgent == nil {
		return m.errorStyle.Render("Error: ") + "UI Agent not found in catalog"
	}

	// Create A2A client for the UI agent
	agentURL := uiAgent.URL
	// If running locally and agent URL points to Docker internal network, use localhost
	if strings.Contains(agentURL, "ui:3210") {
		agentURL = "http://localhost:3212" // UI agent is mapped to port 3212 locally
	}

	agentClient := a2a.NewClient(agentURL)

	// Create message
	message := a2a.NewTextMessage("user", userMessage)
	message.Metadata = map[string]any{
		"origin": "ui-chat",
		"client": "terminal-ui",
	}

	// Send task to UI agent
	taskID := uuid.New().String()

	response, err := agentClient.SendTask(a2a.TaskSendParams{
		ID:        taskID,
		SessionID: m.sessionID,
		Message:   *message,
	})

	if err != nil {
		return m.errorStyle.Render("Error: ") + "Failed to communicate with UI agent: " + err.Error()
	}

	if response.Error != nil {
		return m.errorStyle.Render("Error: ") + fmt.Sprintf("Agent error (Code %d): %s", response.Error.Code, response.Error.Message)
	}

	// Extract response from task
	if response.Result != nil {
		// Marshal the result back to JSON then unmarshal to Task struct
		resultBytes, err := json.Marshal(response.Result)
		if err != nil {
			return m.errorStyle.Render("Error: ") + "Failed to parse agent response: " + err.Error()
		}

		var task a2a.Task
		if err := json.Unmarshal(resultBytes, &task); err != nil {
			return m.errorStyle.Render("Error: ") + "Failed to parse task data: " + err.Error()
		}

		// Check for agent responses in history
		for i := len(task.History) - 1; i >= 0; i-- {
			msg := task.History[i]
			if msg.Role == "assistant" && len(msg.Parts) > 0 {
				return m.agentStyle.Render("Agent: ") + msg.Parts[0].Text
			}
		}

		// Fallback to status message
		if len(task.Status.Message.Parts) > 0 {
			return m.agentStyle.Render("Agent: ") + task.Status.Message.Parts[0].Text
		}
	}

	return m.errorStyle.Render("Error: ") + "No response received from agent"
}
