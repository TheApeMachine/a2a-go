package ui

import (
	"fmt"

	"github.com/theapemachine/a2a-go/pkg/a2a"
)

// Item implementations for the lists
type agentItem struct {
	agent a2a.AgentCard
}

func (i agentItem) Title() string {
	return i.agent.Name
}

func (i agentItem) Description() string {
	if i.agent.Description != nil {
		return *i.agent.Description
	}
	return "No description available"
}

func (i agentItem) FilterValue() string {
	return i.agent.Name
}

type taskItem struct {
	task a2a.Task
}

func (i taskItem) Title() string {
	return i.task.ID
}

func (i taskItem) Description() string {
	// Safe access to task fields
	state := string(i.task.Status.State)
	if state == "" {
		state = "unknown"
	}
	return fmt.Sprintf("Status: %s", state)
}

func (i taskItem) FilterValue() string {
	return i.task.ID
}
