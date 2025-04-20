package types

import (
	"context"
	"fmt"
	"time"

	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

type IdentifiableTaskManager interface {
	TaskManager
	Card() AgentCard
}

// TaskManager is plugged into an A2AServer.  Each method should do its own
// validation and return a *errors.RpcError value if the request is invalid or cannot
// be fulfilled.
type TaskManager interface {
	SendTask(context.Context, Task) (Task, *errors.RpcError)
	GetTask(context.Context, string, int) (Task, *errors.RpcError)
	CancelTask(context.Context, string) (Task, *errors.RpcError)
	StreamTask(context.Context, Task) (<-chan any, *errors.RpcError)
	ResubscribeTask(context.Context, string, int) (<-chan any, *errors.RpcError)
	SetPushNotification(context.Context, TaskPushNotificationConfig) (TaskPushNotificationConfig, *errors.RpcError)
	GetPushNotification(context.Context, string) (TaskPushNotificationConfig, *errors.RpcError)
}

/*
AgentCard conveys the top‑level capabilities and metadata exposed by a remote
agent that supports the A2A protocol.
*/
type AgentCard struct {
	Name               string               `json:"name"`
	Description        *string              `json:"description,omitempty"`
	URL                string               `json:"url"`
	Provider           *AgentProvider       `json:"provider,omitempty"`
	Version            string               `json:"version"`
	DocumentationURL   *string              `json:"documentationUrl,omitempty"`
	Capabilities       AgentCapabilities    `json:"capabilities"`
	Authentication     *AgentAuthentication `json:"authentication,omitempty"`
	DefaultInputModes  []string             `json:"defaultInputModes,omitempty"`
	DefaultOutputModes []string             `json:"defaultOutputModes,omitempty"`
	Skills             []AgentSkill         `json:"skills"`
}

func (card *AgentCard) Tools() map[string]*MCPClient {
	skillTools := map[string]*MCPClient{}

	for _, skill := range card.Skills {
		switch skill.ID {
		case "development":
			skillTools[skill.ID] = ToMCPTool(skill)
		}
	}

	return skillTools
}

type AgentProvider struct {
	Organization string  `json:"organization"`
	URL          *string `json:"url,omitempty"`
}

type AgentCapabilities struct {
	Streaming              bool `json:"streaming,omitempty"`
	PushNotifications      bool `json:"pushNotifications,omitempty"`
	StateTransitionHistory bool `json:"stateTransitionHistory,omitempty"`
}

type AgentAuthentication struct {
	Schemes     []string `json:"schemes"`
	Credentials *string  `json:"credentials,omitempty"`
}

type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

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

type Artifact struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Parts       []Part         `json:"parts"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Index       int            `json:"index,omitempty"`
	Append      *bool          `json:"append,omitempty"`
	LastChunk   *bool          `json:"lastChunk,omitempty"`
}

// Message represents all non‑artefact communication between client & agent.
type Message struct {
	Role     string         `json:"role"` // "user" or "agent"
	Parts    []Part         `json:"parts"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

/*
Part is a discriminated union over Text, File and Data parts.  We keep it
simple by embedding all optional fields in a single struct – this avoids
heavy custom JSON marshalling logic while remaining spec‑compliant.

NOTE: As per A2A spec, exactly ONE of Text, File, or Data should be populated
according to the Type field. This is not enforced at the struct level, but
applications should ensure this constraint is respected when creating Parts.
*/
type Part struct {
	Type PartType `json:"type"`

	// Exactly one of the following should be populated depending on Type.
	Text string         `json:"text,omitempty"`
	File *FilePart      `json:"file,omitempty"`
	Data map[string]any `json:"data,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty"`
}

// Validate checks if the Part is valid according to the A2A spec.
// Returns an error if it does not follow the discriminated union pattern.
func (p *Part) Validate() error {
	// Count how many fields are populated
	fieldsPopulated := 0

	if p.Text != "" {
		fieldsPopulated++
	}
	if p.File != nil {
		fieldsPopulated++
	}
	if p.Data != nil && len(p.Data) > 0 {
		fieldsPopulated++
	}

	// Check the correct field is populated based on Type
	switch p.Type {
	case PartTypeText:
		if p.Text == "" {
			return fmt.Errorf("text part has empty text field")
		}
	case PartTypeFile:
		if p.File == nil {
			return fmt.Errorf("file part has nil file field")
		}
	case PartTypeData:
		if p.Data == nil || len(p.Data) == 0 {
			return fmt.Errorf("data part has empty data field")
		}
	default:
		return fmt.Errorf("unknown part type: %s", p.Type)
	}

	// Check only one field is populated
	if fieldsPopulated != 1 {
		return fmt.Errorf("part should have exactly one of text, file, or data populated, found %d", fieldsPopulated)
	}

	// If it's a file part, validate that too
	if p.Type == PartTypeFile && p.File != nil {
		return p.File.Validate()
	}

	return nil
}

// PartType is the discriminator for a Part union.
type PartType string

const (
	PartTypeText PartType = "text"
	PartTypeFile PartType = "file"
	PartTypeData PartType = "data"
)

type FilePart struct {
	Name     *string `json:"name,omitempty"`
	MimeType *string `json:"mimeType,omitempty"`

	// One‑of: bytes OR uri.  The struct allows both, but the producer should
	// set only one as per the spec.
	Bytes string `json:"bytes,omitempty"` // base‑64 encoded
	URI   string `json:"uri,omitempty"`
}

/*
Validate checks if the FilePart is valid according to the A2A spec.
Returns an error if it violates the "oneof" constraint (bytes XOR uri).
*/
func (fp *FilePart) Validate() error {
	// Either bytes or uri must be set, but not both
	if fp.Bytes != "" && fp.URI != "" {
		return fmt.Errorf("file part cannot have both bytes and uri fields set")
	}

	// At least one of bytes or uri must be set
	if fp.Bytes == "" && fp.URI == "" {
		return fmt.Errorf("file part must have either bytes or uri field set")
	}

	return nil
}

type PushNotificationConfig struct {
	URL            string              `json:"url"`
	Token          *string             `json:"token,omitempty"`
	Authentication *AuthenticationInfo `json:"authentication,omitempty"`
}

type AuthenticationInfo struct {
	Schemes     []string `json:"schemes"`
	Credentials *string  `json:"credentials,omitempty"`
}

type TaskPushNotificationConfig struct {
	ID                     string                 `json:"id"`
	PushNotificationConfig PushNotificationConfig `json:"pushNotificationConfig"`
}
