package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/theapemachine/a2a-go/pkg/types"
)

func TestSSEBrokerBroadcast(t *testing.T) {
	broker := NewTestSSEBroker()

	// HTTP server exposing /events endpoint.
	ts, errTS := newTestServerSSE(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		broker.Subscribe(w, r)
	}))
	if errTS != nil {
		t.Skip("network disabled; skipping SSE test")
	}
	defer ts.Close()

	// Create client connection.
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("client get: %v", err)
	}
	defer resp.Body.Close()

	// Wait briefly to ensure subscription established.
	time.Sleep(100 * time.Millisecond)

	// Broadcast an event.
	ev := types.TaskStatusUpdateEvent{
		ID:    "abc",
		Final: true,
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
		},
	}

	// Try with the old Broadcast method first for backward compatibility
	if err := broker.Broadcast(ev); err != nil {
		t.Fatalf("broadcast: %v", err)
	}

	reader := bufio.NewReader(resp.Body)
	var eventType string
	var dataLine string
	deadline := time.After(1 * time.Second)
	lineCount := 0

L:
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for SSE data line after reading %d lines", lineCount)
		default:
			var err error
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("read error after %d lines: %v", lineCount, err)
			}

			lineCount++
			t.Logf("Read line %d: %q", lineCount, line)

			line = strings.TrimSpace(line)
			// Skip blank lines and comments
			if line == "" || strings.HasPrefix(line, ":") {
				t.Logf("Skipping blank or comment line")
				continue
			}

			// Check if this is an event line
			if strings.HasPrefix(line, "event:") {
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				t.Logf("Found event type: %q", eventType)
				continue
			}

			// Check if this is a data line
			if strings.HasPrefix(line, "data:") {
				dataLine = strings.TrimPrefix(line, "data:")
				dataLine = strings.TrimSpace(dataLine)
				t.Logf("Found data: %q", dataLine)

				// For backward compatibility, if we find data without event type, use the default
				if eventType == "" {
					eventType = "message"
					t.Logf("No event type found, using default: %q", eventType)
				}
				break L
			}

			t.Logf("Unknown line format: %q", line)
		}
	}

	// We don't specifically check event type to maintain backward compatibility

	var got types.TaskStatusUpdateEvent
	if err := json.Unmarshal([]byte(dataLine), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != ev.ID || got.Status.State != ev.Status.State || !got.Final {
		t.Fatalf("event mismatch: %+v vs %+v", got, ev)
	}

	// Close the response body first to trigger the context cancellation
	resp.Body.Close()
	// Then close the broker
	broker.Close()
}

// newTestServer mirrors the helper in jsonrpc_test.go – duplicated to avoid
// import cycles in tests.
func newTestServerSSE(h http.Handler) (*httptest.Server, error) {
	var srv *httptest.Server
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("listener not permitted: %v", r)
			}
		}()
		srv = httptest.NewServer(h)
	}()
	return srv, err
}
