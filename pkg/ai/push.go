package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/theapemachine/a2a-go/pkg/push"
	"github.com/theapemachine/a2a-go/pkg/sse"
	"github.com/theapemachine/a2a-go/pkg/types"
)

/*
Resubscribe handles reconnection to the event stream for a task.

NOTE: Currently, this only fetches the task state via RPC. Full client-side
event stream resubscription requires establishing a separate event connection
(e.g., SSE) after the RPC call, which is not yet implemented here.
*/
func (a *Agent) Resubscribe(ctx context.Context, taskID string) error {
	// Get the current task state
	task, rpcErr := a.GetTask(ctx, taskID, 0) // 0 for no history
	if rpcErr != nil {
		return fmt.Errorf("failed to get task state: %w", rpcErr)
	}

	// Get the push notification configuration
	config, rpcErr := a.GetPushNotification(ctx, taskID)
	if rpcErr != nil {
		return fmt.Errorf("failed to get push notification config: %w", rpcErr)
	}

	// Initialize SSE client for the task's event stream
	sseClient := sse.NewClient(config.PushNotificationConfig.URL)
	if a.AuthHeader != nil {
		// Convert AuthHeader function to map[string]string
		headers := make(map[string]string)
		req, _ := http.NewRequest("GET", config.PushNotificationConfig.URL, nil)
		a.AuthHeader(req)
		for k, v := range req.Header {
			headers[k] = v[0]
		}
		sseClient.Headers = headers
	}

	// Create a channel for backpressure control
	backpressure := make(chan struct{}, 10) // Allow up to 10 pending updates

	// Start SSE subscription in a goroutine
	go func() {
		// Retry loop for reconnection
		for retries := 0; retries < 3; retries++ {
			err := sseClient.SubscribeWithContext(ctx, "", func(event *sse.Event) {
				// Process incoming events
				switch event.Event {
				case "task_status":
					var status types.TaskStatus
					if err := json.Unmarshal(event.Data, &status); err != nil {
						a.Logger("Failed to unmarshal task status: %v", err)
						return
					}
					task.Status = status
					a.notifier(&task)
				case "artifact":
					var artifact types.Artifact
					if err := json.Unmarshal(event.Data, &artifact); err != nil {
						a.Logger("Failed to unmarshal artifact: %v", err)
						return
					}
					task.Artifacts = append(task.Artifacts, artifact)
					a.notifier(&task)
				}

				// Signal completion of processing
				backpressure <- struct{}{}
			})

			if err == nil {
				break // Successfully processed all events
			}

			// Log reconnection attempt
			a.Logger("Reconnection attempt %d failed: %v", retries+1, err)
			sseClient.Metrics.RecordReconnection()

			// Exponential backoff
			time.Sleep(time.Duration(math.Pow(2, float64(retries))) * time.Second)
		}
	}()

	return nil
}

/*
SetPush sets or updates the push notification configuration for a task
*/
func (a *Agent) SetPush(ctx context.Context, config *types.TaskPushNotificationConfig) error {
	if a.pushService == nil {
		a.pushService = push.NewService()
	}

	a.pushService.SetConfig(config)
	return nil
}

/*
GetPush retrieves the push notification configuration for a task
*/
func (a *Agent) GetPush(ctx context.Context, id string) (*types.TaskPushNotificationConfig, error) {
	if a.pushService == nil {
		return nil, fmt.Errorf("push notification service not initialized")
	}

	config, exists := a.pushService.GetConfig(id)
	if !exists {
		return nil, fmt.Errorf("no push notification config found for task %s", id)
	}

	return config, nil
}

/*
SendPushNotification sends a notification for a task
*/
func (a *Agent) SendPushNotification(taskID string, event interface{}) error {
	if a.pushService == nil {
		return fmt.Errorf("push notification service not initialized")
	}

	return a.pushService.SendNotification(taskID, event)
}
