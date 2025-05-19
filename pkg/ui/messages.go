package ui

import "github.com/theapemachine/a2a-go/pkg/a2a"

// AgentSelectedMsg is emitted when an agent is chosen from the list.
type AgentSelectedMsg struct{ Agent a2a.AgentCard }

// TaskSelectedMsg is emitted when a task is chosen from the task list.
type TaskSelectedMsg struct{ Task a2a.Task }

// SendInstructionsMsg is emitted when the user submits instructions via the input area.
type SendInstructionsMsg struct{ Text string }

// AppendDetailMsg instructs the detail view to append output text.
type AppendDetailMsg struct{ Text string }
