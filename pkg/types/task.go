package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

type Task struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionId,omitempty"`
	Status    TaskStatus     `json:"status"`
	History   []Message      `json:"history,omitempty"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func (t *Task) String() string {
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
	sb.WriteString(bullet + labelStyle.Render("ID: ") + valueStyle.Render(t.ID) + "\n")
	if t.SessionID != "" {
		sb.WriteString(bullet + labelStyle.Render("Session ID: ") + valueStyle.Render(t.SessionID) + "\n")
	}

	// Status Section
	sb.WriteString("\n" + sectionStyle.Render("Status") + "\n")
	sb.WriteString(bullet + labelStyle.Render("State: ") + valueStyle.Render(string(t.Status.State)) + "\n")
	if t.Status.Message != nil {
		sb.WriteString(bullet + labelStyle.Render("Message: ") + valueStyle.Render(t.Status.Message.Parts[0].Text) + "\n")
	}
	if t.Status.Timestamp != nil {
		sb.WriteString(bullet + labelStyle.Render("Timestamp: ") + valueStyle.Render(t.Status.Timestamp.Format(time.RFC3339)) + "\n")
	}

	// History Section
	if len(t.History) > 0 {
		sb.WriteString("\n" + sectionStyle.Render("History") + "\n")
		for i, message := range t.History {
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
	if len(t.Artifacts) > 0 {
		sb.WriteString("\n" + sectionStyle.Render("Artifacts") + "\n")
		for i, artifact := range t.Artifacts {
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
	if len(t.Metadata) > 0 {
		sb.WriteString("\n" + sectionStyle.Render("Metadata") + "\n")
		keys := make([]string, 0, len(t.Metadata))
		for k := range t.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(bullet + labelStyle.Render(k+": ") + valueStyle.Render(fmt.Sprintf("%v", t.Metadata[k])) + "\n")
		}
	}

	return sb.String()
}

func (t *Task) Bytes() []byte {
	b, err := json.Marshal(t)
	if err != nil {
		return []byte{}
	}
	return b
}

func (t *Task) Reader() io.Reader {
	return bytes.NewReader(t.Bytes())
}

func (t *Task) AddMessage(role, name, text string) {
	t.History = append(t.History, Message{
		Role:  role,
		Parts: []Part{{Type: PartTypeText, Text: text}},
		Metadata: map[string]any{
			"name": name,
		},
	})
}

func (t *Task) AddArtifact(artifact Artifact) {
	t.Artifacts = append(t.Artifacts, artifact)
}

func (t *Task) ToState(state TaskState, message string) {
	log.Info("task state updated", "task", t.ID, "state", state, "message", message)

	t.Status = TaskStatus{
		State:     state,
		Message:   &Message{Role: "agent", Parts: []Part{{Type: PartTypeText, Text: message}}},
		Timestamp: utils.Ptr(time.Now()),
	}
}

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
