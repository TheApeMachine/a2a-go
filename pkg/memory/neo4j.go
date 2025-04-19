package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Neo4jGraphStore implements the GraphStore interface with Neo4j
type Neo4jGraphStore struct {
	Endpoint   string
	Username   string
	Password   string
	HTTPClient *http.Client
}

// NewNeo4jGraphStore creates a new Neo4j graph store
func NewNeo4jGraphStore(endpoint, username, password string) *Neo4jGraphStore {
	return &Neo4jGraphStore{
		Endpoint:   endpoint,
		Username:   username,
		Password:   password,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// execCypher executes a Cypher query and returns the raw response
func (s *Neo4jGraphStore) execCypher(ctx context.Context, query string, params map[string]any) (map[string]any, error) {
	// Create the request payload
	payload := map[string]any{
		"statements": []map[string]any{{
			"statement":  query,
			"parameters": params,
		}},
	}
	
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/db/neo4j/tx/commit", s.Endpoint),
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	if s.Username != "" {
		req.SetBasicAuth(s.Username, s.Password)
	}
	
	// Send request
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("query failed, status: %d", resp.StatusCode)
	}
	
	// Parse response
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Check for errors in the response
	if errors, ok := result["errors"].([]any); ok && len(errors) > 0 {
		if errorObj, ok := errors[0].(map[string]any); ok {
			if msg, ok := errorObj["message"].(string); ok {
				return nil, fmt.Errorf("neo4j error: %s", msg)
			}
		}
		return nil, fmt.Errorf("neo4j returned errors")
	}
	
	return result, nil
}

// StoreMemory creates a node in the graph database
func (s *Neo4jGraphStore) StoreMemory(ctx context.Context, memory Memory) (string, error) {
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
	
	// Ensure type is set
	if memory.Type == "" {
		memory.Type = "Memory"
	}
	
	// Create parameters
	params := map[string]any{
		"id":        memory.ID,
		"content":   memory.Content,
		"createdAt": memory.CreatedAt.Format(time.RFC3339),
		"updatedAt": memory.UpdatedAt.Format(time.RFC3339),
		"type":      memory.Type,
	}
	
	// Add metadata properties
	if memory.Metadata != nil {
		params["metadata"] = memory.Metadata
	}
	
	// Create Cypher query
	cypher := `
		MERGE (m:Memory {id: $id})
		SET m.content = $content,
			m.createdAt = $createdAt,
			m.updatedAt = $updatedAt,
			m.type = $type
	`
	
	if memory.Metadata != nil {
		cypher += `,
			m.metadata = $metadata`
	}
	
	// Add any custom properties
	if memory.Properties != nil {
		params["properties"] = memory.Properties
		cypher += `,
			m += $properties`
	}
	
	// Add labels for the node
	if memory.Type != "" {
		// Add the type as a label
		cypher += fmt.Sprintf(`
		SET m:%s`, memory.Type)
	}
	
	// Add collections as labels
	if len(memory.Collections) > 0 {
		params["collections"] = memory.Collections
		cypher += `
		WITH m
		UNWIND $collections AS collection
		SET m:` + "`" + `${collection}` + "`"
	}
	
	cypher += `
		RETURN m.id AS id`
	
	// Execute the query
	result, err := s.execCypher(ctx, cypher, params)
	if err != nil {
		return "", fmt.Errorf("failed to store memory: %w", err)
	}
	
	// Parse response to get the ID
	var id string
	if results, ok := result["results"].([]any); ok && len(results) > 0 {
		if resultObj, ok := results[0].(map[string]any); ok {
			if data, ok := resultObj["data"].([]any); ok && len(data) > 0 {
				if dataObj, ok := data[0].(map[string]any); ok {
					if row, ok := dataObj["row"].([]any); ok && len(row) > 0 {
						if rowID, ok := row[0].(string); ok {
							id = rowID
						}
					}
				}
			}
		}
	}
	
	if id == "" {
		return "", fmt.Errorf("failed to get ID from response")
	}
	
	return id, nil
}

// CreateRelation creates a relationship between two nodes
func (s *Neo4jGraphStore) CreateRelation(ctx context.Context, relation Relation) error {
	// Set creation time if not set
	if relation.CreatedAt.IsZero() {
		relation.CreatedAt = time.Now().UTC()
	}
	
	// Set update time
	relation.UpdatedAt = time.Now().UTC()
	
	// Create parameters
	params := map[string]any{
		"sourceId":  relation.Source,
		"targetId":  relation.Target,
		"type":      relation.Type,
		"createdAt": relation.CreatedAt.Format(time.RFC3339),
		"updatedAt": relation.UpdatedAt.Format(time.RFC3339),
	}
	
	// Add properties
	if relation.Properties != nil {
		params["properties"] = relation.Properties
	}
	
	// Create Cypher query
	cypher := `
		MATCH (source:Memory {id: $sourceId})
		MATCH (target:Memory {id: $targetId})
		MERGE (source)-[r:` + "`" + `${type}` + "`" + `]->(target)
		SET r.createdAt = $createdAt,
			r.updatedAt = $updatedAt
	`
	
	if relation.Properties != nil {
		cypher += `,
			r += $properties`
	}
	
	cypher += `
		RETURN r`
	
	// Execute the query
	_, err := s.execCypher(ctx, cypher, params)
	if err != nil {
		return fmt.Errorf("failed to create relation: %w", err)
	}
	
	return nil
}

// GetMemory retrieves a node by ID
func (s *Neo4jGraphStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	// Create Cypher query
	cypher := `
		MATCH (m:Memory {id: $id})
		RETURN m
	`
	
	// Execute the query
	result, err := s.execCypher(ctx, cypher, map[string]any{"id": id})
	if err != nil {
		return Memory{}, fmt.Errorf("failed to get memory: %w", err)
	}
	
	// Parse response
	memory, err := s.parseMemoryFromResult(result)
	if err != nil {
		return Memory{}, err
	}
	
	if memory.ID == "" {
		return Memory{}, fmt.Errorf("memory not found: %s", id)
	}
	
	return memory, nil
}

// FindRelated finds nodes related to a given node
func (s *Neo4jGraphStore) FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error) {
	var cypher string
	params := map[string]any{
		"id":    id,
		"limit": limit,
	}
	
	// If relation types are specified, filter by them
	if len(relationTypes) > 0 {
		params["types"] = relationTypes
		cypher = `
			MATCH (m:Memory {id: $id})-[r]->(related:Memory)
			WHERE type(r) IN $types
			RETURN related
			LIMIT $limit
		`
	} else {
		cypher = `
			MATCH (m:Memory {id: $id})-[r]->(related:Memory)
			RETURN related
			LIMIT $limit
		`
	}
	
	// Execute the query
	result, err := s.execCypher(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find related memories: %w", err)
	}
	
	// Parse response
	memories, err := s.parseMemoriesFromResult(result)
	if err != nil {
		return nil, err
	}
	
	return memories, nil
}

// QueryGraph executes a custom Cypher query and returns the result as memories
func (s *Neo4jGraphStore) QueryGraph(ctx context.Context, query string, params map[string]any) ([]Memory, error) {
	// Execute the query
	result, err := s.execCypher(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute graph query: %w", err)
	}
	
	// Parse response
	memories, err := s.parseMemoriesFromResult(result)
	if err != nil {
		return nil, err
	}
	
	return memories, nil
}

// DeleteMemory removes a node from the graph
func (s *Neo4jGraphStore) DeleteMemory(ctx context.Context, id string) error {
	// Create Cypher query to delete the node and all its relationships
	cypher := `
		MATCH (m:Memory {id: $id})
		DETACH DELETE m
	`
	
	// Execute the query
	_, err := s.execCypher(ctx, cypher, map[string]any{"id": id})
	if err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}
	
	return nil
}

// DeleteRelation removes a relationship between two nodes
func (s *Neo4jGraphStore) DeleteRelation(ctx context.Context, source, target, relationType string) error {
	// Create Cypher query
	cypher := `
		MATCH (source:Memory {id: $sourceId})-[r:` + "`" + `${type}` + "`" + `]->(target:Memory {id: $targetId})
		DELETE r
	`
	
	// Execute the query
	_, err := s.execCypher(ctx, cypher, map[string]any{
		"sourceId": source,
		"targetId": target,
		"type":     relationType,
	})
	if err != nil {
		return fmt.Errorf("failed to delete relation: %w", err)
	}
	
	return nil
}

// Ping checks the connection to the Neo4j server
func (s *Neo4jGraphStore) Ping(ctx context.Context) error {
	// Simple query to check connectivity
	_, err := s.execCypher(ctx, "RETURN 1 AS n", nil)
	return err
}

// Helper function to parse a single memory from a Neo4j result
func (s *Neo4jGraphStore) parseMemoryFromResult(result map[string]any) (Memory, error) {
	var memory Memory
	
	if results, ok := result["results"].([]any); ok && len(results) > 0 {
		if resultObj, ok := results[0].(map[string]any); ok {
			if data, ok := resultObj["data"].([]any); ok && len(data) > 0 {
				if dataObj, ok := data[0].(map[string]any); ok {
					if row, ok := dataObj["row"].([]any); ok && len(row) > 0 {
						if nodeData, ok := row[0].(map[string]any); ok {
							// Extract memory fields
							if id, ok := nodeData["id"].(string); ok {
								memory.ID = id
							}
							
							if content, ok := nodeData["content"].(string); ok {
								memory.Content = content
							}
							
							if typ, ok := nodeData["type"].(string); ok {
								memory.Type = typ
							}
							
							if metadata, ok := nodeData["metadata"].(map[string]any); ok {
								memory.Metadata = metadata
							}
							
							// Parse timestamps
							if createdStr, ok := nodeData["createdAt"].(string); ok {
								if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
									memory.CreatedAt = t
								}
							}
							
							if updatedStr, ok := nodeData["updatedAt"].(string); ok {
								if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
									memory.UpdatedAt = t
								}
							}
							
							// Extract all other properties
							memory.Properties = make(map[string]interface{})
							for k, v := range nodeData {
								if k != "id" && k != "content" && k != "type" && 
								   k != "metadata" && k != "createdAt" && k != "updatedAt" {
									memory.Properties[k] = v
								}
							}
						}
					}
				}
			}
		}
	}
	
	return memory, nil
}

// Helper function to parse multiple memories from a Neo4j result
func (s *Neo4jGraphStore) parseMemoriesFromResult(result map[string]any) ([]Memory, error) {
	var memories []Memory
	
	if results, ok := result["results"].([]any); ok && len(results) > 0 {
		if resultObj, ok := results[0].(map[string]any); ok {
			if data, ok := resultObj["data"].([]any); ok {
				for _, dataEntry := range data {
					if dataObj, ok := dataEntry.(map[string]any); ok {
						if row, ok := dataObj["row"].([]any); ok && len(row) > 0 {
							if nodeData, ok := row[0].(map[string]any); ok {
								// Create a memory
								memory := Memory{
									Properties: make(map[string]interface{}),
								}
								
								// Extract memory fields
								if id, ok := nodeData["id"].(string); ok {
									memory.ID = id
								}
								
								if content, ok := nodeData["content"].(string); ok {
									memory.Content = content
								}
								
								if typ, ok := nodeData["type"].(string); ok {
									memory.Type = typ
								}
								
								if metadata, ok := nodeData["metadata"].(map[string]any); ok {
									memory.Metadata = metadata
								}
								
								// Parse timestamps
								if createdStr, ok := nodeData["createdAt"].(string); ok {
									if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
										memory.CreatedAt = t
									}
								}
								
								if updatedStr, ok := nodeData["updatedAt"].(string); ok {
									if t, err := time.Parse(time.RFC3339, updatedStr); err == nil {
										memory.UpdatedAt = t
									}
								}
								
								// Extract all other properties
								for k, v := range nodeData {
									if k != "id" && k != "content" && k != "type" && 
									   k != "metadata" && k != "createdAt" && k != "updatedAt" {
										memory.Properties[k] = v
									}
								}
								
								memories = append(memories, memory)
							}
						}
					}
				}
			}
		}
	}
	
	return memories, nil
}