package ui

import "github.com/theapemachine/a2a-go/pkg/a2a"

// Message types for internal events
type fetchAgentsMsg struct{ agents []a2a.AgentCard }
type fetchAgentDetailMsg struct{ agent a2a.AgentCard }
type fetchTasksMsg struct{ tasks []a2a.Task }
type fetchTaskDetailMsg struct{ task a2a.Task }
type errorMsg struct{ err error }
type streamEventMsg struct{ event any }
type LogMsg struct{ Log string }
type TaskMessage struct {
	Tasks []a2a.Task
} 