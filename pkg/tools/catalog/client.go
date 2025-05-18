package catalog

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/log"
	fiberClient "github.com/gofiber/fiber/v3/client"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

/*
CatalogClient connects to the A2A Agent Catalog service, so it can retrieve all
the available agents.
*/
type CatalogClient struct {
	baseURL string
	conn    *fiberClient.Client
}

type CatalogClientOption func(*CatalogClient)

/*
NewCatalogClient creates a new catalog client with the given base URL.
*/
func NewCatalogClient(baseURL string, opts ...CatalogClientOption) *CatalogClient {
	client := &CatalogClient{
		baseURL: baseURL,
		conn:    fiberClient.New().SetBaseURL(baseURL).SetTimeout(5 * time.Second),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func (client *CatalogClient) Register(card *a2a.AgentCard) error {
	var (
		resp *fiberClient.Response
		err  error
	)

	if resp, err = client.conn.Post("/agent", fiberClient.Config{
		Header: map[string]string{
			"Content-Type": "application/json",
		},
		Body: card,
	}); err != nil {
		log.Error("failed to register agent", "error", err)
		return &ConnectionError{Message: "registration failed", Err: err}
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusBadRequest {
		log.Error("failed to register agent", "error", resp.Status())
		return &RegistrationError{
			StatusCode: resp.StatusCode(),
			Message:    resp.Status(),
		}
	}

	return nil
}

// GetAgents retrieves all agent cards from the catalog.
func (client *CatalogClient) GetAgents() ([]a2a.AgentCard, error) {
	resp, err := client.conn.Get("/.well-known/catalog.json")

	if err != nil {
		return nil, &ConnectionError{Message: "failed to get agents", Err: err}
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusBadRequest {
		return nil, &ConnectionError{
			Message: fmt.Sprintf("catalog returned non-OK status: %d", resp.StatusCode()),
		}
	}

	var agents []a2a.AgentCard

	if err = resp.JSON(&agents); err != nil {
		return nil, &DecodingError{Message: "failed to decode agents list", Err: err}
	}

	return agents, nil
}

// GetAgent retrieves a specific agent card by ID from the catalog.
func (client *CatalogClient) GetAgent(id string) (*a2a.AgentCard, error) {
	resp, err := client.conn.Get(fmt.Sprintf("/agent/%s", url.PathEscape(id)))

	if err != nil {
		return nil, &ConnectionError{Message: "failed to get agent", Err: err}
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, &NotFoundError{AgentID: id}
	}

	if resp.StatusCode() < http.StatusOK || resp.StatusCode() >= http.StatusBadRequest {
		return nil, &ConnectionError{
			Message: fmt.Sprintf("catalog returned non-OK status: %d", resp.StatusCode()),
		}
	}

	var agent a2a.AgentCard

	if err = resp.JSON(&agent); err != nil {
		return nil, &DecodingError{Message: "failed to decode agent", Err: err}
	}

	return &agent, nil
}

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

