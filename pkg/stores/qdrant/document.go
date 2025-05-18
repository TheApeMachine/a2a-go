package qdrant

type Document struct {
	ID       string
	Content  string
	Metadata map[string]any
}

func NewDocument(id, content string, metadata map[string]any) *Document {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["content"] = content
	return &Document{
		ID:       id,
		Content:  content,
		Metadata: metadata,
	}
}
