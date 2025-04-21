package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"

	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
Resubscribe reconnects to an existing task's event stream.
*/
func (a *Agent) Resubscribe(
	ctx context.Context,
	id string,
	historyLength int,
	onStatus func(types.TaskStatusUpdateEvent),
	onArtifact func(types.TaskArtifactUpdateEvent),
) error {
	task := types.Task{}

	a.rpc.Call(ctx, "tasks/resubscribe", types.TaskResubscribeParams{
		ID: id,
	}, &task)

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
