// Package neo4j provides a *minimal* HTTP driver for Neo4j’s transactional
// Cypher endpoint.  It avoids pulling the official bolt driver so the project
// can be built without CGO and without internet access.  Only the 2‑3 calls
// required by the memory façade are implemented.

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
    Endpoint   string // e.g. http://localhost:7474
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
func (c *Client) ExecCypher(ctx context.Context, cypher string, params map[string]interface{}) (map[string]interface{}, error) {
    payload := map[string]interface{}{
        "statements": []map[string]interface{}{ {
            "statement":  cypher,
            "parameters": params,
        }},
    }
    b, _ := json.Marshal(payload)

    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/db/neo4j/tx/commit", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    if c.Username != "" {
        req.SetBasicAuth(c.Username, c.Password)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        return nil, fmt.Errorf("neo4j: status %s", resp.Status)
    }

    var out map[string]interface{}
    _ = json.NewDecoder(resp.Body).Decode(&out)
    return out, nil
}
