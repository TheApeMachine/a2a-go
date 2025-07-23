// Optimized UnifiedMemory implementation
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// UnifiedMemory implements the UnifiedStore interface with caching and batching.
type UnifiedMemory struct {
	embedder     Embedder
	vector       VectorStore
	graph        GraphStore
	cache        *MemoryCache
	batchSize    int
	batchTimeout time.Duration
	memBatch     []Memory
	batchMutex   sync.Mutex
	batchTimer   *time.Timer
}

// MemoryCache provides a simple in-memory cache for frequently accessed memories
type MemoryCache struct {
	items      map[string]memoryCacheItem
	mu         sync.RWMutex
	maxSize    int
	expiration time.Duration
}

type memoryCacheItem struct {
	memory    Memory
	timestamp time.Time
}

// NewMemoryCache creates a new memory cache with specified size and expiration
func NewMemoryCache(maxSize int, expiration time.Duration) *MemoryCache {
	return &MemoryCache{
		items:      make(map[string]memoryCacheItem, maxSize),
		maxSize:    maxSize,
		expiration: expiration,
	}
}

// Get retrieves a memory from the cache
func (c *MemoryCache) Get(id string) (Memory, bool) {
	c.mu.RLock()
	item, exists := c.items[id]
	c.mu.RUnlock()

	if !exists {
		return Memory{}, false
	}

	// Check if item has expired
	if time.Since(item.timestamp) > c.expiration {
		c.mu.Lock()
		delete(c.items, id)
		c.mu.Unlock()
		return Memory{}, false
	}

	return item.memory, true
}

// Set adds a memory to the cache
func (c *MemoryCache) Set(memory Memory) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If cache is full, remove oldest item
	if len(c.items) >= c.maxSize {
		var oldestID string
		var oldestTime time.Time

		for id, item := range c.items {
			if oldestID == "" || item.timestamp.Before(oldestTime) {
				oldestID = id
				oldestTime = item.timestamp
			}
		}

		if oldestID != "" {
			delete(c.items, oldestID)
		}
	}

	c.items[memory.ID] = memoryCacheItem{
		memory:    memory,
		timestamp: time.Now(),
	}
}

// NewUnifiedStore creates a new unified memory store with caching and batching
func NewUnifiedStore(embedder Embedder, vector VectorStore, graph GraphStore) *UnifiedMemory {
	store := &UnifiedMemory{
		embedder:     embedder,
		vector:       vector,
		graph:        graph,
		cache:        NewMemoryCache(1000, 10*time.Minute),
		batchSize:    50,
		batchTimeout: 5 * time.Second,
		memBatch:     make([]Memory, 0, 50),
	}

	store.batchTimer = time.AfterFunc(store.batchTimeout, func() {
		store.flushBatch()
	})
	store.batchTimer.Stop()

	return store
}

// flushBatch writes any pending memories to storage
func (u *UnifiedMemory) flushBatch() {
	u.batchMutex.Lock()
	defer u.batchMutex.Unlock()

	if len(u.memBatch) == 0 {
		return
	}

	// Copy batch to avoid race conditions
	batch := make([]Memory, len(u.memBatch))
	copy(batch, u.memBatch)
	u.memBatch = u.memBatch[:0]

	// Stop timer if it's running
	u.batchTimer.Stop()

	// Store batch in background
	go func(memories []Memory) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := u.vector.StoreMemories(ctx, memories); err != nil {
			// Log error and retry individual items
			for _, mem := range memories {
				_, _ = u.vector.StoreMemory(ctx, mem)
			}
		}

		// Store in graph if available
		if u.graph != nil {
			for _, mem := range memories {
				_, _ = u.graph.StoreMemory(ctx, mem)
			}
		}
	}(batch)
}

// StoreMemory stores a memory with batching for better performance
func (u *UnifiedMemory) StoreMemory(ctx context.Context, content string, metadata map[string]any, memType string) (string, error) {
	mem := Memory{Content: content, Metadata: metadata, Type: memType}

	// Generate embedding if needed
	if u.embedder != nil {
		emb, err := u.embedder.Embed(ctx, content)
		if err != nil {
			return "", err
		}
		mem.Embedding = emb
	}

	// Generate ID if needed
	if mem.ID == "" {
		id, err := u.vector.StoreMemory(ctx, mem)
		if err != nil {
			return "", err
		}
		mem.ID = id

		// Add to cache
		u.cache.Set(mem)

		// Store in graph if available
		if u.graph != nil {
			if _, err := u.graph.StoreMemory(ctx, mem); err != nil {
				return "", fmt.Errorf("failed to store memory in graph store: %w", err)
			}
		}

		return id, nil
	}

	// Add to batch for efficient storage
	u.batchMutex.Lock()
	defer u.batchMutex.Unlock()

	u.memBatch = append(u.memBatch, mem)

	// Flush if batch is full
	if len(u.memBatch) >= u.batchSize {
		u.flushBatch()
	} else if len(u.memBatch) == 1 {
		// Start timer for first item in batch
		u.batchTimer.Reset(u.batchTimeout)
	}

	// Add to cache
	u.cache.Set(mem)

	return mem.ID, nil
}

// CreateRelation creates a relation between two memories
func (u *UnifiedMemory) CreateRelation(ctx context.Context, source, target, relationType string, properties map[string]any) error {
	if u.graph == nil {
		return nil
	}
	return u.graph.CreateRelation(ctx, Relation{SourceID: source, TargetID: target, Type: relationType, Properties: properties})
}

// SearchSimilar searches for similar memories with caching
func (u *UnifiedMemory) SearchSimilar(ctx context.Context, query string, params SearchParams) ([]Memory, error) {
	if u.vector == nil || u.embedder == nil {
		return nil, nil
	}

	// Generate embedding
	emb, err := u.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Search vector store
	results, err := u.vector.SearchSimilar(ctx, emb, params)
	if err != nil {
		return nil, err
	}

	// Update cache with results
	for _, mem := range results {
		u.cache.Set(mem)
	}

	return results, nil
}

// FindRelated finds related memories with caching
func (u *UnifiedMemory) FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error) {
	if u.graph == nil {
		return nil, nil
	}

	// Check cache first
	cachedMem, found := u.cache.Get(id)
	if !found {
		// If not in cache, try to get from graph store
		var err error
		cachedMem, err = u.graph.GetMemory(ctx, id)
		if err != nil {
			return nil, err
		}
		u.cache.Set(cachedMem)
	}

	// Find related memories
	results, err := u.graph.FindRelated(ctx, id, relationTypes, limit)
	if err != nil {
		return nil, err
	}

	// Update cache with results
	for _, mem := range results {
		u.cache.Set(mem)
	}

	return results, nil
}

// InjectMemories injects relevant memories into a task
func (u *UnifiedMemory) InjectMemories(ctx context.Context, task TaskLike) error {
	last := task.LastMessage()
	if last == nil || u.vector == nil || u.embedder == nil {
		return nil
	}

	// Generate embedding
	emb, err := u.embedder.Embed(ctx, last.String())
	if err != nil {
		return err
	}

	// Search for similar memories
	mems, err := u.vector.SearchSimilar(ctx, emb, SearchParams{Limit: 5})
	if err != nil {
		return err
	}

	// Add memories to task
	for _, m := range mems {
		task.AddMessage("system", "memory", m.Content)

		// Find related memories if graph store is available
		if u.graph != nil {
			// Check cache first
			cachedMem, found := u.cache.Get(m.ID)
			if found {
				m = cachedMem
			}

			// Find related memories
			rels, err := u.FindRelated(ctx, m.ID, nil, 5)
			if err == nil {
				for _, r := range rels {
					task.AddMessage("system", "relation", r.Content)
				}
			}
		}
	}

	return nil
}

// ExtractMemories extracts memories from a task with batching
func (u *UnifiedMemory) ExtractMemories(ctx context.Context, task TaskLike) error {
	msg := task.LastMessage()
	if msg == nil {
		return nil
	}

	// Store memory with batching
	_, err := u.StoreMemory(ctx, msg.String(), map[string]any{"role": msg.Role}, "message")
	return err
}
