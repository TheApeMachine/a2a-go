package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
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
	params := struct {
		ID            string `json:"id"`
		HistoryLength int    `json:"historyLength,omitempty"`
	}{ID: id, HistoryLength: historyLength}

	// First perform the JSON‑RPC call but keep the HTTP response body for SSE
	payload := jsonrpc.RPCRequest{
		JSONRPC: "2.0",
		ID:      marshalID(1),
		Method:  "tasks/resubscribe",
	}
	b, _ := json.Marshal(params)
	payload.Params = b

	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.rpcEndpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.AuthHeader != nil {
		a.AuthHeader(req)
	}

	httpClient := a.httpClient()
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resubscribe request failed: HTTP %d", resp.StatusCode)
	}

	// Read the first response event which contains the task status
	var firstResponse struct {
		JSONRPC string           `json:"jsonrpc"`
		ID      json.RawMessage  `json:"id"`
		Result  json.RawMessage  `json:"result"`
		Error   *errors.RpcError `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&firstResponse); err != nil {
		return err
	}

	if firstResponse.Error != nil {
		return fmt.Errorf("resubscribe failed: %s", firstResponse.Error.Message)
	}

	reader := bufio.NewReader(resp.Body)

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
	return a.call(ctx, "tasks/pushNotification/set", cfg, nil)
}

/*
GetPush fetches the push‑notification config for a task.
*/
func (a *Agent) GetPush(ctx context.Context, id string) (*types.TaskPushNotificationConfig, error) {
	params := struct {
		ID string `json:"id"`
	}{ID: id}

	var out types.TaskPushNotificationConfig
	if err := a.call(ctx, "tasks/pushNotification/get", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
