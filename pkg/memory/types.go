package memory

// Memory represents a single unit of stored knowledge.
type Memory struct {
	ID        string
	Content   string
	Metadata  map[string]any
	Type      string
	Embedding []float32
}

// Relation connects two memories in the graph store.
type Relation struct {
	SourceID   string
	TargetID   string
	Type       string
	Properties map[string]any
}

// Filter represents an optional search filter.
type Filter struct {
	Field    string
	Operator string
	Value    any
}

// SearchParams specify vector search options.
type SearchParams struct {
	Limit   int
	Types   []string
	Filters []Filter
}
