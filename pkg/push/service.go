package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

// Service represents a push notification service
type Service struct {
	mu            sync.RWMutex
	configs       map[string]*a2a.TaskPushNotificationConfig
	clients       map[string]*http.Client
	retryQueue    chan *notificationRequest
	maxRetries    int
	retryInterval time.Duration
}

// notificationRequest represents a notification to be sent
type notificationRequest struct {
	taskID    string
	event     any
	retries   int
	timestamp time.Time
}

// NewService creates a new push notification service
func NewService() *Service {
	service := &Service{
		configs:       make(map[string]*a2a.TaskPushNotificationConfig),
		clients:       make(map[string]*http.Client),
		retryQueue:    make(chan *notificationRequest, 1000),
		maxRetries:    3,
		retryInterval: time.Second * 5,
	}

	// Start the retry worker
	go service.retryWorker()

	return service
}

// SetConfig sets or updates the push notification configuration for a task
func (s *Service) SetConfig(config *a2a.TaskPushNotificationConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs[config.ID] = config
	s.clients[config.ID] = &http.Client{
		Timeout: time.Second * 10,
	}
}

// GetConfig retrieves the push notification configuration for a task
func (s *Service) GetConfig(taskID string) (*a2a.TaskPushNotificationConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.configs[taskID]
	return config, exists
}

// SendNotification sends a notification for a task
func (s *Service) SendNotification(taskID string, event any) error {
	s.mu.RLock()
	config, exists := s.configs[taskID]
	client := s.clients[taskID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no push notification config found for task %s", taskID)
	}

	// Create the request
	req, err := http.NewRequest("POST", config.PushNotificationConfig.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers if needed
	if config.PushNotificationConfig.Authentication != nil {
		for _, scheme := range config.PushNotificationConfig.Authentication.Schemes {
			if scheme == "Bearer" && config.PushNotificationConfig.Authentication.Credentials != nil {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *config.PushNotificationConfig.Authentication.Credentials))
			}
		}
	}

	// Add task token if available
	if config.PushNotificationConfig.Token != nil {
		req.Header.Set("X-Task-Token", *config.PushNotificationConfig.Token)
	}

	// Marshal the event data
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(bytes.NewReader(eventData))

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		// Queue for retry
		s.retryQueue <- &notificationRequest{
			taskID:    taskID,
			event:     event,
			retries:   0,
			timestamp: time.Now(),
		}
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		// Queue for retry
		s.retryQueue <- &notificationRequest{
			taskID:    taskID,
			event:     event,
			retries:   0,
			timestamp: time.Now(),
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// retryWorker processes the retry queue
func (s *Service) retryWorker() {
	for req := range s.retryQueue {
		// Check if we should retry
		if req.retries >= s.maxRetries {
			log.Error("Max retries reached for notification", "taskID", req.taskID)
			continue
		}

		// Wait for the retry interval
		time.Sleep(s.retryInterval)

		// Retry the notification
		if err := s.SendNotification(req.taskID, req.event); err != nil {
			// Increment retry count and queue again
			req.retries++
			req.timestamp = time.Now()
			s.retryQueue <- req
		}
	}
}
