package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
Helper function to marshal an ID for JSON-RPC
*/
func marshalID(v int) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

/*
Agent encapsulates a remote A2A‑speaking agent.  It stores the published
AgentCard for inspection and offers helper methods for the standard task
lifecycle.  All network traffic goes through the embedded RPCClient so the
behaviour is easily customisable by swapping the underlying *http.Client* or
adding an AuthHeader callback.
*/
type Agent struct {
	chatClient  *provider.ChatClient
	card        types.AgentCard
	rpcEndpoint string
	sseEndpoint string
	rpc         jsonrpc.RPCClient
	AuthHeader  func(*http.Request)
	Logger      func(string, ...any)
}

/*
NewAgentFromCard constructs an Agent from an already‑fetched AgentCard.
No network requests are performed.
*/
func NewAgentFromCard(card types.AgentCard) *Agent {
	v := viper.GetViper()

	base := strings.TrimRight(card.URL, "/")

	agent := &Agent{
		card:        card,
		rpcEndpoint: base + v.GetString("server.defaultRPCPath"),
		sseEndpoint: base + v.GetString("server.defaultSSEPath"),
	}

	agent.rpc.Endpoint = agent.rpcEndpoint
	agent.chatClient = provider.NewChatClient(agent.execute)
	return agent
}

/*
SendStream sends tasks/sendSubscribe and dispatches streaming events to the
provided callbacks.  If the agent reports final=true the function returns
nil.  Note: this implementation performs a best‑effort SSE parse; for
production‑grade robustness applications may want a more sophisticated
parser with reconnection logic.
*/
func (agent *Agent) SendStream(
	ctx context.Context,
	params types.Task,
	onStatus func(types.TaskStatusUpdateEvent),
	onArtifact func(types.TaskArtifactUpdateEvent),
) error {
	// First perform the JSON‑RPC call but keep the HTTP response body for SSE.
	// Encode request manually because RPCClient currently hides http.Response.
	payload := jsonrpc.RPCRequest{
		JSONRPC: "2.0",
		ID:      marshalID(1),
		Method:  "tasks/sendSubscribe",
	}

	b, err := json.Marshal(params)

	if err != nil {
		return err
	}

	payload.Params = b

	body, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		agent.rpcEndpoint,
		bytes.NewReader(body),
	)

	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if agent.AuthHeader != nil {
		agent.AuthHeader(req)
	}

	httpClient := agent.httpClient()
	resp, err := httpClient.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stream request failed: HTTP %d", resp.StatusCode)
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

		// Determine event type by probing presence of fields.
		if strings.Contains(data, "\"artifact\"") {
			var evt types.TaskArtifactUpdateEvent

			if err := json.Unmarshal(
				[]byte(data), &evt,
			); err == nil && onArtifact != nil {
				onArtifact(evt)
			}

			continue
		}

		var evt types.TaskStatusUpdateEvent

		if err := json.Unmarshal(
			[]byte(data), &evt,
		); err == nil && onStatus != nil {
			onStatus(evt)

			if evt.Final {
				return nil
			}
		}
	}
}

/*
call is a helper method for making JSON‑RPC calls to the agent.
*/
func (a *Agent) call(ctx context.Context, method string, params any, result any) error {
	// Use the embedded RPCClient but inject auth headers if necessary.
	if a.rpc.HTTP == nil {
		a.rpc.HTTP = a.httpClient()
	}

	// Wrap http.Client.Transport to inject headers.
	if a.AuthHeader != nil {
		base := a.rpc.HTTP.Transport

		if base == nil {
			base = http.DefaultTransport
		}

		a.rpc.HTTP.Transport = authInjectingRoundTripper{base, a.AuthHeader}
	}

	return a.rpc.Call(ctx, method, params, result)
}

/*
httpClient returns the underlying *http.Client* for the RPCClient.
*/
func (a *Agent) httpClient() *http.Client {
	if a.rpc.HTTP != nil {
		return a.rpc.HTTP
	}

	return http.DefaultClient
}
