package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/theapemachine/a2a-go/pkg/stores/neo4j"
)

// Neo4jGraphStore implements GraphStore using Neo4j with connection pooling and caching.
type Neo4jGraphStore struct {
	client      *neo4j.Client
	cache       *MemoryCache
	queryCache  *QueryCache
	batchSize   int
	batchMutex  sync.Mutex
	memBatch    []Memory
	relBatch    []Relation
	batchTimer  *time.Timer
	batchPeriod time.Duration
}

// QueryCache provides caching for frequently executed queries
type QueryCache struct {
	items      map[string]queryCacheItem
	mu         sync.RWMutex
	maxSize    int
	expiration time.Duration
}

type queryCacheItem struct {
	results   []Memory
	timestamp time.Time
}

// NewQueryCache creates a new query cache
func NewQueryCache(maxSize int, expiration time.Duration) *QueryCache {
	return &QueryCache{
		items:      make(map[string]queryCacheItem, maxSize),
		maxSize:    maxSize,
		expiration: expiration,
	}
}

// Get retrieves results from the query cache
func (c *QueryCache) Get(query string, params map[string]any) ([]Memory, bool) {
	// Create cache key from query and params
	key := query
	if len(params) > 0 {
		paramsJSON, _ := json.Marshal(params)
		key += string(paramsJSON)
	}

	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Check if item has expired
	if time.Since(item.timestamp) > c.expiration {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}

	return item.results, true
}

// Set adds results to the query cache
func (c *QueryCache) Set(query string, params map[string]any, results []Memory) {
	// Create cache key from query and params
	key := query
	if len(params) > 0 {
		paramsJSON, _ := json.Marshal(params)
		key += string(paramsJSON)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// If cache is full, remove oldest item
	if len(c.items) >= c.maxSize {
		var oldestKey string
		var oldestTime time.Time

		for k, item := range c.items {
			if oldestKey == "" || item.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = item.timestamp
			}
		}

		if oldestKey != "" {
			delete(c.items, oldestKey)
		}
	}

	c.items[key] = queryCacheItem{
		results:   results,
		timestamp: time.Now(),
	}
}

// NewNeo4jGraphStore creates a new Neo4j graph store with optimized settings
func NewNeo4jGraphStore(endpoint, user, pass string) *Neo4jGraphStore {
	store := &Neo4jGraphStore{
		client:      neo4j.New(endpoint, user, pass),
		cache:       NewMemoryCache(1000, 5*time.Minute),
		queryCache:  NewQueryCache(100, 1*time.Minute),
		batchSize:   50,
		memBatch:    make([]Memory, 0, 50),
		relBatch:    make([]Relation, 0, 100),
		batchPeriod: 2 * time.Second,
	}

	// Initialize batch timer
	store.batchTimer = time.AfterFunc(store.batchPeriod, func() {
		store.flushBatch()
	})
	store.batchTimer.Stop()

	return store
}

// flushBatch writes any pending memories and relations to Neo4j
func (s *Neo4jGraphStore) flushBatch() {
	s.batchMutex.Lock()
	defer s.batchMutex.Unlock()

	// Stop timer
	s.batchTimer.Stop()

	// Process memory batch
	if len(s.memBatch) > 0 {
		// Copy batch to avoid race conditions
		memBatch := make([]Memory, len(s.memBatch))
		copy(memBatch, s.memBatch)
		s.memBatch = s.memBatch[:0]

		// Process in background
		go func(memories []Memory) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Build batch query
			var query strings.Builder
			params := make(map[string]any)

			query.WriteString("UNWIND $batch AS item ")
			query.WriteString("MERGE (m:Memory {id: item.id}) ")
			query.WriteString("SET m.content = item.content, ")
			query.WriteString("m.type = item.type, ")
			query.WriteString("m.metadata = item.metadata ")
			query.WriteString("RETURN m.id")

			batch := make([]map[string]any, len(memories))
			for i, mem := range memories {
				mdBytes, _ := json.Marshal(mem.Metadata)
				batch[i] = map[string]any{
					"id":       mem.ID,
					"content":  mem.Content,
					"type":     mem.Type,
					"metadata": string(mdBytes),
				}
			}

			params["batch"] = batch

			_, err := s.client.ExecCypher(ctx, query.String(), params)
			if err != nil {
				// Log error and retry individual items
				for _, mem := range memories {
					mdBytes, _ := json.Marshal(mem.Metadata)
					_, _ = s.client.ExecCypher(ctx,
						"MERGE (m:Memory {id:$id}) SET m.content=$content, m.type=$type, m.metadata=$metadata RETURN m.id",
						map[string]any{"id": mem.ID, "content": mem.Content, "type": mem.Type, "metadata": string(mdBytes)})
				}
			}
		}(memBatch)
	}

	// Process relation batch
	if len(s.relBatch) > 0 {
		// Copy batch to avoid race conditions
		relBatch := make([]Relation, len(s.relBatch))
		copy(relBatch, s.relBatch)
		s.relBatch = s.relBatch[:0]

		// Group relations by type for efficient batching
		relationsByType := make(map[string][]Relation)
		for _, rel := range relBatch {
			relationsByType[rel.Type] = append(relationsByType[rel.Type], rel)
		}

		// Process each relation type in background
		for relType, relations := range relationsByType {
			go func(relType string, relations []Relation) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				// Build batch query
				var query strings.Builder
				params := make(map[string]any)

				query.WriteString("UNWIND $batch AS item ")
				query.WriteString("MATCH (a:Memory {id: item.source}), (b:Memory {id: item.target}) ")
				query.WriteString(fmt.Sprintf("MERGE (a)-[r:%s {props: item.props}]->(b)", relType))

				batch := make([]map[string]any, len(relations))
				for i, rel := range relations {
					propsBytes, _ := json.Marshal(rel.Properties)
					batch[i] = map[string]any{
						"source": rel.SourceID,
						"target": rel.TargetID,
						"props":  string(propsBytes),
					}
				}

				params["batch"] = batch

				_, err := s.client.ExecCypher(ctx, query.String(), params)
				if err != nil {
					// Log error and retry individual items
					for _, rel := range relations {
						propsBytes, _ := json.Marshal(rel.Properties)
						_, _ = s.client.ExecCypher(ctx,
							fmt.Sprintf("MATCH (a:Memory {id:$source}), (b:Memory {id:$target}) MERGE (a)-[r:%s {props:$props}]->(b)", rel.Type),
							map[string]any{"source": rel.SourceID, "target": rel.TargetID, "props": string(propsBytes)})
					}
				}
			}(relType, relations)
		}
	}
}

// StoreMemory stores a memory with batching for better performance
func (s *Neo4jGraphStore) StoreMemory(ctx context.Context, mem Memory) (string, error) {
	if mem.ID == "" {
		mem.ID = uuid.NewString()
	}

	// Add to cache immediately
	s.cache.Set(mem)

	// Add to batch
	s.batchMutex.Lock()
	defer s.batchMutex.Unlock()

	s.memBatch = append(s.memBatch, mem)

	// Flush if batch is full
	if len(s.memBatch) >= s.batchSize {
		s.flushBatch()
	} else if len(s.memBatch) == 1 {
		// Start timer for first item in batch
		s.batchTimer.Reset(s.batchPeriod)
	}

	return mem.ID, nil
}

// CreateRelation creates a relation with batching for better performance
func (s *Neo4jGraphStore) CreateRelation(ctx context.Context, rel Relation) error {
	// Add to batch
	s.batchMutex.Lock()
	defer s.batchMutex.Unlock()

	s.relBatch = append(s.relBatch, rel)

	// Flush if batch is full
	if len(s.relBatch) >= s.batchSize*2 { // Relations can be batched more aggressively
		s.flushBatch()
	} else if len(s.memBatch) == 0 && len(s.relBatch) == 1 {
		// Start timer for first item in batch
		s.batchTimer.Reset(s.batchPeriod)
	}

	return nil
}

// GetMemory retrieves a memory with caching
func (s *Neo4jGraphStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	// Check cache first
	if cachedMem, found := s.cache.Get(id); found {
		return cachedMem, nil
	}

	// Not in cache, query Neo4j
	out, err := s.client.ExecCypher(ctx,
		"MATCH (m:Memory {id:$id}) RETURN m.id as id, m.content as content, m.metadata as metadata, m.type as type",
		map[string]any{"id": id})

	if err != nil {
		return Memory{}, err
	}

	if len(out["results"].([]any)) == 0 {
		return Memory{}, fmt.Errorf("not found")
	}

	row := out["results"].([]any)[0].(map[string]any)["data"].([]any)[0].(map[string]any)["row"].([]any)

	meta := make(map[string]any)
	if err := json.Unmarshal([]byte(row[2].(string)), &meta); err != nil {
		return Memory{}, fmt.Errorf("failed to unmarshal metadata for memory %s: %w", id, err)
	}

	mem := Memory{
		ID:       row[0].(string),
		Content:  row[1].(string),
		Metadata: meta,
		Type:     row[3].(string),
	}

	// Add to cache
	s.cache.Set(mem)

	return mem, nil
}

// FindRelated finds related memories with caching
func (s *Neo4jGraphStore) FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error) {
	var query string
	params := map[string]any{"id": id, "limit": limit}

	// Build query based on relation types
	if len(relationTypes) == 0 {
		query = "MATCH (a:Memory {id:$id})-->(b:Memory) RETURN b.id as id, b.content as content, b.metadata as metadata, b.type as type LIMIT $limit"
	} else {
		var relTypeStr string
		for i, relType := range relationTypes {
			if i > 0 {
				relTypeStr += "|"
			}
			relTypeStr += ":" + relType
		}
		query = fmt.Sprintf("MATCH (a:Memory {id:$id})-[r %s]->(b:Memory) RETURN b.id as id, b.content as content, b.metadata as metadata, b.type as type LIMIT $limit", relTypeStr)
	}

	// Check query cache
	if cachedResults, found := s.queryCache.Get(query, params); found {
		return cachedResults, nil
	}

	// Execute query
	out, err := s.client.ExecCypher(ctx, query, params)
	if err != nil {
		return nil, err
	}

	rows := out["results"].([]any)[0].(map[string]any)["data"].([]any)
	mems := make([]Memory, 0, len(rows))

	for _, r := range rows {
		row := r.(map[string]any)["row"].([]any)

		meta := make(map[string]any)
		if err := json.Unmarshal([]byte(row[2].(string)), &meta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata for related memory %s: %w", row[0].(string), err)
		}

		mem := Memory{
			ID:       row[0].(string),
			Content:  row[1].(string),
			Metadata: meta,
			Type:     row[3].(string),
		}

		mems = append(mems, mem)

		// Add to memory cache
		s.cache.Set(mem)
	}

	// Add to query cache
	s.queryCache.Set(query, params, mems)

	return mems, nil
}

// QueryGraph executes a custom Cypher query with caching
func (s *Neo4jGraphStore) QueryGraph(ctx context.Context, query string, params map[string]any) ([]Memory, error) {
	// Check query cache
	if cachedResults, found := s.queryCache.Get(query, params); found {
		return cachedResults, nil
	}

	// Execute query
	out, err := s.client.ExecCypher(ctx, query, params)
	if err != nil {
		return nil, err
	}

	rows := out["results"].([]any)[0].(map[string]any)["data"].([]any)
	mems := make([]Memory, 0, len(rows))

	for _, r := range rows {
		row := r.(map[string]any)["row"].([]any)

		meta := make(map[string]any)
		if len(row) > 2 {
			if err := json.Unmarshal([]byte(fmt.Sprintf("%v", row[2])), &meta); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata for query graph: %w", err)
			}
		}

		mem := Memory{
			ID:       fmt.Sprintf("%v", row[0]),
			Content:  fmt.Sprintf("%v", row[1]),
			Metadata: meta,
		}

		mems = append(mems, mem)

		// Add to memory cache
		s.cache.Set(mem)
	}

	// Add to query cache
	s.queryCache.Set(query, params, mems)

	return mems, nil
}

// DeleteMemory removes a memory and its relations
func (s *Neo4jGraphStore) DeleteMemory(ctx context.Context, id string) error {
	// Remove from cache
	s.cache.mu.Lock()
	delete(s.cache.items, id)
	s.cache.mu.Unlock()

	// Clear query cache since results may change
	s.queryCache.mu.Lock()
	s.queryCache.items = make(map[string]queryCacheItem)
	s.queryCache.mu.Unlock()

	// Delete from Neo4j
	_, err := s.client.ExecCypher(ctx, "MATCH (m:Memory {id:$id}) DETACH DELETE m", map[string]any{"id": id})
	return err
}

// DeleteRelation removes a relation
func (s *Neo4jGraphStore) DeleteRelation(ctx context.Context, source, target, relationType string) error {
	// Clear query cache since results may change
	s.queryCache.mu.Lock()
	s.queryCache.items = make(map[string]queryCacheItem)
	s.queryCache.mu.Unlock()

	// Delete from Neo4j
	_, err := s.client.ExecCypher(ctx,
		fmt.Sprintf("MATCH (a:Memory {id:$source})-[r:%s]->(b:Memory {id:$target}) DELETE r", relationType),
		map[string]any{"source": source, "target": target})
	return err
}

// Ping checks if the Neo4j connection is alive
func (s *Neo4jGraphStore) Ping(ctx context.Context) error {
	_, err := s.client.ExecCypher(ctx, "RETURN 1", nil)
	return err
}
