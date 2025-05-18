package a2a

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/cohesivestack/valgo"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/transport"
)

type Task struct {
	*transport.Stream[Task]
	ID        string         `json:"id"`
	SessionID string         `json:"sessionId,omitempty"`
	Status    TaskStatus     `json:"status"`
	History   []Message      `json:"history,omitempty"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func (task *Task) Validate() bool {
	return valgo.Is(
		valgo.String(task.ID).Not().Blank(),
		valgo.String(task.SessionID).Not().Blank(),
		valgo.String(task.Status.State).Not().Blank(),
		valgo.String(task.Status.Message.Parts[0].Text).Not().Blank(),
	).Valid()
}

func NewTask(agentName string) *Task {
	task := &Task{
		ID:        uuid.New().String(),
		SessionID: uuid.New().String(),
		Status: TaskStatus{
			State:   TaskStateSubmitted,
			Message: NewTextMessage("system", "Task created"),
		},
		History:   make([]Message, 0),
		Artifacts: make([]Artifact, 0),
		Metadata:  make(map[string]any),
	}

	task.History = append(task.History, *NewTextMessage("system",
		viper.GetViper().GetString(fmt.Sprintf("agent.%s.system", agentName)),
	))

	return task
}

func NewTaskFromRequest(body []byte) (*Task, error) {
	var task Task
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

/*
Prefix returns a deterministic S3 compatible storage key for a task.
It consists out of the following segments, which have been selected for
the following reasons:

  - State: at the front so no matter how much time has passed, we can always easily
    get all tasks for a given state.
  - SessionID: naturally groups all tasks for a given session, so we can easily retrieve
    all tasks for a given session.
  - ID: the main unique segment needed to make sure all tasks have their own
    space on storage, and no overwriting occurs.
  - Timestamp: makes it easy to get the latest version of a task, since updates do not overwrite,
    but instead follow an append only strategy.
*/
func (task *Task) Prefix(optionals ...string) string {
	builder := optionals
	builder = append(builder, []string{
		string(task.Status.State),
		task.SessionID,
		task.ID,
		strconv.FormatInt(time.Now().UnixNano(), 10),
	}...)

	return strings.Join(builder, "/")
}

func (task *Task) ToStatus(status TaskState, message *Message) {
	log.Info("task status update", "status", status, "message", message)

	task.Status.State = status
	task.Status.Timestamp = time.Now()
	task.Status.Message = message
}

func (task *Task) LastMessage() *Message {
	if len(task.History) == 0 {
		return nil
	}

	return &task.History[len(task.History)-1]
}

func (task *Task) AddArtifact(artifact Artifact) {
	task.Artifacts = append(task.Artifacts, artifact)
}

func (task *Task) AddFinalPart(part Part) {
	task.History = append(task.History, Message{
		Role:  "assistant",
		Parts: []Part{part},
	})
}

/*
TaskStatusUpdateEvent is sent when the agent wishes to inform the client of
a status transition.
*/
type TaskStatusUpdateEvent struct {
	ID       string         `json:"id"`
	Status   TaskStatus     `json:"status"`
	Final    bool           `json:"final"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

/*
TaskArtifactUpdateEvent is emitted when a new or updated artefact is
available for a task.
*/
type TaskArtifactUpdateEvent struct {
	ID       string         `json:"id"`
	Artifact Artifact       `json:"artifact"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskHistory represents the history of a task
type TaskHistory struct {
	// MessageHistory is the list of messages in chronological order
	MessageHistory []Message `json:"messageHistory,omitempty"`
}

// TaskSendParams represents the parameters for sending a task message
type TaskSendParams struct {
	ID               string                  `json:"id"`
	SessionID        string                  `json:"sessionId,omitempty"`
	Message          Message                 `json:"message"`
	PushNotification *PushNotificationConfig `json:"pushNotification,omitempty"`
	HistoryLength    *int                    `json:"historyLength,omitempty"`
	Metadata         map[string]any          `json:"metadata,omitempty"`
}

// TaskIDParams represents the base parameters for task ID-based operations
type TaskIDParams struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskQueryParams represents the parameters for querying task information
type TaskQueryParams struct {
	TaskIDParams
	HistoryLength *int `json:"historyLength,omitempty"`
}

// PushNotificationConfig represents the configuration for push notifications
type PushNotificationConfig struct {
	URL            string               `json:"url"`
	Token          *string              `json:"token,omitempty"`
	Authentication *AgentAuthentication `json:"authentication,omitempty"`
}

// TaskPushNotificationConfig represents the configuration for task-specific push notifications
type TaskPushNotificationConfig struct {
	ID                     string                 `json:"id"`
	PushNotificationConfig PushNotificationConfig `json:"pushNotificationConfig"`
}

// SendTaskRequest represents a request to send a task message
type SendTaskRequest struct {
	jsonrpc.Request
	Method string         `json:"method"`
	Params TaskSendParams `json:"params"`
}

// GetTaskRequest represents a request to get task status
type GetTaskRequest struct {
	jsonrpc.Request
	Method string          `json:"method"`
	Params TaskQueryParams `json:"params"`
}

// CancelTaskRequest represents a request to cancel a task
type CancelTaskRequest struct {
	jsonrpc.Request
	Method string       `json:"method"`
	Params TaskIDParams `json:"params"`
}

// SetTaskPushNotificationRequest represents a request to set task notifications
type SetTaskPushNotificationRequest struct {
	jsonrpc.Request
	Method string                     `json:"method"`
	Params TaskPushNotificationConfig `json:"params"`
}

// GetTaskPushNotificationRequest represents a request to get task notification configuration
type GetTaskPushNotificationRequest struct {
	jsonrpc.Request
	Method string       `json:"method"`
	Params TaskIDParams `json:"params"`
}

// TaskResubscriptionRequest represents a request to resubscribe to task updates
type TaskResubscriptionRequest struct {
	jsonrpc.Request
	Method string          `json:"method"`
	Params TaskQueryParams `json:"params"`
}

// SendTaskStreamingRequest represents a request to send a task message and subscribe to updates
type SendTaskStreamingRequest struct {
	jsonrpc.Request
	Method string         `json:"method"`
	Params TaskSendParams `json:"params"`
}

type TaskStatusUpdateResponse struct {
	jsonrpc.Response
	Result TaskStatusUpdateResult `json:"result"`
}

type TaskStatusUpdateResult struct {
	ID       string         `json:"id"`
	Status   TaskStatus     `json:"status"`
	Final    bool           `json:"final"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func (task *Task) String() string {
	var sb strings.Builder

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		Bold(true)

	// Indentation and box-drawing chars
	indent := "   "
	bullet := "│ "

	// Task Details Header
	sb.WriteString(headerStyle.Render("Task Details") + "\n")
	sb.WriteString(bullet + labelStyle.Render("ID: ") + valueStyle.Render(task.ID) + "\n")
	if task.SessionID != "" {
		sb.WriteString(bullet + labelStyle.Render("Session ID: ") + valueStyle.Render(task.SessionID) + "\n")
	}

	// Status Section
	sb.WriteString("\n" + sectionStyle.Render("Status") + "\n")
	sb.WriteString(bullet + labelStyle.Render("State: ") + valueStyle.Render(string(task.Status.State)) + "\n")
	if task.Status.Message != nil {
		sb.WriteString(bullet + labelStyle.Render("Message: ") + valueStyle.Render(task.Status.Message.Parts[0].Text) + "\n")
	}

	sb.WriteString(bullet + labelStyle.Render("Timestamp: ") + valueStyle.Render(task.Status.Timestamp.Format(time.RFC3339)) + "\n")

	// History Section
	if len(task.History) > 0 {
		sb.WriteString("\n" + sectionStyle.Render("History") + "\n")
		for i, message := range task.History {
			sb.WriteString(bullet + labelStyle.Render(fmt.Sprintf("Message %d", i+1)) + "\n")
			sb.WriteString(bullet + indent + labelStyle.Render("Role: ") + valueStyle.Render(message.Role) + "\n")
			if name, ok := message.Metadata["name"].(string); ok && name != "" {
				sb.WriteString(bullet + indent + labelStyle.Render("Name: ") + valueStyle.Render(name) + "\n")
			}
			for _, part := range message.Parts {
				sb.WriteString(bullet + indent + labelStyle.Render("Content: ") + valueStyle.Render(part.Text) + "\n")
			}
		}
	}

	// Artifacts Section
	if len(task.Artifacts) > 0 {
		sb.WriteString("\n" + sectionStyle.Render("Artifacts") + "\n")
		for i, artifact := range task.Artifacts {
			sb.WriteString(bullet + labelStyle.Render(fmt.Sprintf("Artifact %d", i+1)) + "\n")
			if artifact.Name != nil {
				sb.WriteString(bullet + indent + labelStyle.Render("Name: ") + valueStyle.Render(*artifact.Name) + "\n")
			}
			if artifact.Description != nil {
				sb.WriteString(bullet + indent + labelStyle.Render("Description: ") + valueStyle.Render(*artifact.Description) + "\n")
			}
			for j, part := range artifact.Parts {
				sb.WriteString(bullet + indent + labelStyle.Render(fmt.Sprintf("Part %d: ", j+1)) + valueStyle.Render(part.Text) + "\n")
			}
		}
	}

	// Metadata Section
	if len(task.Metadata) > 0 {
		sb.WriteString("\n" + sectionStyle.Render("Metadata") + "\n")
		keys := make([]string, 0, len(task.Metadata))
		for k := range task.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(bullet + labelStyle.Render(k+": ") + valueStyle.Render(fmt.Sprintf("%v", task.Metadata[k])) + "\n")
		}
	}

	return sb.String()
}
