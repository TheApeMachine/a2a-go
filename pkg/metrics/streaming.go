package metrics

import (
	"sync"
	"time"
)

// StreamingMetrics tracks performance metrics for streaming operations
type StreamingMetrics struct {
	mu sync.RWMutex

	// Connection metrics
	TotalConnections   int64
	FailedConnections  int64
	Reconnections      int64
	ConnectionDuration time.Duration

	// Event metrics
	TotalEvents    int64
	DroppedEvents  int64
	EventLatency   time.Duration
	ProcessingTime time.Duration
}

// NewStreamingMetrics creates a new StreamingMetrics instance
func NewStreamingMetrics() *StreamingMetrics {
	return &StreamingMetrics{}
}

// RecordConnection records a connection attempt
func (m *StreamingMetrics) RecordConnection(success bool, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalConnections++
	if !success {
		m.FailedConnections++
	}
	m.ConnectionDuration += duration
}

// RecordReconnection records a reconnection attempt
func (m *StreamingMetrics) RecordReconnection() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Reconnections++
}

// RecordEvent records an event processing
func (m *StreamingMetrics) RecordEvent(dropped bool, latency, processingTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalEvents++
	if dropped {
		m.DroppedEvents++
	}
	m.EventLatency += latency
	m.ProcessingTime += processingTime
}

// GetMetrics returns a snapshot of the current metrics
func (m *StreamingMetrics) GetMetrics() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]any{
		"total_connections":   m.TotalConnections,
		"failed_connections":  m.FailedConnections,
		"reconnections":       m.Reconnections,
		"connection_duration": m.ConnectionDuration.Seconds(),
		"total_events":        m.TotalEvents,
		"dropped_events":      m.DroppedEvents,
		"avg_event_latency":   m.EventLatency.Seconds() / float64(m.TotalEvents),
		"avg_processing_time": m.ProcessingTime.Seconds() / float64(m.TotalEvents),
	}
}
