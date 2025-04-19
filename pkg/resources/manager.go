package resources

import (
	"context"
	"fmt"
	"sync"
)

// DefaultManager is an in‑memory ResourceManager good enough for demos.
type DefaultManager struct {
	resources           []Resource
	templates           []ResourceTemplate
	subManager          *SubscriptionManager
	activeSubscriptions map[string][]*Subscription
	mu                  sync.RWMutex
}

func NewDefaultManager() *DefaultManager {
	m := &DefaultManager{
		subManager:          NewSubscriptionManager(),
		activeSubscriptions: map[string][]*Subscription{},
	}

	// Inject a couple of sample resources so list isn’t empty.
	m.AddResource(Resource{URI: "file:///hello.txt", Name: "Hello", MimeType: "text/plain", Type: TextResource})
	m.AddTemplate(ResourceTemplate{URITemplate: "file:///docs/{version}/{page}", Name: "Docs", MimeType: "text/markdown", Type: TextResource})
	return m
}

// List returns static + template resources.
func (m *DefaultManager) List(ctx context.Context) ([]Resource, []ResourceTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rs := append([]Resource(nil), m.resources...)
	ts := append([]ResourceTemplate(nil), m.templates...)
	return rs, ts, nil
}

// Read returns placeholder data right now – future work could fetch from disk
// or DB.
func (m *DefaultManager) Read(ctx context.Context, uri string) ([]ResourceContent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Try static first.
	if r := m.findResource(uri); r != nil {
		txt := fmt.Sprintf("Content of %s", r.Name)
		return []ResourceContent{{URI: uri, MimeType: r.MimeType, Text: txt}}, nil
	}

	// Attempt template match.
	if tmpl, vars := m.matchTemplate(uri); tmpl != nil {
		txt := "Template Variables:\n"
		for k, v := range vars {
			txt += fmt.Sprintf("- %s: %s\n", k, v)
		}
		return []ResourceContent{{URI: uri, MimeType: tmpl.MimeType, Text: txt}}, nil
	}

	return nil, fmt.Errorf("resource not found: %s", uri)
}

func (m *DefaultManager) Subscribe(ctx context.Context, uri string) error {
	sub, err := m.subManager.Subscribe(ctx, uri)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.activeSubscriptions[uri] = append(m.activeSubscriptions[uri], sub)
	m.mu.Unlock()
	return nil
}

func (m *DefaultManager) Unsubscribe(ctx context.Context, uri string) error {
	m.mu.Lock()
	subs := m.activeSubscriptions[uri]
	delete(m.activeSubscriptions, uri)
	m.mu.Unlock()
	for _, s := range subs {
		m.subManager.Unsubscribe(uri, s)
	}
	return nil
}

// Helper registration methods ------------------------------------------------

func (m *DefaultManager) AddResource(r Resource) {
	m.mu.Lock()
	m.resources = append(m.resources, r)
	m.mu.Unlock()
}

func (m *DefaultManager) AddTemplate(t ResourceTemplate) {
	m.mu.Lock()
	m.templates = append(m.templates, t)
	m.mu.Unlock()
}

func (m *DefaultManager) findResource(uri string) *Resource {
	for _, r := range m.resources {
		if r.URI == uri {
			return &r
		}
	}
	return nil
}

func (m *DefaultManager) matchTemplate(uri string) (*ResourceTemplate, map[string]string) {
	for _, t := range m.templates {
		vars, err := matchTemplate(t.URITemplate, uri)
		if err == nil {
			return &t, vars
		}
	}
	return nil, nil
}
