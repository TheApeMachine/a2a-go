package ai

import (
	"github.com/theapemachine/a2a-go/pkg/types"
)

/*
Card retrieves the published agent card from the well‑known path
and constructs an Agent instance.  Convenience helper for quick experiments.
*/
func (agent *Agent) Card() types.AgentCard {
	return agent.card
}
