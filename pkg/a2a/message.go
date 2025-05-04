package a2a

import "strings"

/*
Message represents all non‑artifact communication between client & agent.
*/
type Message struct {
	Role     string         `json:"role"` // "user" or "agent"
	Parts    []Part         `json:"parts"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func NewTextMessage(role string, text string) *Message {
	return &Message{
		Role: role,
		Parts: []Part{
			{Type: PartTypeText, Text: text},
		},
	}
}

func NewFileMessage(role string, file *FilePart) *Message {
	return &Message{
		Role: role,
		Parts: []Part{
			{Type: PartTypeFile, File: file},
		},
	}
}

func NewDataMessage(role string, data map[string]any) *Message {
	return &Message{
		Role: role,
		Parts: []Part{
			{Type: PartTypeData, Data: data},
		},
	}
}

func (msg *Message) String() string {
	var sb strings.Builder

	for _, part := range msg.Parts {
		sb.WriteString(part.Text)
	}

	return sb.String()
}
