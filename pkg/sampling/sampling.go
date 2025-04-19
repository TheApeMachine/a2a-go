package sampling

import "time"

// Data types mirror those defined in the MCP spec so we can translate 1‑to‑1.

type ModelPreferences struct {
    Temperature      float64  `json:"temperature"`
    MaxTokens        int      `json:"maxTokens"`
    TopP             float64  `json:"topP"`
    FrequencyPenalty float64  `json:"frequencyPenalty"`
    PresencePenalty  float64  `json:"presencePenalty"`
    Stop             []string `json:"stop,omitempty"`
}

type Message struct {
    ID        string    `json:"id"`
    Role      string    `json:"role"`
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"createdAt"`
    Metadata  any       `json:"metadata,omitempty"`
}

type Context struct {
    Messages []Message `json:"messages"`
    Files    []string  `json:"files,omitempty"`
    Data     any       `json:"data,omitempty"`
}

type SamplingOptions struct {
    ModelPreferences ModelPreferences `json:"modelPreferences"`
    Context          *Context         `json:"context,omitempty"`
    Stream           bool             `json:"stream"`
}

type Usage struct {
    PromptTokens     int `json:"promptTokens"`
    CompletionTokens int `json:"completionTokens"`
    TotalTokens      int `json:"totalTokens"`
}

type SamplingResult struct {
    Message  Message `json:"message"`
    Usage    Usage   `json:"usage"`
    Duration float64 `json:"duration"`
    Metadata any     `json:"metadata,omitempty"`
}
