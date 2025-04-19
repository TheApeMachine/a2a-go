package sampling

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Manager interface {
	CreateMessage(ctx context.Context, content string, opts SamplingOptions) (*SamplingResult, error)
	StreamMessage(ctx context.Context, content string, opts SamplingOptions) (<-chan *SamplingResult, error)
	GetModelPreferences(ctx context.Context) (*ModelPreferences, error)
	UpdateModelPreferences(ctx context.Context, prefs ModelPreferences) error
}

// DefaultManager is a stub that simply echoes content back; useful for tests.
type DefaultManager struct {
	mu           sync.RWMutex
	defaultPrefs ModelPreferences
}

func NewDefaultManager() *DefaultManager {
	return &DefaultManager{
		defaultPrefs: ModelPreferences{Temperature: 0.7, MaxTokens: 2048, TopP: 1},
	}
}

func isZeroPrefs(p ModelPreferences) bool {
    return p.MaxTokens == 0 && p.Temperature == 0 && p.TopP == 0 && p.FrequencyPenalty == 0 && p.PresencePenalty == 0 && len(p.Stop) == 0
}

func (m *DefaultManager) CreateMessage(ctx context.Context, content string, opts SamplingOptions) (*SamplingResult, error) {
    if isZeroPrefs(opts.ModelPreferences) {
        opts.ModelPreferences = m.defaultPrefs
    }
	start := time.Now()
	msg := Message{ID: uuid.NewString(), Role: "assistant", Content: content, CreatedAt: time.Now()}
	usage := Usage{PromptTokens: len(content) / 4, CompletionTokens: len(content) / 4, TotalTokens: len(content) / 2}
	return &SamplingResult{Message: msg, Usage: usage, Duration: time.Since(start).Seconds()}, nil
}

func (m *DefaultManager) StreamMessage(ctx context.Context, content string, opts SamplingOptions) (<-chan *SamplingResult, error) {
    if isZeroPrefs(opts.ModelPreferences) {
        opts.ModelPreferences = m.defaultPrefs
    }

    ch := make(chan *SamplingResult)
	go func() {
		defer close(ch)
		start := time.Now()
		id := uuid.NewString()
		for _, r := range content {
			select {
			case <-ctx.Done():
				return
			default:
				msg := Message{ID: id, Role: "assistant", Content: string(r), CreatedAt: time.Now()}
				ch <- &SamplingResult{Message: msg, Usage: Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}, Duration: time.Since(start).Seconds()}
				time.Sleep(30 * time.Millisecond)
			}
		}
	}()
	return ch, nil
}

func (m *DefaultManager) GetModelPreferences(ctx context.Context) (*ModelPreferences, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	prefs := m.defaultPrefs
	return &prefs, nil
}

func (m *DefaultManager) UpdateModelPreferences(ctx context.Context, prefs ModelPreferences) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultPrefs = prefs
	return nil
}
