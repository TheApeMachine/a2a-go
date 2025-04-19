package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// QdrantVectorStore implements the VectorStore interface with Qdrant
type QdrantVectorStore struct {
	Endpoint   string
	Collection string
	HTTPClient *http.Client
	Embedding  EmbeddingService
}

// NewQdrantVectorStore creates a new Qdrant vector store
func NewQdrantVectorStore(endpoint, collection string, embeddingService EmbeddingService) *QdrantVectorStore {
	return &QdrantVectorStore{
		Endpoint:   endpoint,
		Collection: collection,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		Embedding: embeddingService,
	}
}

// ensureCollection makes sure the collection exists, creating it if needed
func (s *QdrantVectorStore) ensureCollection(ctx context.Context, dimension int) error {
	// Check if collection exists
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/collections/%s", s.Endpoint, s.Collection),
		nil,
	)
	if err != nil {
		return err
	}
	
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	
	// Collection exists
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	
	// Create collection if it doesn't exist
	createPayload := map[string]any{
		"name": s.Collection,
		"vectors": map[string]any{
			"size":     dimension,
			"distance": "Cosine",
		},
	}
	
	createBody, err := json.Marshal(createPayload)
	if err != nil {
		return err
	}
	
	createReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		fmt.Sprintf("%s/collections/%s", s.Endpoint, s.Collection),
		bytes.NewReader(createBody),
	)
	if err != nil {
		return err
	}
	createReq.Header.Set("Content-Type", "application/json")
	
	createResp, err := s.HTTPClient.Do(createReq)
	if err != nil {
		return err
	}
	createResp.Body.Close()
	
	if createResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create collection, status: %d", createResp.StatusCode)
	}
	
	return nil
}

// StoreMemory adds a memory to the vector store
func (s *QdrantVectorStore) StoreMemory(ctx context.Context, memory Memory) (string, error) {
	// Generate embedding if not provided
	if memory.Embedding == nil || len(memory.Embedding) == 0 {
		embedding, err := s.Embedding.GenerateEmbedding(ctx, memory.Content)
		if err != nil {
			return "", fmt.Errorf("failed to generate embedding: %w", err)
		}
		memory.Embedding = embedding
	}
	
	// Ensure collection exists
	if err := s.ensureCollection(ctx, len(memory.Embedding)); err != nil {
		return "", fmt.Errorf("failed to ensure collection: %w", err)
	}
	
	// If no ID is provided, use the current timestamp
	if memory.ID == "" {
		memory.ID = fmt.Sprintf("mem_%d", time.Now().UnixNano())
	}
	
	// Set creation time if not set
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = time.Now().UTC()
	}
	
	// Set update time
	memory.UpdatedAt = time.Now().UTC()
	
	// Create a point
	point := map[string]any{
		"id": memory.ID,
		"vector": memory.Embedding,
		"payload": map[string]any{
			"content":   memory.Content,
			"metadata":  memory.Metadata,
			"createdAt": memory.CreatedAt,
			"updatedAt": memory.UpdatedAt,
			"type":      memory.Type,
		},
	}
	
	// Create the request payload
	payload := map[string]any{
		"points": []map[string]any{point},
	}
	
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Make the request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		fmt.Sprintf("%s/collections/%s/points", s.Endpoint, s.Collection),
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to store memory, status: %d", resp.StatusCode)
	}
	
	return memory.ID, nil
}

// StoreMemories adds multiple memories to the vector store in a batch
func (s *QdrantVectorStore) StoreMemories(ctx context.Context, memories []Memory) error {
	if len(memories) == 0 {
		return nil
	}
	
	// Generate embeddings for memories that don't have them
	var textsToEmbed []string
	var indicesNeedingEmbedding []int
	
	for i, memory := range memories {
		if memory.Embedding == nil || len(memory.Embedding) == 0 {
			textsToEmbed = append(textsToEmbed, memory.Content)
			indicesNeedingEmbedding = append(indicesNeedingEmbedding, i)
		}
	}
	
	if len(textsToEmbed) > 0 {
		embeddings, err := s.Embedding.GenerateEmbeddings(ctx, textsToEmbed)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings: %w", err)
		}
		
		for i, idx := range indicesNeedingEmbedding {
			memories[idx].Embedding = embeddings[i]
		}
	}
	
	// Ensure collection exists (using the dimension from the first memory)
	if err := s.ensureCollection(ctx, len(memories[0].Embedding)); err != nil {
		return fmt.Errorf("failed to ensure collection: %w", err)
	}
	
	// Set timestamps and IDs
	now := time.Now().UTC()
	var points []map[string]any
	
	for i := range memories {
		if memories[i].ID == "" {
			memories[i].ID = fmt.Sprintf("mem_%d_%d", now.UnixNano(), i)
		}
		
		if memories[i].CreatedAt.IsZero() {
			memories[i].CreatedAt = now
		}
		
		memories[i].UpdatedAt = now
		
		points = append(points, map[string]any{
			"id": memories[i].ID,
			"vector": memories[i].Embedding,
			"payload": map[string]any{
				"content":   memories[i].Content,
				"metadata":  memories[i].Metadata,
				"createdAt": memories[i].CreatedAt,
				"updatedAt": memories[i].UpdatedAt,
				"type":      memories[i].Type,
			},
		})
	}
	
	// Create the request payload
	payload := map[string]any{
		"points": points,
	}
	
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Make the request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		fmt.Sprintf("%s/collections/%s/points", s.Endpoint, s.Collection),
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to store memories, status: %d", resp.StatusCode)
	}
	
	return nil
}

// GetMemory retrieves a memory by ID
func (s *QdrantVectorStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	// Create the request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/collections/%s/points/%s", s.Endpoint, s.Collection, id),
		nil,
	)
	if err != nil {
		return Memory{}, fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return Memory{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNotFound {
		return Memory{}, fmt.Errorf("memory not found: %s", id)
	}
	
	if resp.StatusCode != http.StatusOK {
		return Memory{}, fmt.Errorf("failed to get memory, status: %d", resp.StatusCode)
	}
	
	// Parse the response
	var result struct {
		Result struct {
			ID      string          `json:"id"`
			Payload map[string]any  `json:"payload"`
			Vector  []float32       `json:"vector"`
		} `json:"result"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Memory{}, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Extract memory from payload
	memory := Memory{
		ID:        result.Result.ID,
		Embedding: result.Result.Vector,
	}
	
	// Extract fields from payload
	if payload := result.Result.Payload; payload != nil {
		if content, ok := payload["content"].(string); ok {
			memory.Content = content
		}
		
		if metadata, ok := payload["metadata"].(map[string]any); ok {
			memory.Metadata = metadata
		}
		
		if typ, ok := payload["type"].(string); ok {
			memory.Type = typ
		}
		
		// Parse timestamps
		if createdStr, ok := payload["createdAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
				memory.CreatedAt = t
			}
		}
		
		if updatedStr, ok := payload["updatedAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
				memory.UpdatedAt = t
			}
		}
	}
	
	return memory, nil
}

// SearchSimilar finds semantically similar memories
func (s *QdrantVectorStore) SearchSimilar(ctx context.Context, embedding []float32, params SearchParams) ([]Memory, error) {
	// Prepare filters if needed
	var filter map[string]any
	if len(params.Filters) > 0 {
		// Convert filters to Qdrant format
		conditions := make([]map[string]any, 0, len(params.Filters))
		
		for _, f := range params.Filters {
			condition := map[string]any{}
			
			switch f.Operator {
			case "eq":
				condition["match"] = map[string]any{
					"key":   fmt.Sprintf("metadata.%s", f.Field),
					"value": f.Value,
				}
			case "ne":
				condition["match"] = map[string]any{
					"key":     fmt.Sprintf("metadata.%s", f.Field),
					"value":   f.Value,
					"matched": false,
				}
			case "gt", "gte", "lt", "lte":
				rangeOp := map[string]any{}
				
				switch f.Operator {
				case "gt":
					rangeOp["gt"] = f.Value
				case "gte":
					rangeOp["gte"] = f.Value
				case "lt":
					rangeOp["lt"] = f.Value
				case "lte":
					rangeOp["lte"] = f.Value
				}
				
				condition["range"] = map[string]any{
					"key":   fmt.Sprintf("metadata.%s", f.Field),
					"range": rangeOp,
				}
			}
			
			if len(condition) > 0 {
				conditions = append(conditions, condition)
			}
		}
		
		if len(conditions) > 0 {
			filter = map[string]any{
				"must": conditions,
			}
		}
	}
	
	// Prepare type filter if specified
	if len(params.Types) > 0 {
		typeConditions := make([]map[string]any, 0, len(params.Types))
		
		for _, t := range params.Types {
			typeConditions = append(typeConditions, map[string]any{
				"match": map[string]any{
					"key":   "type",
					"value": t,
				},
			})
		}
		
		// Add type conditions to filter
		if filter == nil {
			filter = map[string]any{}
		}
		
		if len(typeConditions) == 1 {
			if must, ok := filter["must"].([]map[string]any); ok {
				filter["must"] = append(must, typeConditions[0])
			} else {
				filter["must"] = []map[string]any{typeConditions[0]}
			}
		} else {
			if must, ok := filter["must"].([]map[string]any); ok {
				filter["must"] = append(must, map[string]any{
					"should": typeConditions,
				})
			} else {
				filter["must"] = []map[string]any{
					{
						"should": typeConditions,
					},
				}
			}
		}
	}
	
	// Create the search request
	searchPayload := map[string]any{
		"vector":    embedding,
		"limit":     params.Limit,
		"with_payload": true,
		"with_vector": true,
	}
	
	// Add filter if present
	if filter != nil {
		searchPayload["filter"] = filter
	}
	
	// Marshal the request
	body, err := json.Marshal(searchPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/collections/%s/points/search", s.Endpoint, s.Collection),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Send request
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed, status: %d", resp.StatusCode)
	}
	
	// Parse response
	var result struct {
		Result []struct {
			ID      string         `json:"id"`
			Score   float32        `json:"score"`
			Payload map[string]any `json:"payload"`
			Vector  []float32      `json:"vector"`
		} `json:"result"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Convert to Memory objects
	memories := make([]Memory, 0, len(result.Result))
	
	for _, item := range result.Result {
		memory := Memory{
			ID:        item.ID,
			Embedding: item.Vector,
		}
		
		// Extract fields from payload
		if payload := item.Payload; payload != nil {
			if content, ok := payload["content"].(string); ok {
				memory.Content = content
			}
			
			if metadata, ok := payload["metadata"].(map[string]any); ok {
				memory.Metadata = metadata
			}
			
			if typ, ok := payload["type"].(string); ok {
				memory.Type = typ
			}
			
			// Parse timestamps
			if createdStr, ok := payload["createdAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
					memory.CreatedAt = t
				}
			}
			
			if updatedStr, ok := payload["updatedAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
					memory.UpdatedAt = t
				}
			}
		}
		
		memories = append(memories, memory)
	}
	
	return memories, nil
}

// DeleteMemory removes a memory from the vector store
func (s *QdrantVectorStore) DeleteMemory(ctx context.Context, id string) error {
	// Create the request payload
	payload := map[string]any{
		"points": []string{id},
	}
	
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/collections/%s/points/delete", s.Endpoint, s.Collection),
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Send request
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete failed, status: %d", resp.StatusCode)
	}
	
	return nil
}

// Ping checks the connection to the Qdrant server
func (s *QdrantVectorStore) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/collections", s.Endpoint),
		nil,
	)
	if err != nil {
		return err
	}
	
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping failed, status: %d", resp.StatusCode)
	}
	
	return nil
}