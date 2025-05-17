// Kubrick is a tool for managing Kubernetes clusters
package types

import "errors"

type State uint

const (
	StateInitialized State = iota
	StateCreated
	StateRunning
	StateUpdated
	StateErrored
	StateCanceled
	StateClosed
	StateSuccess
	StateFailure
)

// Common errors
var (
	ErrInvalidScreen = errors.New("invalid screen index")
)