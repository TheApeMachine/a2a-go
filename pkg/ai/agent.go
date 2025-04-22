package ai

import (
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v3/client"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/types"
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
	notifier   func(*types.Task)
}

/*
NewAgentFromCard constructs an Agent from an already‑fetched AgentCard.
No network requests are performed.
*/
func NewAgentFromCard(card *types.AgentCard) *Agent {
	v := viper.GetViper()

	agent := &Agent{
		card: card,
		rpc:  jsonrpc.NewRPCClient(card.URL),
	}

	agent.chatClient = provider.NewOpenAIProvider(agent.execute)

	resp, err := client.Post(
		v.GetString("catalogServer.host"),
		client.Config{
			Header: map[string]string{
				"Content-Type": "application/json",
			},
			Body: card,
		},
	)

	if err != nil {
		log.Warn("failed to register agent with catalog", "error", err)
		return agent
	}

	if resp.StatusCode() != http.StatusCreated {
		log.Warn("failed to register agent with catalog", "status", resp.StatusCode())
		return agent
	}

	log.Info("registered agent with catalog")

	return agent
}

func (a *Agent) SetNotifier(notifier func(*types.Task)) {
	a.notifier = notifier
}
