package memory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/theapemachine/a2a-go/pkg/stores/neo4j"
)

// Neo4jGraphStore implements GraphStore using Neo4j.
type Neo4jGraphStore struct {
	client *neo4j.Client
}

func NewNeo4jGraphStore(endpoint, user, pass string) *Neo4jGraphStore {
	return &Neo4jGraphStore{client: neo4j.New(endpoint, user, pass)}
}

func (s *Neo4jGraphStore) StoreMemory(ctx context.Context, mem Memory) (string, error) {
	if mem.ID == "" {
		mem.ID = uuid.NewString()
	}
	mdBytes, err := json.Marshal(mem.Metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	_, err = s.client.ExecCypher(ctx,
		"MERGE (m:Memory {id:$id}) SET m.content=$content, m.type=$type, m.metadata=$metadata RETURN m.id",
		map[string]any{"id": mem.ID, "content": mem.Content, "type": mem.Type, "metadata": string(mdBytes)})
	if err != nil {
		return "", err
	}
	return mem.ID, nil
}

func (s *Neo4jGraphStore) CreateRelation(ctx context.Context, rel Relation) error {
	props, err := json.Marshal(rel.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal relation properties: %w", err)
	}
	_, err = s.client.ExecCypher(ctx,
		fmt.Sprintf("MATCH (a:Memory {id:$source}), (b:Memory {id:$target}) MERGE (a)-[r:%s {props:$props}]->(b)", rel.Type),
		map[string]any{"source": rel.SourceID, "target": rel.TargetID, "props": string(props)})
	return err
}

func (s *Neo4jGraphStore) GetMemory(ctx context.Context, id string) (Memory, error) {
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
	return Memory{ID: row[0].(string), Content: row[1].(string), Metadata: meta, Type: row[3].(string)}, nil
}

func (s *Neo4jGraphStore) FindRelated(ctx context.Context, id string, relationTypes []string, limit int) ([]Memory, error) {
	query := "MATCH (a:Memory {id:$id})-->(b:Memory) RETURN b.id as id, b.content as content, b.metadata as metadata, b.type as type LIMIT $limit"
	out, err := s.client.ExecCypher(ctx, query, map[string]any{"id": id, "limit": limit})
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
		mems = append(mems, Memory{ID: row[0].(string), Content: row[1].(string), Metadata: meta, Type: row[3].(string)})
	}
	return mems, nil
}

func (s *Neo4jGraphStore) QueryGraph(ctx context.Context, query string, params map[string]any) ([]Memory, error) {
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
		mems = append(mems, Memory{ID: fmt.Sprintf("%v", row[0]), Content: fmt.Sprintf("%v", row[1]), Metadata: meta})
	}
	return mems, nil
}

func (s *Neo4jGraphStore) DeleteMemory(ctx context.Context, id string) error {
	_, err := s.client.ExecCypher(ctx, "MATCH (m:Memory {id:$id}) DETACH DELETE m", map[string]any{"id": id})
	return err
}

func (s *Neo4jGraphStore) DeleteRelation(ctx context.Context, source, target, relationType string) error {
	_, err := s.client.ExecCypher(ctx,
		fmt.Sprintf("MATCH (a:Memory {id:$source})-[r:%s]->(b:Memory {id:$target}) DELETE r", relationType),
		map[string]any{"source": source, "target": target})
	return err
}

func (s *Neo4jGraphStore) Ping(ctx context.Context) error {
	_, err := s.client.ExecCypher(ctx, "RETURN 1", nil)
	return err
}
