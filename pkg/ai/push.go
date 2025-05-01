package ai

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/types"
)

/*
Resubscribe reconnects to an existing task's event stream.

NOTE: Currently, this only fetches the task state via RPC. Full client-side
event stream resubscription requires establishing a separate event connection
(e.g., SSE) after the RPC call, which is not yet implemented here.
*/
func (a *Agent) Resubscribe(
	ctx context.Context,
	id string,
	historyLength int,
	onStatus func(types.TaskStatusUpdateEvent),
	onArtifact func(types.TaskArtifactUpdateEvent),
) error {
	task := types.Task{}

	if err := a.rpc.Call(ctx, "tasks/resubscribe", types.TaskResubscribeParams{
		ID: id,
	}, &task); err != nil {
		log.Error("RPC call to tasks/resubscribe failed", "error", err)
		return err
	}

	log.Warn("Client-side Resubscribe event streaming not fully implemented. Only fetched current task state.", "taskID", id)

	// TODO: Implement client-side SSE connection establishment and handling
	// based on potential information returned in the task or agent card.

	// The original code attempted to read from task.Reader(), which is invalid.
	// The loop below is removed as there is no reader to read from.
	/*
		reader := bufio.NewReader(task.Reader())

		for {
			data, err := utils.ReadSSE(reader)

			if err != nil {
				return err
			}

			if data == "" {
				continue
			}

			// Determine event type by probing presence of fields
			if strings.Contains(data, "\"artifact\"") {
				var evt types.TaskArtifactUpdateEvent

				if err := json.Unmarshal([]byte(data), &evt); err == nil && onArtifact != nil {
					onArtifact(evt)
				}

				continue
			}

			var evt types.TaskStatusUpdateEvent
			if err := json.Unmarshal([]byte(data), &evt); err == nil && onStatus != nil {
				onStatus(evt)
				if evt.Final {
					return nil
				}
			}
		}
	*/

	return nil // Returning nil as the RPC succeeded, but streaming is NYI.
}

/*
SetPush sets or updates the push‑notification config.
*/
func (a *Agent) SetPush(ctx context.Context, cfg types.TaskPushNotificationConfig) error {
	return a.rpc.Call(ctx, "tasks/pushNotification/set", cfg, nil)
}

/*
GetPush fetches the push‑notification config for a task.
*/
func (a *Agent) GetPush(ctx context.Context, id string) (*types.TaskPushNotificationConfig, error) {
	params := struct {
		ID string `json:"id"`
	}{ID: id}

	var out types.TaskPushNotificationConfig
	if err := a.rpc.Call(ctx, "tasks/pushNotification/get", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
