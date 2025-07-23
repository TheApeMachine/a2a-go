package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client wraps an endpoint + collection with connection pooling and retries.
type Client struct {
	Endpoint    string // e.g. http://localhost:6333
	Collection  string // e.g. "memory"
	httpClient  *http.Client
	maxRetries  int
	retryDelay  time.Duration
	connPool    chan *http.Client
	poolSize    int
	healthCheck bool
}

// ClientOption defines functional options for the Client
type ClientOption func(*Client)

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithRetries configures retry behavior
func WithRetries(maxRetries int, retryDelay time.Duration) ClientOption {
	return func(c *Client) {
		c.maxRetries = maxRetries
		c.retryDelay = retryDelay
	}
}

// WithConnectionPool configures the connection pool
func WithConnectionPool(poolSize int) ClientOption {
	return func(c *Client) {
		c.poolSize = poolSize
		c.connPool = make(chan *http.Client, poolSize)

		// Initialize pool
		for i := 0; i < poolSize; i++ {
			c.connPool <- &http.Client{
				Timeout: c.httpClient.Timeout,
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 100,
					IdleConnTimeout:     90 * time.Second,
				},
			}
		}
	}
}

// WithHealthCheck enables periodic health checks
func WithHealthCheck(enabled bool) ClientOption {
	return func(c *Client) {
		c.healthCheck = enabled
	}
}

// New returns a Client with optimized defaults.
func New(endpoint, collection string, options ...ClientOption) *Client {
	client := &Client{
		Endpoint:   endpoint,
		Collection: collection,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		maxRetries: 3,
		retryDelay: 500 * time.Millisecond,
		poolSize:   10,
	}

	// Apply options
	for _, option := range options {
		option(client)
	}

	// Initialize connection pool if not already done
	if client.connPool == nil {
		client.connPool = make(chan *http.Client, client.poolSize)
		for i := 0; i < client.poolSize; i++ {
			client.connPool <- &http.Client{
				Timeout: client.httpClient.Timeout,
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 100,
					IdleConnTimeout:     90 * time.Second,
				},
			}
		}
	}

	// Start health check if enabled
	if client.healthCheck {
		go client.startHealthCheck()
	}

	return client
}

// startHealthCheck periodically checks the Qdrant server health
func (client *Client) startHealthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := client.Search(ctx, []float32{0}, 1)
		cancel()

		if err != nil {
			// Log health check failure
			fmt.Printf("Qdrant health check failed: %v\n", err)
		}
	}
}

// getClient gets a client from the connection pool
func (client *Client) getClient() *http.Client {
	select {
	case httpClient := <-client.connPool:
		return httpClient
	case <-time.After(1 * time.Second):
		// If pool is exhausted, create a new client
		return &http.Client{
			Timeout: client.httpClient.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}
}

// releaseClient returns a client to the connection pool
func (client *Client) releaseClient(httpClient *http.Client) {
	select {
	case client.connPool <- httpClient:
		// Successfully returned to pool
	default:
		// Pool is full, discard client
	}
}

// doRequest performs an HTTP request with retries
func (client *Client) doRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
		req  *http.Request
	)

	// Get client from pool
	httpClient := client.getClient()
	defer client.releaseClient(httpClient)

	// Try request with retries
	for attempt := 0; attempt <= client.maxRetries; attempt++ {
		// Create new request for each attempt (body might be consumed)
		if body != nil {
			// If body is a bytes.Reader, we can seek to beginning
			if bodySeeker, ok := body.(io.Seeker); ok {
				_, _ = bodySeeker.Seek(0, io.SeekStart)
			}
		}

		req, err = http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		if method == http.MethodPost || method == http.MethodPut {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err = httpClient.Do(req)

		// Success or non-retriable error
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// Close response body if we got a response
		if resp != nil {
			resp.Body.Close()
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// Continue with retries
		}

		// Wait before retry (except on last attempt)
		if attempt < client.maxRetries {
			select {
			case <-time.After(client.retryDelay * time.Duration(attempt+1)):
				// Exponential backoff
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	// Return last error
	if err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("qdrant: request failed after %d attempts", client.maxRetries+1)
}

// Get retrieves a document by ID including its payload with retries and connection pooling.
func (client *Client) Get(ctx context.Context, id string) (*Document, error) {
	url := fmt.Sprintf("%s/collections/%s/points/%s", client.Endpoint, client.Collection, id)

	resp, err := client.doRequest(ctx, http.MethodGet, url, nil)
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

// Delete removes a document by ID with retries.
func (client *Client) Delete(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/collections/%s/points/%s", client.Endpoint, client.Collection, id)

	resp, err := client.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant: delete status %s", resp.Status)
	}

	return nil
}

// Put upserts a batch of documents as points with retries and connection pooling.
func (client *Client) Put(ctx context.Context, docs []Document) error {
	// Build Qdrant "points" payload.
	var points []map[string]any

	for _, d := range docs {
		// Ensure content is in metadata
		if d.Metadata == nil {
			d.Metadata = map[string]any{}
		}
		d.Metadata["content"] = d.Content

		points = append(points, map[string]any{
			"id":      d.ID,
			"payload": d.Metadata,
			"vectors": d.Metadata["embedding"],
		})
	}

	body := map[string]any{"points": points}
	b, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/collections/%s/points", client.Endpoint, client.Collection)

	resp, err := client.doRequest(ctx, http.MethodPut, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant: put status %s", resp.Status)
	}

	return nil
}

// BatchPut efficiently upserts large batches of documents by splitting them into smaller chunks.
func (client *Client) BatchPut(ctx context.Context, docs []Document, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 100 // Default batch size
	}

	var wg sync.WaitGroup
	errChan := make(chan error, (len(docs)+batchSize-1)/batchSize)

	// Process in batches
	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}

		batch := docs[i:end]

		wg.Add(1)
		go func(batch []Document) {
			defer wg.Done()

			if err := client.Put(ctx, batch); err != nil {
				errChan <- err
			}
		}(batch)
	}

	// Wait for all batches to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		return err // Return first error
	}

	return nil
}

// Search performs a vector search with retries and connection pooling.
func (client *Client) Search(ctx context.Context, queryVec []float32, limit int) ([]Document, error) {
	body := map[string]any{
		"vector":       queryVec,
		"limit":        limit,
		"with_payload": true,
	}

	b, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/collections/%s/points/search", client.Endpoint, client.Collection)

	resp, err := client.doRequest(ctx, http.MethodPost, url, bytes.NewReader(b))
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
			Score   float64        `json:"score"`
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

		// Add score to metadata
		if r.Payload == nil {
			r.Payload = make(map[string]any)
		}
		r.Payload["_score"] = r.Score

		docs = append(docs, Document{
			ID:       r.ID,
			Content:  content,
			Metadata: r.Payload,
		})
	}

	return docs, nil
}

// FilterSearch performs a search with filters.
func (client *Client) FilterSearch(ctx context.Context, queryVec []float32, limit int, filters map[string]any) ([]Document, error) {
	body := map[string]any{
		"vector":       queryVec,
		"limit":        limit,
		"with_payload": true,
	}

	if len(filters) > 0 {
		body["filter"] = map[string]any{
			"must": buildFilters(filters),
		}
	}

	b, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/collections/%s/points/search", client.Endpoint, client.Collection)

	resp, err := client.doRequest(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qdrant: filter search status %s", resp.Status)
	}

	var out struct {
		Result []struct {
			ID      string         `json:"id"`
			Payload map[string]any `json:"payload"`
			Score   float64        `json:"score"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("qdrant: failed to decode filter search response: %w", err)
	}

	docs := make([]Document, 0, len(out.Result))

	for _, r := range out.Result {
		var content string
		if c, ok := r.Payload["content"].(string); ok {
			content = c
		} else if r.Payload["content"] != nil {
			content = fmt.Sprintf("%v", r.Payload["content"])
		}

		// Add score to metadata
		if r.Payload == nil {
			r.Payload = make(map[string]any)
		}
		r.Payload["_score"] = r.Score

		docs = append(docs, Document{
			ID:       r.ID,
			Content:  content,
			Metadata: r.Payload,
		})
	}

	return docs, nil
}

// buildFilters converts a map of filters to Qdrant filter format
func buildFilters(filters map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(filters))

	for key, value := range filters {
		filter := map[string]any{}

		switch v := value.(type) {
		case []string:
			// Handle string arrays as "any" match
			values := make([]any, len(v))
			for i, s := range v {
				values[i] = s
			}
			filter["key"] = key
			filter["match"] = map[string]any{
				"any": values,
			}
		case string:
			filter["key"] = key
			filter["match"] = map[string]any{
				"value": v,
			}
		case float64, float32, int, int64, int32:
			filter["key"] = key
			filter["match"] = map[string]any{
				"value": v,
			}
		case bool:
			filter["key"] = key
			filter["match"] = map[string]any{
				"value": v,
			}
		default:
			// Skip unsupported types
			continue
		}

		result = append(result, filter)
	}

	return result
}
