package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
Agent encapsulates a remote A2A‑speaking agent.  It stores the published
AgentCard for inspection and offers helper methods for the standard task
lifecycle.  All network traffic goes through the embedded RPCClient so the
behaviour is easily customisable by swapping the underlying *http.Client* or
adding an AuthHeader callback.
*/
type Agent struct {
	chatClient *provider.OpenAIProvider
	card       *types.AgentCard
	rpc        *jsonrpc.RPCClient
	AuthHeader func(*http.Request)
	Logger     func(string, ...any)
}

/*
NewAgentFromCard constructs an Agent from an already‑fetched AgentCard.
No network requests are performed.
*/
func NewAgentFromCard(card *types.AgentCard) *Agent {
	agent := &Agent{
		card: card,
		rpc:  jsonrpc.NewRPCClient(card.URL),
	}

	agent.chatClient = provider.NewOpenAIProvider(agent.execute)
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
	agent.rpc.Call(ctx, "tasks/sendSubscribe", params, &params)
	reader := bufio.NewReader(params.Reader())

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
