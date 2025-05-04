package a2a

import "github.com/theapemachine/a2a-go/pkg/jsonrpc"

/*
Artifact is the output of a task.
*/
type Artifact struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Parts       []Part         `json:"parts"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Index       int            `json:"index,omitempty"`
	Append      *bool          `json:"append,omitempty"`
	LastChunk   *bool          `json:"lastChunk,omitempty"`
}

func NewFileArtifact(name string, mimeType string, data string) Artifact {
	return Artifact{
		Name: &name,
		Parts: []Part{
			{
				Type: PartTypeFile,
				File: &FilePart{
					MimeType: &mimeType,
					Data:     data,
				},
			},
		},
	}
}

type ArtifactResult struct {
	ID       string   `json:"id"`
	Artifact Artifact `json:"artifact"`
}

func NewArtifactResult(id string, parts ...Part) jsonrpc.Response {
	return jsonrpc.Response{
		Result: ArtifactResult{
			ID:       id,
			Artifact: Artifact{Parts: parts},
		},
	}
}
