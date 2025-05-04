package ai

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/auth"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/errors"
)

/*
Agent encapsulates a remote A2A‑speaking agent.  It stores the published
AgentCard for inspection and offers helper methods for the standard task
lifecycle.  All network traffic goes through the embedded RPCClient so the
behaviour is easily customisable by swapping the underlying *http.Client* or
adding an AuthHeader callback.
*/
type Agent struct {
	*TaskManager
	card          *a2a.AgentCard
	authService   *auth.Service
	catalogClient *catalog.CatalogClient
}

type AgentOption func(*Agent)

/*
NewAgentFromCard constructs an Agent from an already‑fetched AgentCard.
The common way to define the Agent Cards for various use-cases is to use
the `cmd/cfg/config.yml` which will be automatically copied from the
embedded file to `~/.a2a-go/config.yml` on first run.
*/
func NewAgentFromCard(
	card *a2a.AgentCard,
	options ...AgentOption,
) (*Agent, error) {
	agent := &Agent{
		card: card,
	}

	for _, option := range options {
		option(agent)
	}

	if agent.TaskManager == nil {
		return nil, errors.NewError(errors.ErrMissingTaskManager{})
	}

	if agent.catalogClient == nil {
		return nil, errors.NewError(errors.ErrMissingCatalog{})
	}

	attempt := 0

	for attempt < 10 {
		if err := agent.catalogClient.Register(agent.card); err != nil {
			log.Error("failed to register agent", "error", err)
			attempt++
			time.Sleep(time.Second * time.Duration(attempt*2))
		} else {
			log.Info("agent registered", "name", agent.card.Name)
			break
		}
	}

	return agent, nil
}

func (agent *Agent) Name() string {
	return agent.card.Name
}

func (agent *Agent) Card() *a2a.AgentCard {
	return agent.card
}

func WithTaskManager(taskManager *TaskManager) AgentOption {
	return func(agent *Agent) {
		agent.TaskManager = taskManager
	}
}

func WithCatalogClient(catalogClient *catalog.CatalogClient) AgentOption {
	return func(agent *Agent) {
		agent.catalogClient = catalogClient
	}
}

func WithAuthService(authService *auth.Service) AgentOption {
	return func(agent *Agent) {
		agent.authService = authService
	}
}
