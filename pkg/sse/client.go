package sse

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/theapemachine/a2a-go/pkg/metrics"
)

// Event represents a Server-Sent Event
type Event struct {
	ID    string
	Event string
	Data  []byte
}

// Client represents an SSE client with connection management
type Client struct {
	URL           string
	Headers       map[string]string
	Metrics       *metrics.StreamingMetrics
	mu            sync.RWMutex
	conn          *http.Response
	reader        *bufio.Reader
	reconnectChan chan struct{}
	stopChan      chan struct{}
}

// NewClient creates a new SSE client
func NewClient(url string) *Client {
	return &Client{
		URL:           url,
		Headers:       make(map[string]string),
		Metrics:       metrics.NewStreamingMetrics(),
		reconnectChan: make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
	}
}

// SubscribeWithContext subscribes to an SSE stream with reconnection support
func (c *Client) SubscribeWithContext(ctx context.Context, lastEventID string, handler func(*Event)) error {
	var retryCount int
	maxRetries := 3
	baseDelay := time.Second
	shouldReconnect := false

	for {
		select {
		case <-ctx.Done():
			c.cleanup()
			return ctx.Err()
		case <-c.stopChan:
			c.cleanup()
			return nil
		case <-c.reconnectChan:
			shouldReconnect = true
		default:
			if shouldReconnect {
				c.cleanup()
				shouldReconnect = false
			}

			if err := c.connect(ctx, lastEventID); err != nil {
				if retryCount >= maxRetries {
					return fmt.Errorf("max retries exceeded: %w", err)
				}
				delay := baseDelay * time.Duration(1<<retryCount)
				time.Sleep(delay)
				retryCount++
				continue
			}

			// Reset retry count after successful connection
			retryCount = 0

			if err := c.processEvents(ctx, handler); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					shouldReconnect = true
					continue
				}
				return err
			}
		}
	}
}

// cleanup closes any existing connection and resets the client state
func (c *Client) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Body.Close()
		c.conn = nil
		c.reader = nil
	}
}

// connect establishes a new SSE connection
func (c *Client) connect(ctx context.Context, lastEventID string) error {
	startTime := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", c.URL, nil)
	if err != nil {
		c.Metrics.RecordConnection(false, time.Since(startTime))
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	// Make the request
	client := &http.Client{
		// Add timeout to client for better error handling
		Timeout: 30 * time.Second,
		// Handle redirects gracefully
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		if os.IsTimeout(err) || strings.Contains(err.Error(), "connection reset by peer") {
			// Handle connection resets and timeouts specifically
			c.Metrics.RecordConnection(false, time.Since(startTime))
			return fmt.Errorf("failed to connect (network error): %w", err)
		}
		c.Metrics.RecordConnection(false, time.Since(startTime))
		return fmt.Errorf("failed to connect: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		respBodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		c.Metrics.RecordConnection(false, time.Since(startTime))
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBodyBytes))
	}

	c.mu.Lock()
	c.conn = resp
	c.reader = bufio.NewReader(resp.Body) // Use the body normally now
	c.mu.Unlock()

	c.Metrics.RecordConnection(true, time.Since(startTime))
	return nil
}

// processEvents processes incoming SSE events
func (c *Client) processEvents(ctx context.Context, handler func(*Event)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopChan:
			return nil
		case <-c.reconnectChan:
			return io.EOF
		default:
			event, err := c.readEvent()
			if err != nil {
				return err
			}

			if event != nil {
				eventStart := time.Now()
				handler(event)
				c.Metrics.RecordEvent(false, time.Since(eventStart), time.Since(eventStart))
			}
		}
	}
}

// readEvent reads a single SSE event
func (c *Client) readEvent() (*Event, error) {
	c.mu.RLock()
	reader := c.reader
	c.mu.RUnlock()

	if reader == nil {
		return nil, io.EOF
	}

	event := &Event{}
	var eventData strings.Builder
	inEvent := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimRight(line, "\n\r")

		// Empty line marks the end of an event
		if line == "" {
			if inEvent {
				// Event is complete, return it
				event.Data = []byte(eventData.String())
				return event, nil
			}
			// Empty line but not in event, continue reading
			continue
		}

		// We're now in an event
		inEvent = true

		// Parse the line
		if strings.HasPrefix(line, "id:") {
			event.ID = strings.TrimSpace(line[3:])
		} else if strings.HasPrefix(line, "event:") {
			event.Event = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "data:") {
			// For data, we may need to append multiple lines
			dataLine := strings.TrimPrefix(line, "data:")
			if eventData.Len() > 0 {
				eventData.WriteString("\n")
			}
			eventData.WriteString(strings.TrimPrefix(dataLine, " "))
		} else if strings.HasPrefix(line, ":") {
			// Comment line, ignore
			continue
		}
	}
}

// Close closes the SSE connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.stopChan)
	if c.conn != nil {
		return c.conn.Body.Close()
	}
	return nil
}

// Reconnect triggers a reconnection
func (c *Client) Reconnect() {
	select {
	case c.reconnectChan <- struct{}{}:
	default:
	}
}
