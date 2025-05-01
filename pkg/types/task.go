package types

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
TaskState enumerates the mutually‑exclusive states a task may be in.  The
zero value is "unknown" per the spec.
*/
type TaskState string

const (
	TaskStateSubmitted TaskState = "submitted"
	TaskStateWorking   TaskState = "working"
	TaskStateInputReq  TaskState = "input-required"
	TaskStateCompleted TaskState = "completed"
	TaskStateCanceled  TaskState = "canceled"
	TaskStateFailed    TaskState = "failed"
	TaskStateUnknown   TaskState = "unknown"
)

type TaskStatus struct {
	State     TaskState  `json:"state"`
	Message   *Message   `json:"message,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
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

type Task struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionId,omitempty"`
	Status    TaskStatus     `json:"status"`
	History   []Message      `json:"history,omitempty"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func (task *Task) AddMessage(role, name, text string) {
	task.History = append(task.History, Message{
		Role:  role,
		Parts: []Part{{Type: PartTypeText, Text: text}},
		Metadata: map[string]any{
			"name": name,
		},
	})
}

func (task *Task) AddArtifact(artifact Artifact) {
	task.Artifacts = append(task.Artifacts, artifact)
}

func (task *Task) ToState(state TaskState, message string) {
	log.Info("task state updated", "task", task.ID, "state", state, "message", message)

	task.Status = TaskStatus{
		State:     state,
		Message:   &Message{Role: "agent", Parts: []Part{{Type: PartTypeText, Text: message}}},
		Timestamp: utils.Ptr(time.Now()),
	}
}

type TaskGetParams struct {
	ID string `json:"id"`
}

type TaskCancelParams struct {
	ID string `json:"id"`
}

type TaskResubscribeParams struct {
	ID string `json:"id"`
}

type TaskPushNotificationParams struct {
	ID string `json:"id"`
}

// TaskSendParams defines the parameters for the tasks/send and tasks/sendSubscribe methods.
type TaskSendParams struct {
	ID               string                  `json:"id"`
	SessionID        *string                 `json:"sessionId,omitempty"` // Optional
	Message          Message                 `json:"message"`
	PushNotification *PushNotificationConfig `json:"pushNotification,omitempty"` // Optional
	HistoryLength    *int                    `json:"historyLength,omitempty"`    // Optional
	Metadata         map[string]any          `json:"metadata,omitempty"`         // Optional
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
	if task.Status.Timestamp != nil {
		sb.WriteString(bullet + labelStyle.Render("Timestamp: ") + valueStyle.Render(task.Status.Timestamp.Format(time.RFC3339)) + "\n")
	}

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
