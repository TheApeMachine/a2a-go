package prompts

import (
    "context"
    "time"
)

// PromptType indicates single or multi‑step prompt.
type PromptType string

const (
    SingleStepPrompt PromptType = "single"
    MultiStepPrompt  PromptType = "multi"
)

// Prompt is a reusable piece of text (or recipe) that can be injected into an
// LLM interaction as a system or user instruction.
type Prompt struct {
    ID          string     `json:"id"`
    Name        string     `json:"name"`
    Description string     `json:"description"`
    Type        PromptType `json:"type"`
    Content     string     `json:"content"`
    Version     string     `json:"version"`
    CreatedAt   time.Time  `json:"createdAt"`
    UpdatedAt   time.Time  `json:"updatedAt"`
    Metadata    any        `json:"metadata,omitempty"`
}

// PromptStep belongs to a multi‑step prompt.
type PromptStep struct {
    ID          string `json:"id"`
    PromptID    string `json:"promptId"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Content     string `json:"content"`
    Order       int    `json:"order"`
    Metadata    any    `json:"metadata,omitempty"`
}

// PromptManager is the contract used by higher‑level components.
type PromptManager interface {
    List(ctx context.Context) ([]Prompt, error)
    Get(ctx context.Context, id string) (*Prompt, error)
    GetSteps(ctx context.Context, promptID string) ([]PromptStep, error)
    Create(ctx context.Context, prompt Prompt) (*Prompt, error)
    Update(ctx context.Context, prompt Prompt) (*Prompt, error)
    Delete(ctx context.Context, id string) error

    CreateStep(ctx context.Context, step PromptStep) (*PromptStep, error)
    UpdateStep(ctx context.Context, step PromptStep) (*PromptStep, error)
    DeleteStep(ctx context.Context, stepID string) error
}
