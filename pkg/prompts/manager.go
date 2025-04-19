package prompts

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/google/uuid"
)

// A handful of small error helpers so callers can inspect failures.

type ErrorPromptNotFound struct{ ID string }

func (e ErrorPromptNotFound) Error() string { return fmt.Sprintf("prompt not found: %s", e.ID) }

type ErrorStepNotFound struct{ ID string }

func (e ErrorStepNotFound) Error() string { return fmt.Sprintf("step not found: %s", e.ID) }

type ErrorInvalidPromptType struct {
    ID   string
    Type PromptType
}

func (e ErrorInvalidPromptType) Error() string {
    return fmt.Sprintf("invalid prompt type for prompt %s: %s", e.ID, e.Type)
}

// DefaultManager is an in‑memory implementation suitable for tests and small
// demos.  It is *not* thread‑safe for persistence across restarts but it does
// use a RWMutex so concurrent callers are fine.

type DefaultManager struct {
    prompts map[string]*Prompt
    steps   map[string][]*PromptStep // keyed by promptID
    mu      sync.RWMutex
}

func NewDefaultManager() *DefaultManager {
    m := &DefaultManager{
        prompts: make(map[string]*Prompt),
        steps:   make(map[string][]*PromptStep),
    }
    m.seed()
    return m
}

// Convenience helper seeds two demo prompts so the server is interesting out
// of the box.
func (m *DefaultManager) seed() {
    single := &Prompt{
        ID:          uuid.NewString(),
        Name:        "Greeting",
        Description: "A friendly greeting",
        Type:        SingleStepPrompt,
        Content:     "Hello – how can I help you today?",
        Version:     "1.0.0",
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    m.prompts[single.ID] = single

    multi := &Prompt{
        ID:          uuid.NewString(),
        Name:        "Customer‑Support",
        Description: "4‑step customer support flow",
        Type:        MultiStepPrompt,
        Content:     "Multi step support flow.",
        Version:     "1.0.0",
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    m.prompts[multi.ID] = multi

    steps := []*PromptStep{
        {ID: uuid.NewString(), PromptID: multi.ID, Name: "Greeting", Content: "Hello, thanks for contacting us – how can we help?", Order: 1},
        {ID: uuid.NewString(), PromptID: multi.ID, Name: "GatherInfo", Content: "Can you describe the issue in detail?", Order: 2},
        {ID: uuid.NewString(), PromptID: multi.ID, Name: "ProvideSolution", Content: "Here is what you can try ...", Order: 3},
        {ID: uuid.NewString(), PromptID: multi.ID, Name: "Closing", Content: "Is there anything else I can help you with?", Order: 4},
    }
    m.steps[multi.ID] = steps
}

// ----- PromptManager implementation -----

func (m *DefaultManager) List(ctx context.Context) ([]Prompt, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    out := make([]Prompt, 0, len(m.prompts))
    for _, p := range m.prompts {
        out = append(out, *p)
    }
    return out, nil
}

func (m *DefaultManager) Get(ctx context.Context, id string) (*Prompt, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    p, ok := m.prompts[id]
    if !ok {
        return nil, ErrorPromptNotFound{ID: id}
    }
    return p, nil
}

func (m *DefaultManager) GetSteps(ctx context.Context, promptID string) ([]PromptStep, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    pr, ok := m.prompts[promptID]
    if !ok {
        return nil, ErrorPromptNotFound{ID: promptID}
    }
    if pr.Type != MultiStepPrompt {
        return nil, ErrorInvalidPromptType{ID: promptID, Type: pr.Type}
    }
    arr := m.steps[promptID]
    out := make([]PromptStep, len(arr))
    for i, s := range arr {
        out[i] = *s
    }
    return out, nil
}

func (m *DefaultManager) Create(ctx context.Context, p Prompt) (*Prompt, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if p.ID == "" {
        p.ID = uuid.NewString()
    }
    now := time.Now()
    p.CreatedAt, p.UpdatedAt = now, now
    m.prompts[p.ID] = &p
    if p.Type == MultiStepPrompt {
        m.steps[p.ID] = []*PromptStep{}
    }
    return &p, nil
}

func (m *DefaultManager) Update(ctx context.Context, p Prompt) (*Prompt, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    old, ok := m.prompts[p.ID]
    if !ok {
        return nil, ErrorPromptNotFound{ID: p.ID}
    }
    p.CreatedAt = old.CreatedAt
    p.UpdatedAt = time.Now()
    m.prompts[p.ID] = &p
    return &p, nil
}

func (m *DefaultManager) Delete(ctx context.Context, id string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if _, ok := m.prompts[id]; !ok {
        return ErrorPromptNotFound{ID: id}
    }
    delete(m.prompts, id)
    delete(m.steps, id)
    return nil
}

func (m *DefaultManager) CreateStep(ctx context.Context, s PromptStep) (*PromptStep, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    p, ok := m.prompts[s.PromptID]
    if !ok {
        return nil, ErrorPromptNotFound{ID: s.PromptID}
    }
    if p.Type != MultiStepPrompt {
        return nil, ErrorInvalidPromptType{ID: p.ID, Type: p.Type}
    }
    if s.ID == "" {
        s.ID = uuid.NewString()
    }
    m.steps[s.PromptID] = append(m.steps[s.PromptID], &s)
    return &s, nil
}

func (m *DefaultManager) UpdateStep(ctx context.Context, s PromptStep) (*PromptStep, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    arr := m.steps[s.PromptID]
    for i, step := range arr {
        if step.ID == s.ID {
            arr[i] = &s
            return &s, nil
        }
    }
    return nil, ErrorStepNotFound{ID: s.ID}
}

func (m *DefaultManager) DeleteStep(ctx context.Context, stepID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    for pid, arr := range m.steps {
        for i, s := range arr {
            if s.ID == stepID {
                m.steps[pid] = append(arr[:i], arr[i+1:]...)
                return nil
            }
        }
    }
    return ErrorStepNotFound{ID: stepID}
}
