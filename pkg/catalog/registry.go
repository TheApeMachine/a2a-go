package catalog

import (
	"sync"

	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

type Registry struct {
	agents *sync.Map
}

func NewRegistry() *Registry {
	return &Registry{
		agents: new(sync.Map),
	}
}

func (registry *Registry) AddAgent(agentCard a2a.AgentCard) {
	log.Info("adding agent to catalog", "name", agentCard.Name)
	registry.agents.Store(agentCard.Name, agentCard)
}

func (registry *Registry) GetAgent(name string) a2a.AgentCard {
	log.Info("getting agent from catalog", "name", name)

	agent, ok := registry.agents.Load(name)

	if !ok {
		return a2a.AgentCard{}
	}

	// Create a copy of the agent card
	agentCard := agent.(a2a.AgentCard)
	return agentCard
}

func (registry *Registry) GetAgents() []a2a.AgentCard {
	log.Info("getting all agents from catalog")

	agents := make([]a2a.AgentCard, 0)

	registry.agents.Range(func(key, value any) bool {
		agents = append(agents, value.(a2a.AgentCard))
		return true
	})

	return agents
}
