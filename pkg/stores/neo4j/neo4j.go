package neo4j

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	Endpoint   string
	Username   string
	Password   string
	httpClient *http.Client
}

func New(endpoint, user, pass string) *Client {
	return &Client{
		Endpoint:   endpoint,
		Username:   user,
		Password:   pass,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// ExecCypher sends a single Cypher statement with optional parameters and
// returns the raw Neo4j JSON response.
func (client *Client) ExecCypher(
	ctx context.Context, cypher string, params map[string]any,
) (map[string]any, error) {
	payload := map[string]any{
		"statements": []map[string]any{{
			"statement":  cypher,
			"parameters": params,
		}},
	}

	b, err := json.Marshal(payload)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		client.Endpoint+"/db/neo4j/tx/commit",
		bytes.NewReader(b),
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if client.Username != "" {
		req.SetBasicAuth(client.Username, client.Password)
	}

	resp, err := client.httpClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("neo4j: status %s", resp.Status)
	}

	var out map[string]any

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}
