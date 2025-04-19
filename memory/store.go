// Package memory provides a **very small in‑memory** implementation for a
// unified long‑term memory façade.  The goal is to give the a2a‑go SDK a
// working, dependency‑free backend so the built‑in “memory_*” MCP tools and
// examples compile and run out‑of‑the‑box.  It purposefully keeps the data
// structures minimal and the matching logic naive – we only need something
// good enough for unit tests and demos.  Production deployments should replace
// this with a real vector database (e.g. Qdrant) and a graph database (e.g.
// Neo4j).

package memory

import (
    "strings"
    "sync"
    "time"

    "github.com/google/uuid"
)

// Document represents a single item stored in memory.  It resembles the
// qdrant.Document structure we intend to use in the fully‑featured
// implementation but is deliberately simplified to avoid external
// dependencies for this stub.
type Document struct {
    ID       string                 `json:"id"`
    Content  string                 `json:"content"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
    Created  time.Time              `json:"created_at"`
}

// Store is a unified façade hiding the actual backend (vector store vs graph
// store) behind two **very** small in‑memory maps.  The public API is
// intentionally tiny – just what we need for the memory_* MCP tools.
type Store struct {
    mu         sync.RWMutex
    vectorDocs map[string]Document // keyed by ID
    graphDocs  map[string]Document // ditto
}

// New returns an empty Store instance.
func New() *Store {
    return &Store{
        vectorDocs: make(map[string]Document),
        graphDocs:  make(map[string]Document),
    }
}

// Put stores the given content + metadata.  The caller chooses which logical
// backend to use via the `backend` parameter ("vector" or "graph").  The
// generated document ID is returned.
func (s *Store) Put(backend, content string, meta map[string]interface{}) string {
    if meta == nil {
        meta = map[string]interface{}{}
    }
    doc := Document{
        ID:       uuid.NewString(),
        Content:  content,
        Metadata: meta,
        Created:  time.Now().UTC(),
    }

    s.mu.Lock()
    switch strings.ToLower(backend) {
    case "graph":
        s.graphDocs[doc.ID] = doc
    default:
        s.vectorDocs[doc.ID] = doc
    }
    s.mu.Unlock()
    return doc.ID
}

// Get returns the document with the given ID if present.
func (s *Store) Get(id string) (Document, bool) {
    s.mu.RLock()
    if d, ok := s.vectorDocs[id]; ok {
        s.mu.RUnlock()
        return d, true
    }
    if d, ok := s.graphDocs[id]; ok {
        s.mu.RUnlock()
        return d, true
    }
    s.mu.RUnlock()
    return Document{}, false
}

// Search performs a **very naive** substring search across both backends (or a
// single backend if provided).  It returns the IDs of matching documents.
//
// The behaviour is deterministic, case‑insensitive, and stops scanning after
// `limit` hits if limit > 0.
func (s *Store) Search(query, backend string, limit int) []string {
    q := strings.ToLower(query)
    var ids []string

    match := func(docs map[string]Document) bool {
        for id, d := range docs {
            if strings.Contains(strings.ToLower(d.Content), q) {
                ids = append(ids, id)
                if limit > 0 && len(ids) >= limit {
                    return true // reached limit
                }
            }
        }
        return false
    }

    s.mu.RLock()
    switch strings.ToLower(backend) {
    case "vector":
        match(s.vectorDocs)
    case "graph":
        match(s.graphDocs)
    default:
        if stop := match(s.vectorDocs); stop {
            s.mu.RUnlock()
            return ids
        }
        match(s.graphDocs)
    }
    s.mu.RUnlock()
    return ids
}
