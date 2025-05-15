package catalog

import "fmt"

// Error types for the catalog package
type (
	// RegistrationError represents an error that occurred during agent registration
	RegistrationError struct {
		StatusCode int
		Message    string
	}

	// ConnectionError represents an error that occurred while connecting to the catalog
	ConnectionError struct {
		Message string
		Err     error
	}

	// DecodingError represents an error that occurred while decoding a response
	DecodingError struct {
		Message string
		Err     error
	}

	// NotFoundError represents an error when an agent is not found
	NotFoundError struct {
		AgentID string
	}
)

func (e *RegistrationError) Error() string {
	return fmt.Sprintf("failed to register agent: %s (status: %d)", e.Message, e.StatusCode)
}

func (e *ConnectionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to connect to catalog: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("failed to connect to catalog: %s", e.Message)
}

func (e *DecodingError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to decode catalog response: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("failed to decode catalog response: %s", e.Message)
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("agent not found: %s", e.AgentID)
}
