package resources

import (
    "context"
    "sync"
    "time"
)

type Subscription struct {
    URI     string
    Channel chan ResourceContent
    ctx     context.Context
    cancel  context.CancelFunc
    created time.Time
}

// SubscriptionManager is a tiny pub/sub helper.
type SubscriptionManager struct {
    mu            sync.RWMutex
    subscriptions map[string][]*Subscription // keyed by uri
}

func NewSubscriptionManager() *SubscriptionManager {
    return &SubscriptionManager{subscriptions: map[string][]*Subscription{}}
}

func (m *SubscriptionManager) Subscribe(ctx context.Context, uri string) (*Subscription, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    subCtx, cancel := context.WithCancel(ctx)
    s := &Subscription{URI: uri, Channel: make(chan ResourceContent, 10), ctx: subCtx, cancel: cancel, created: time.Now()}
    m.subscriptions[uri] = append(m.subscriptions[uri], s)

    go func() { <-subCtx.Done(); m.Unsubscribe(uri, s) }()
    return s, nil
}

func (m *SubscriptionManager) Unsubscribe(uri string, s *Subscription) {
    m.mu.Lock()
    defer m.mu.Unlock()
    subs := m.subscriptions[uri]
    for i, sub := range subs {
        if sub == s {
            close(sub.Channel)
            m.subscriptions[uri] = append(subs[:i], subs[i+1:]...)
            break
        }
    }
    if len(m.subscriptions[uri]) == 0 {
        delete(m.subscriptions, uri)
    }
}

func (m *SubscriptionManager) Notify(uri string, c ResourceContent) {
    m.mu.RLock()
    subs := m.subscriptions[uri]
    m.mu.RUnlock()
    for _, s := range subs {
        select {
        case s.Channel <- c:
        default:
        }
    }
}
