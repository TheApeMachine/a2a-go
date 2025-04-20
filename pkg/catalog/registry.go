package catalog

import (
	"sync"

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

func (registry *Registry) AddAgent(agent types.IdentifiableTaskManager) {
	registry.agents.Store(agent.Card().Name, agent)
}

func (registry *Registry) GetAgent(name string) types.IdentifiableTaskManager {
	agent, ok := registry.agents.Load(name)

	if !ok {
		return nil
	}

	return agent.(types.IdentifiableTaskManager)
}

func (registry *Registry) GetAgents() []types.IdentifiableTaskManager {
	agents := make([]types.IdentifiableTaskManager, 0)
	registry.agents.Range(func(key, value any) bool {
		agents = append(agents, value.(types.IdentifiableTaskManager))
		return true
	})

	return agents
}
