package catalog

import (
	"sync"

	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/types"
)

var (
	once     sync.Once
	instance *Registry
)

type Registry struct {
	agents *sync.Map
}

func NewRegistry() *Registry {
	once.Do(func() {
		instance = &Registry{
			agents: new(sync.Map),
		}
	})

	return instance
}

func (registry *Registry) AddAgent(agentCard types.AgentCard) {
	log.Info("adding agent to catalog", "name", agentCard.Name)
	registry.agents.Store(agentCard.Name, agentCard)
}

func (registry *Registry) GetAgent(name string) *types.AgentCard {
	log.Info("getting agent from catalog", "name", name)

	agent, ok := registry.agents.Load(name)

	if !ok {
		return nil
	}

	return agent.(*types.AgentCard)
}

func (registry *Registry) GetAgents() []types.AgentCard {
	log.Info("getting all agents from catalog")

	agents := make([]types.AgentCard, 0)

	registry.agents.Range(func(key, value any) bool {
		agents = append(agents, value.(types.AgentCard))
		return true
	})

	return agents
}
