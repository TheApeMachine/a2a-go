package qdrant

type Document struct {
	ID       string
	Content  string
	Metadata map[string]any
}

func NewDocument(id, content string, metadata map[string]any) *Document {
	return &Document{
		ID:       id,
		Content:  content,
		Metadata: metadata,
	}
}
