package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client wraps an endpoint + collection.
type Client struct {
	Endpoint   string // e.g. http://localhost:6333
	Collection string // e.g. "memory"
	httpClient *http.Client
}

// Get retrieves a document by ID including its payload.
func (client *Client) Get(ctx context.Context, id string) (*Document, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/collections/%s/points/%s", client.Endpoint, client.Collection, id),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.httpClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant: get status %s", resp.Status)
	}

	var out struct {
		Result struct {
			ID      string         `json:"id"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	var content string
	if c, ok := out.Result.Payload["content"].(string); ok {
		content = c
	} else if out.Result.Payload["content"] != nil {
		content = fmt.Sprintf("%v", out.Result.Payload["content"])
	}
	doc := &Document{
		ID:       out.Result.ID,
		Content:  content,
		Metadata: out.Result.Payload,
	}

	return doc, nil
}

// Delete removes a document by ID.
func (client *Client) Delete(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("%s/collections/%s/points/%s", client.Endpoint, client.Collection, id),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := client.httpClient.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant: delete status %s", resp.Status)
	}

	return nil
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
func (client *Client) Put(ctx context.Context, docs []Document) error {
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

	req, _ := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		fmt.Sprintf("%s/collections/%s/points", client.Endpoint, client.Collection),
		bytes.NewReader(b),
	)

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)

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
func (client *Client) Search(ctx context.Context, queryVec []float32, limit int) ([]Document, error) {
	body := map[string]any{
		"vector":       queryVec,
		"limit":        limit,
		"with_payload": true,
	}

	b, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/collections/%s/points/search", client.Endpoint, client.Collection),
		bytes.NewReader(b),
	)

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant: search status %s", resp.Status)
	}

	var out struct {
		Result []struct {
			ID      string         `json:"id"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("qdrant: failed to decode search response: %w", err)
	}

	docs := make([]Document, 0, len(out.Result))

	for _, r := range out.Result {
		var content string
		if c, ok := r.Payload["content"].(string); ok {
			content = c
		} else if r.Payload["content"] != nil {
			content = fmt.Sprintf("%v", r.Payload["content"])
		}
		docs = append(docs, Document{
			ID:       r.ID,
			Content:  content,
			Metadata: r.Payload,
		})
	}

	return docs, nil
}
