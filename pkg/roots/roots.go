package roots

import "time"

// Root identifies a “namespace” or top‑level collection within an MCP server –
// e.g. file:///, agent:// or data://project‑123.
type Root struct {
    ID          string    `json:"id"`
    URI         string    `json:"uri"`
    Name        string    `json:"name"`
    Description string    `json:"description,omitempty"`
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
    Metadata    any       `json:"metadata,omitempty"`
}
