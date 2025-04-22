package types

import (
	"context"
	"fmt"

	"github.com/theapemachine/a2a-go/pkg/errors"
)

type IdentifiableTaskManager interface {
	TaskManager
	Card() *AgentCard
}

// TaskManager is plugged into an A2AServer.  Each method should do its own
// validation and return a *errors.RpcError value if the request is invalid or cannot
// be fulfilled.
type TaskManager interface {
	SendTask(context.Context, Task) (Task, *errors.RpcError)
	GetTask(context.Context, string, int) (Task, *errors.RpcError)
	CancelTask(context.Context, string) (Task, *errors.RpcError)
	StreamTask(context.Context, Task) (Task, *errors.RpcError)
	ResubscribeTask(context.Context, string, int) (<-chan any, *errors.RpcError)
	SetPushNotification(context.Context, TaskPushNotificationConfig) (TaskPushNotificationConfig, *errors.RpcError)
	GetPushNotification(context.Context, string) (TaskPushNotificationConfig, *errors.RpcError)
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
