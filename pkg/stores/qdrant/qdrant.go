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
	req, _ := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/collections/%s/points/%s", client.Endpoint, client.Collection, id),
		nil,
	)

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

	doc := &Document{
		ID:       out.Result.ID,
		Content:  fmt.Sprintf("%v", out.Result.Payload["content"]),
		Metadata: out.Result.Payload,
	}

	return doc, nil
}

// Delete removes a document by ID.
func (client *Client) Delete(ctx context.Context, id string) error {
	req, _ := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("%s/collections/%s/points/%s", client.Endpoint, client.Collection, id),
		nil,
	)

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

	_ = json.NewDecoder(resp.Body).Decode(&out)

	docs := make([]Document, 0, len(out.Result))

	for _, r := range out.Result {
		docs = append(docs, Document{
			ID:       r.ID,
			Content:  fmt.Sprintf("%v", r.Payload["content"]),
			Metadata: r.Payload,
		})
	}

	return docs, nil
}
