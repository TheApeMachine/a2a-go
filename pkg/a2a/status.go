package a2a

import "time"

/*
TaskState enumerates the mutually‑exclusive states a task may be in.  The
zero value is "unknown" per the spec.
*/
type TaskState string

const (
	TaskStateSubmitted TaskState = "submitted"
	TaskStateWorking   TaskState = "working"
	TaskStateInputReq  TaskState = "input-required"
	TaskStateCompleted TaskState = "completed"
	TaskStateCanceled  TaskState = "canceled"
	TaskStateFailed    TaskState = "failed"
	TaskStateUnknown   TaskState = "unknown"
)

type TaskStatus struct {
	State     TaskState  `json:"state"`
	Message   *Message   `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}
