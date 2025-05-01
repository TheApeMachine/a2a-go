package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/types"
)

// AgentCard is an alias to types.AgentCard for simplicity in our API.
type AgentCard = types.AgentCard

// CatalogClient provides functionality to interact with the agent catalog service.
type CatalogClient struct {
	baseURL  string
	httpClient *http.Client
}

// NewCatalogClient creates a new catalog client with the given base URL.
func NewCatalogClient(baseURL string) *CatalogClient {
	return &CatalogClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetAgents retrieves all agent cards from the catalog.
func (c *CatalogClient) GetAgents() ([]AgentCard, error) {
	// Request the catalog agents endpoint
	url := fmt.Sprintf("%s/.well-known/catalog.json", c.baseURL)
	
	log.Debug("Fetching agents from catalog", "url", url)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to catalog: %w", err)
	}
	defer resp.Body.Close()
	
	// Check for non-200 responses
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("catalog returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}
	
	// Parse the response
	var agents []AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, fmt.Errorf("failed to decode catalog response: %w", err)
	}
	
	log.Debug("Retrieved agents from catalog", "count", len(agents))
	return agents, nil
}

// GetAgent retrieves a specific agent card by ID from the catalog.
func (c *CatalogClient) GetAgent(id string) (*AgentCard, error) {
	// Request the specific agent
	url := fmt.Sprintf("%s/agent/%s", c.baseURL, id)
	
	log.Debug("Fetching agent from catalog", "agentID", id, "url", url)
	
	// Using POST as specified in the A2A spec
	resp, err := c.httpClient.Post(url, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to catalog: %w", err)
	}
	defer resp.Body.Close()
	
	// Check for non-200 responses
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("agent not found: %s", id)
		}
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("catalog returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}
	
	// Parse the response
	var agent AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return nil, fmt.Errorf("failed to decode agent response: %w", err)
	}
	
	return &agent, nil
} 