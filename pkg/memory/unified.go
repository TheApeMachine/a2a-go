package memory

import (
	"context"
	"fmt"
	"sync"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/theapemachine/a2a-go/memory"
)

// UnifiedMemoryStore implements the UnifiedStore interface by combining
// vector and graph stores
type UnifiedMemoryStore struct {
	VectorStore      VectorStore
	GraphStore       GraphStore
	EmbeddingService EmbeddingService
}

// NewUnifiedMemoryStore creates a new unified store with the provided backends
func NewUnifiedMemoryStore(vectorStore VectorStore, graphStore GraphStore, embeddingService EmbeddingService) *UnifiedMemoryStore {
	return &UnifiedMemoryStore{
		VectorStore:      vectorStore,
		GraphStore:       graphStore,
		EmbeddingService: embeddingService,
	}
}

// NewUnifiedStore creates a new unified store with the provided backends
func NewUnifiedStore(embeddingService EmbeddingService, vectorStore VectorStore, graphStore GraphStore) *UnifiedMemoryStore {
	return &UnifiedMemoryStore{
		VectorStore:      vectorStore,
		GraphStore:       graphStore,
		EmbeddingService: embeddingService,
	}
}

// StoreMemory stores a memory in the appropriate store(s) based on the provided type
func (s *UnifiedMemoryStore) StoreMemory(ctx context.Context, content string, metadata map[string]any, storeType string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("content cannot be empty")
	}

	// Generate an embedding
	embedding, err := s.EmbeddingService.GenerateEmbedding(ctx, content)
	if err != nil {
		return "", fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Create memory object
	memory := Memory{
		Content:   content,
		Metadata:  metadata,
		Embedding: embedding,
		Type:      storeType,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	var id string

	// Store in appropriate backend based on type
	switch storeType {
	case "vector":
		// Store only in vector store
		id, err = s.VectorStore.StoreMemory(ctx, memory)
		if err != nil {
			return "", fmt.Errorf("failed to store in vector store: %w", err)
		}
	case "graph":
		// Store only in graph store
		id, err = s.GraphStore.StoreMemory(ctx, memory)
		if err != nil {
			return "", fmt.Errorf("failed to store in graph store: %w", err)
		}
	default:
		// Store in both stores for unified access
		vectorID, err := s.VectorStore.StoreMemory(ctx, memory)
		if err != nil {
			return "", fmt.Errorf("failed to store in vector store: %w", err)
		}

		// Set the ID for graph store to match
		memory.ID = vectorID

		_, err = s.GraphStore.StoreMemory(ctx, memory)
		if err != nil {
			return "", fmt.Errorf("failed to store in graph store: %w", err)
		}

		id = vectorID
	}

	return id, nil
}

// GetMemory retrieves a memory by ID from the appropriate store
func (s *UnifiedMemoryStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	// Try to get from vector store first
	vectorMem, vectorErr := s.VectorStore.GetMemory(ctx, id)
	if vectorErr == nil {
		return vectorMem, nil
	}

	// If not found in vector store, try graph store
	graphMem, graphErr := s.GraphStore.GetMemory(ctx, id)
	if graphErr == nil {
		return graphMem, nil
	}

	// If not found in either store, return an error
	return Memory{}, fmt.Errorf("memory not found: %s (vector error: %v, graph error: %v)", id, vectorErr, graphErr)
}

// CreateRelation creates a relationship between two memories in the graph store
func (s *UnifiedMemoryStore) CreateRelation(ctx context.Context, source, target, relationType string, properties map[string]any) error {
	relation := Relation{
		Source:     source,
		Target:     target,
		Type:       relationType,
		Properties: properties,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	return s.GraphStore.CreateRelation(ctx, relation)
}

// SearchSimilar searches for memories based on semantic similarity
func (s *UnifiedMemoryStore) SearchSimilar(ctx context.Context, query string, params SearchParams) ([]Memory, error) {
	// Generate embedding for the query
	embedding, err := s.EmbeddingService.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Search in vector store
	return s.VectorStore.SearchSimilar(ctx, embedding, params)
}

// FindRelated finds memories related to a given memory
func (s *UnifiedMemoryStore) FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error) {
	return s.GraphStore.FindRelated(ctx, id, relationTypes, limit)
}

// DeleteMemory deletes a memory from all stores
func (s *UnifiedMemoryStore) DeleteMemory(ctx context.Context, id string) error {
	// Delete from vector store
	vectorErr := s.VectorStore.DeleteMemory(ctx, id)

	// Delete from graph store
	graphErr := s.GraphStore.DeleteMemory(ctx, id)

	// If both operations failed, return an error
	if vectorErr != nil && graphErr != nil {
		return fmt.Errorf("failed to delete memory: vector error: %v, graph error: %v", vectorErr, graphErr)
	}

	return nil
}

// InMemoryUnifiedStore provides a simple in-memory implementation of UnifiedStore
// for testing and demonstrations
type InMemoryUnifiedStore struct {
	store            *memory.Store
	embeddingService EmbeddingService
}

// NewInMemoryUnifiedStore creates a new in-memory unified store
func NewInMemoryUnifiedStore() *InMemoryUnifiedStore {
	return &InMemoryUnifiedStore{
		store:            memory.New(),
		embeddingService: NewMockEmbeddingService(),
	}
}

// NewInMemoryVectorStore creates a new in-memory vector store for testing
func NewInMemoryVectorStore() VectorStore {
	return &MockVectorStore{
		memories: make(map[string]Memory),
	}
}

// NewInMemoryGraphStore creates a new in-memory graph store for testing
func NewInMemoryGraphStore() GraphStore {
	return &MockGraphStore{
		memories:  make(map[string]Memory),
		relations: make([]Relation, 0),
	}
}

// MockVectorStore implements a simple in-memory vector store for testing
type MockVectorStore struct {
	memories map[string]Memory
	mu       sync.RWMutex
}

// StoreMemory stores a memory in the vector database
func (s *MockVectorStore) StoreMemory(ctx context.Context, memory Memory) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if memory.ID == "" {
		memory.ID = uuid.NewString()
	}
	
	// Set timestamps if not set
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = time.Now().UTC()
	}
	if memory.UpdatedAt.IsZero() {
		memory.UpdatedAt = time.Now().UTC()
	}
	
	s.memories[memory.ID] = memory
	return memory.ID, nil
}

// StoreMemories stores multiple memories in a batch
func (s *MockVectorStore) StoreMemories(ctx context.Context, memories []Memory) error {
	for _, memory := range memories {
		_, err := s.StoreMemory(ctx, memory)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetMemory gets a memory by ID
func (s *MockVectorStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	memory, exists := s.memories[id]
	if !exists {
		return Memory{}, fmt.Errorf("memory not found: %s", id)
	}
	
	return memory, nil
}

// SearchSimilar searches for semantically similar memories
func (s *MockVectorStore) SearchSimilar(ctx context.Context, embedding []float32, params SearchParams) ([]Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var results []Memory
	
	// Since this is a mock, we'll do a simple text search instead of vector similarity
	query := strings.ToLower(params.Query)
	
	for _, memory := range s.memories {
		// Apply type filter if specified
		if len(params.Types) > 0 {
			typeMatch := false
			for _, t := range params.Types {
				if memory.Type == t {
					typeMatch = true
					break
				}
			}
			if !typeMatch {
				continue
			}
		}
		
		// Apply metadata filters if specified
		if len(params.Filters) > 0 {
			match := true
			for _, filter := range params.Filters {
				value, exists := memory.Metadata[filter.Field]
				if !exists {
					match = false
					break
				}
				
				switch filter.Operator {
				case "eq":
					if value != filter.Value {
						match = false
					}
				}
				
				if !match {
					break
				}
			}
			
			if !match {
				continue
			}
		}
		
		// Simple content matching
		if strings.Contains(strings.ToLower(memory.Content), query) {
			results = append(results, memory)
		}
	}
	
	// Apply limit
	if params.Limit > 0 && len(results) > params.Limit {
		results = results[:params.Limit]
	}
	
	return results, nil
}

// DeleteMemory deletes a memory
func (s *MockVectorStore) DeleteMemory(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.memories[id]; !exists {
		return fmt.Errorf("memory not found: %s", id)
	}
	
	delete(s.memories, id)
	return nil
}

// Ping checks connection to the store
func (s *MockVectorStore) Ping(ctx context.Context) error {
	return nil
}

// MockGraphStore implements a simple in-memory graph store for testing
type MockGraphStore struct {
	memories  map[string]Memory
	relations []Relation
	mu        sync.RWMutex
}

// StoreMemory stores a memory as a node in the graph
func (s *MockGraphStore) StoreMemory(ctx context.Context, memory Memory) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if memory.ID == "" {
		memory.ID = uuid.NewString()
	}
	
	// Set timestamps if not set
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = time.Now().UTC()
	}
	if memory.UpdatedAt.IsZero() {
		memory.UpdatedAt = time.Now().UTC()
	}
	
	s.memories[memory.ID] = memory
	return memory.ID, nil
}

// CreateRelation creates a relationship between two memories
func (s *MockGraphStore) CreateRelation(ctx context.Context, relation Relation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Verify both memories exist
	if _, exists := s.memories[relation.Source]; !exists {
		return fmt.Errorf("source memory not found: %s", relation.Source)
	}
	
	if _, exists := s.memories[relation.Target]; !exists {
		return fmt.Errorf("target memory not found: %s", relation.Target)
	}
	
	// Set timestamps if not set
	if relation.CreatedAt.IsZero() {
		relation.CreatedAt = time.Now().UTC()
	}
	if relation.UpdatedAt.IsZero() {
		relation.UpdatedAt = time.Now().UTC()
	}
	
	// Store the relation
	s.relations = append(s.relations, relation)
	
	return nil
}

// GetMemory gets a memory by ID
func (s *MockGraphStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	memory, exists := s.memories[id]
	if !exists {
		return Memory{}, fmt.Errorf("memory not found: %s", id)
	}
	
	return memory, nil
}

// FindRelated finds related memories
func (s *MockGraphStore) FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var results []Memory
	relatedIDs := make(map[string]bool)
	
	for _, relation := range s.relations {
		if relation.Source == id {
			// Apply relation type filter if specified
			if len(relationTypes) > 0 {
				typeMatch := false
				for _, t := range relationTypes {
					if relation.Type == t {
						typeMatch = true
						break
					}
				}
				if !typeMatch {
					continue
				}
			}
			
			// Add target to results if not already added
			if !relatedIDs[relation.Target] {
				if memory, exists := s.memories[relation.Target]; exists {
					results = append(results, memory)
					relatedIDs[relation.Target] = true
				}
			}
		}
	}
	
	// Apply limit
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	
	return results, nil
}

// QueryGraph performs a graph query to find connected memories
func (s *MockGraphStore) QueryGraph(ctx context.Context, query string, params map[string]any) ([]Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// This is a very simple implementation for testing
	// In a real implementation, this would translate the query to a Cypher query
	var results []Memory
	
	for _, memory := range s.memories {
		if strings.Contains(strings.ToLower(memory.Content), strings.ToLower(query)) {
			results = append(results, memory)
		}
	}
	
	return results, nil
}

// DeleteMemory deletes a memory node
func (s *MockGraphStore) DeleteMemory(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.memories[id]; !exists {
		return fmt.Errorf("memory not found: %s", id)
	}
	
	// Delete the memory
	delete(s.memories, id)
	
	// Remove all relations involving this memory
	var newRelations []Relation
	for _, relation := range s.relations {
		if relation.Source != id && relation.Target != id {
			newRelations = append(newRelations, relation)
		}
	}
	s.relations = newRelations
	
	return nil
}

// DeleteRelation deletes a relation
func (s *MockGraphStore) DeleteRelation(ctx context.Context, source, target, relationType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Find and remove matching relations
	var newRelations []Relation
	found := false
	
	for _, relation := range s.relations {
		if relation.Source == source && relation.Target == target && relation.Type == relationType {
			found = true
			continue
		}
		newRelations = append(newRelations, relation)
	}
	
	if !found {
		return fmt.Errorf("relation not found: %s -> %s (%s)", source, target, relationType)
	}
	
	s.relations = newRelations
	return nil
}

// Ping checks connection to the store
func (s *MockGraphStore) Ping(ctx context.Context) error {
	return nil
}

// StoreMemory stores a memory in the in-memory store
func (s *InMemoryUnifiedStore) StoreMemory(ctx context.Context, content string, metadata map[string]any, storeType string) (string, error) {
	return s.store.Put(storeType, content, metadata), nil
}

// GetMemory retrieves a memory by ID
func (s *InMemoryUnifiedStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	doc, found := s.store.Get(id)
	if !found {
		return Memory{}, fmt.Errorf("memory not found: %s", id)
	}

	memory := Memory{
		ID:        doc.ID,
		Content:   doc.Content,
		Metadata:  doc.Metadata,
		CreatedAt: doc.Created,
		UpdatedAt: doc.Created,
	}

	return memory, nil
}

// CreateRelation creates a relationship between two memories
// (simplified for in-memory implementation)
func (s *InMemoryUnifiedStore) CreateRelation(ctx context.Context, source, target, relationType string, properties map[string]any) error {
	// For the in-memory implementation, we just add the relation to metadata
	srcDoc, found := s.store.Get(source)
	if !found {
		return fmt.Errorf("source memory not found: %s", source)
	}

	_, found = s.store.Get(target)
	if !found {
		return fmt.Errorf("target memory not found: %s", target)
	}

	// Create or update relations in metadata
	if srcDoc.Metadata == nil {
		srcDoc.Metadata = make(map[string]any)
	}

	relations, ok := srcDoc.Metadata["relations"].([]map[string]any)
	if !ok {
		relations = make([]map[string]any, 0)
	}

	relation := map[string]any{
		"target":     target,
		"type":       relationType,
		"properties": properties,
		"createdAt":  time.Now().UTC(),
	}

	relations = append(relations, relation)
	srcDoc.Metadata["relations"] = relations

	// Update the document
	s.store.Put("graph", srcDoc.Content, srcDoc.Metadata)

	return nil
}

// SearchSimilar performs a naive text search in the in-memory store
func (s *InMemoryUnifiedStore) SearchSimilar(ctx context.Context, query string, params SearchParams) ([]Memory, error) {
	// Simple text search using the in-memory store's Search method
	ids := s.store.Search(query, "", params.Limit)

	// Convert to Memory objects
	memories := make([]Memory, 0, len(ids))
	for _, id := range ids {
		doc, found := s.store.Get(id)
		if found {
			memories = append(memories, Memory{
				ID:        doc.ID,
				Content:   doc.Content,
				Metadata:  doc.Metadata,
				CreatedAt: doc.Created,
				UpdatedAt: doc.Created,
			})
		}
	}

	return memories, nil
}

// FindRelated finds related memories using in-memory metadata
func (s *InMemoryUnifiedStore) FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error) {
	doc, found := s.store.Get(id)
	if !found {
		return nil, fmt.Errorf("memory not found: %s", id)
	}

	// Get relations from metadata
	if doc.Metadata == nil {
		return []Memory{}, nil
	}

	relations, ok := doc.Metadata["relations"].([]map[string]any)
	if !ok {
		return []Memory{}, nil
	}

	// Filter by relation types if provided
	var matchingRelations []map[string]any
	if len(relationTypes) > 0 {
		for _, relation := range relations {
			relType, ok := relation["type"].(string)
			if !ok {
				continue
			}

			for _, targetType := range relationTypes {
				if relType == targetType {
					matchingRelations = append(matchingRelations, relation)
					break
				}
			}
		}
	} else {
		matchingRelations = relations
	}

	// Apply limit
	if limit > 0 && len(matchingRelations) > limit {
		matchingRelations = matchingRelations[:limit]
	}

	// Get related memories
	memories := make([]Memory, 0, len(matchingRelations))
	for _, relation := range matchingRelations {
		targetID, ok := relation["target"].(string)
		if !ok {
			continue
		}

		targetDoc, found := s.store.Get(targetID)
		if found {
			memories = append(memories, Memory{
				ID:        targetDoc.ID,
				Content:   targetDoc.Content,
				Metadata:  targetDoc.Metadata,
				CreatedAt: targetDoc.Created,
				UpdatedAt: targetDoc.Created,
			})
		}
	}

	return memories, nil
}

// DeleteMemory removes a memory from the in-memory store
func (s *InMemoryUnifiedStore) DeleteMemory(ctx context.Context, id string) error {
	// The in-memory store doesn't provide a delete method,
	// so we'll just overwrite with an empty content
	if _, found := s.store.Get(id); !found {
		return fmt.Errorf("memory not found: %s", id)
	}

	// Mark as deleted in metadata
	s.store.Put("", "", map[string]any{
		"deleted":   true,
		"deletedAt": time.Now().UTC(),
	})

	return nil
}
