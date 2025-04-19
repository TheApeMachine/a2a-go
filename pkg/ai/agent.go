package ai

// Agent is a high‑level façade that hides the raw JSON‑RPC wiring and exposes
// convenience methods that map directly to the A2A Task operations described
// in TECHSpec.txt.  It intentionally stays thin – all heavy lifting is still
// performed by RPCClient, SSEBroker, etc. – but provides a single entry point
// developers can reason about when interacting with a remote agent.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/tools"
	"github.com/theapemachine/a2a-go/pkg/types"
)

// DefaultRPCPath is appended to the AgentCard.URL when the caller does not
// specify a fully‑qualified RPC endpoint.  It matches the recommended
// discovery convention from the spec.
const DefaultRPCPath = "/rpc"

// DefaultSSEPath is appended to the base URL when streaming updates are
// requested.
const DefaultSSEPath = "/events"

// Helper function to marshal an ID for JSON-RPC
func marshalID(v int) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// Agent encapsulates a remote A2A‑speaking agent.  It stores the published
// AgentCard for inspection and offers helper methods for the standard task
// lifecycle.  All network traffic goes through the embedded RPCClient so the
// behaviour is easily customisable by swapping the underlying *http.Client* or
// adding an AuthHeader callback.
type Agent struct {
	Card types.AgentCard

	rpcEndpoint string // fully‑qualified URL for JSON‑RPC POSTs
	sseEndpoint string // optional URL for SSE stream (if Card.Capabilities.Streaming)

	rpc service.RPCClient

	// Optional hooks – callers may set these after construction.
	AuthHeader func(*http.Request) // injects auth / tracing headers
	Logger     func(string, ...interface{})
}

// NewAgentFromCard constructs an Agent from an already‑fetched AgentCard.  No
// network requests are performed.
func NewAgentFromCard(card types.AgentCard) *Agent {
	base := strings.TrimRight(card.URL, "/")
	ag := &Agent{
		Card:        card,
		rpcEndpoint: base + DefaultRPCPath,
		sseEndpoint: base + DefaultSSEPath,
	}
	ag.rpc.Endpoint = ag.rpcEndpoint
	return ag
}

// ------------------------------ Task helpers -----------------------------------

// Send issues a tasks/send request and returns the resulting Task.
func (a *Agent) Send(ctx context.Context, params types.TaskSendParams) (*types.Task, error) {
	var task types.Task
	if err := a.call(ctx, "tasks/send", params, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// SendStream sends tasks/sendSubscribe and dispatches streaming events to the
// provided callbacks.  If the agent reports final=true the function returns
// nil.  Note: this implementation performs a best‑effort SSE parse; for
// production‑grade robustness applications may want a more sophisticated
// parser with reconnection logic.
func (a *Agent) SendStream(
	ctx context.Context,
	params types.TaskSendParams,
	onStatus func(types.TaskStatusUpdateEvent),
	onArtifact func(types.TaskArtifactUpdateEvent),
) error {
	// First perform the JSON‑RPC call but keep the HTTP response body for SSE.
	// Encode request manually because RPCClient currently hides http.Response.

	payload := service.RPCRequest{
		JSONRPC: "2.0",
		ID:      marshalID(1),
		Method:  "tasks/sendSubscribe",
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
		return fmt.Errorf("stream request failed: HTTP %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") { // comments / keep‑alive
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		// Determine event type by probing presence of fields.
		if strings.Contains(data, "\"artifact\"") {
			var evt types.TaskArtifactUpdateEvent
			if err := json.Unmarshal([]byte(data), &evt); err == nil && onArtifact != nil {
				onArtifact(evt)
			}
		} else {
			var evt types.TaskStatusUpdateEvent
			if err := json.Unmarshal([]byte(data), &evt); err == nil && onStatus != nil {
				onStatus(evt)
				if evt.Final {
					return nil
				}
			}
		}
	}
}

// Get retrieves a task.
func (a *Agent) Get(ctx context.Context, id string, historyLength int) (*types.Task, error) {
	params := struct {
		ID            string `json:"id"`
		HistoryLength int    `json:"historyLength,omitempty"`
	}{ID: id, HistoryLength: historyLength}

	var task types.Task
	if err := a.call(ctx, "tasks/get", params, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// Cancel cancels a running task.
func (a *Agent) Cancel(ctx context.Context, id string) error {
	params := struct {
		ID string `json:"id"`
	}{ID: id}
	return a.call(ctx, "tasks/cancel", params, nil)
}

// SetPush sets or updates the push‑notification config.
func (a *Agent) SetPush(ctx context.Context, cfg types.TaskPushNotificationConfig) error {
	return a.call(ctx, "tasks/pushNotification/set", cfg, nil)
}

// GetPush fetches the push‑notification config for a task.
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

// ------------------------------ helpers -------------------------------------

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

func (a *Agent) httpClient() *http.Client {
	if a.rpc.HTTP != nil {
		return a.rpc.HTTP
	}
	return http.DefaultClient
}

// authInjectingRoundTripper adds custom headers right before the request is
// sent.  Needed because RPCClient hides the underlying *http.Request*.
type authInjectingRoundTripper struct {
	base       http.RoundTripper
	injectFunc func(*http.Request)
}

func (rt authInjectingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if rt.injectFunc != nil {
		rt.injectFunc(r)
	}
	return rt.base.RoundTrip(r)
}

// ------------------------------ MCP helpers ----------------------------------

// ToMCPResource proxies to the existing helper on AgentCard.
func (a *Agent) ToMCPResource() mcp.Resource {
	return tools.ToMCPResource(a.Card)
}

// ToMCPTools converts all skills to MCP tools.
func (a *Agent) ToMCPTools() []mcp.Tool {
	out := make([]mcp.Tool, 0, len(a.Card.Skills))
	for _, s := range a.Card.Skills {
		out = append(out, tools.ToMCPTool(s))
	}
	return out
}

// ------------------------------ misc -----------------------------------------

// FetchAgentCard retrieves the published agent card from the well‑known path
// and constructs an Agent instance.  Convenience helper for quick experiments.
func FetchAgentCard(ctx context.Context, baseURL string) (*Agent, error) {
	wellKnown := strings.TrimRight(baseURL, "/") + "/.well-known/agent.json"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch agent card: HTTP %d", resp.StatusCode)
	}
	var card types.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, err
	}
	return NewAgentFromCard(card), nil
}

// ------------------------------- compile guard -------------------------------

var _ = errors.New // keep "errors" imported if file otherwise has no uses
