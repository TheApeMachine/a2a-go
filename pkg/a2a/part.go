package a2a

import "encoding/base64"

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
	Data     string  `json:"bytes,omitempty"`
	URI      string  `json:"uri,omitempty"`
}

func NewTextPart(text string) Part {
	return Part{
		Type: PartTypeText,
		Text: text,
	}
}

func NewFilePart(name string, mimeType string, data []byte) Part {
	return Part{
		Type: PartTypeFile,
		File: &FilePart{
			Name:     &name,
			MimeType: &mimeType,
			Data:     base64.StdEncoding.EncodeToString(data),
		},
	}
}
