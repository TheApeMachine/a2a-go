// Package qdrant provides a **minimal** HTTP wrapper around a Qdrant vector
// database.  It is *not* a full‑featured client – only the handful of
// operations needed by the a2a‑go memory façade are implemented.  The code is
// written against Qdrant’s REST API so no external proto / gRPC dependencies
// are required (keeping offline builds possible).

package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/theapemachine/a2a-go/memory"
)

// Client wraps an endpoint + collection.
type Client struct {
	Endpoint   string // e.g. http://localhost:6333
	Collection string // e.g. "memory"
	httpClient *http.Client
}

// New returns a Client with sane defaults.
func New(endpoint, collection string) *Client {
	return &Client{
		Endpoint:   endpoint,
		Collection: collection,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// Put upserts a batch of documents as points.  We expect the embedder to have
// added an "embedding" float32 slice into Metadata["embedding"].  For this
// stub we do *not* calculate embeddings.
func (c *Client) Put(ctx context.Context, docs []memory.Document) error {
	// Build Qdrant “points” payload.
	var points []map[string]any
	for _, d := range docs {
		points = append(points, map[string]any{
			"id":      d.ID,
			"payload": d.Metadata,
			"vectors": d.Metadata["embedding"],
		})
	}
	body := map[string]any{"points": points}
	b, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPut,
		fmt.Sprintf("%s/collections/%s/points", c.Endpoint, c.Collection), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant: unexpected status %s", resp.Status)
	}
	return nil
}

// Search performs a basic vector search by accepting a query vector inside the
// request payload (again passed via metadata["embedding"].  For demo purposes
// we fall back to an empty result slice on error so callers can work offline.
func (c *Client) Search(ctx context.Context, queryVec []float32, limit int) ([]string, error) {
	body := map[string]any{
		"vector": queryVec,
		"limit":  limit,
	}
	b, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/collections/%s/points/search", c.Endpoint, c.Collection), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant: search status %s", resp.Status)
	}
	var out struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)

	ids := make([]string, 0, len(out.Result))
	for _, r := range out.Result {
		ids = append(ids, r.ID)
	}
	return ids, nil
}
