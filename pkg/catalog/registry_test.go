package catalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

type MockServer struct {
	*httptest.Server
	registry *Registry
	// Custom handlers for testing
	customRegister  http.HandlerFunc
	customGetAgents http.HandlerFunc
	customGetAgent  http.HandlerFunc
}

func NewMockServer() *MockServer {
	registry := NewRegistry()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	mock := &MockServer{
		Server:   server,
		registry: registry,
	}

	mux.HandleFunc("/agent", mock.handleRegister)
	mux.HandleFunc("/.well-known/catalog.json", mock.handleGetAgents)
	mux.HandleFunc("/agent/", mock.handleGetAgent)

	return mock
}

func (s *MockServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if s.customRegister != nil {
		s.customRegister(w, r)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var card a2a.AgentCard
	if err := json.NewDecoder(r.Body).Decode(&card); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.registry.AddAgent(card)
	w.WriteHeader(http.StatusOK)
}

func (s *MockServer) handleGetAgents(w http.ResponseWriter, r *http.Request) {
	if s.customGetAgents != nil {
		s.customGetAgents(w, r)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	agents := s.registry.GetAgents()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(agents)
}

func (s *MockServer) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	if s.customGetAgent != nil {
		s.customGetAgent(w, r)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract agent ID from URL path
	id := r.URL.Path[len("/agent/"):]
	agent := s.registry.GetAgent(id)

	if agent.Name == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(agent)
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	Convey("Given a new registry", t, func() {
		Convey("Should have an empty map of agents", func() {
			So(registry.agents, ShouldResemble, &sync.Map{})
		})
	})
}

func TestAddAgent(t *testing.T) {
	registry := NewRegistry()

	Convey("Given a new registry", t, func() {
		Convey("And an agent card", func() {
			name := "test-agent"
			agentCard := a2a.AgentCard{
				Name: name,
			}

			Convey("When an agent is added", func() {
				registry.AddAgent(agentCard)

				Convey("It should have a registered agent", func() {
					loaded, ok := registry.agents.Load(name)

					So(ok, ShouldBeTrue)
					So(loaded.(a2a.AgentCard).Name, ShouldEqual, name)
				})
			})
		})
	})
}

func TestRegistryGetAgents(t *testing.T) {
	registry := NewRegistry()

	Convey("Given a registry with agents", t, func() {
		agent1 := a2a.AgentCard{Name: "agent1"}
		agent2 := a2a.AgentCard{Name: "agent2"}
		registry.AddAgent(agent1)
		registry.AddAgent(agent2)

		Convey("When getting all agents", func() {
			agents := registry.GetAgents()

			Convey("It should return all registered agents", func() {
				So(len(agents), ShouldEqual, 2)

				// Check that both agents are in the result without assuming order
				foundAgent1 := false
				foundAgent2 := false

				for _, agent := range agents {
					if agent.Name == "agent1" {
						foundAgent1 = true
					}
					if agent.Name == "agent2" {
						foundAgent2 = true
					}
				}

				So(foundAgent1, ShouldBeTrue)
				So(foundAgent2, ShouldBeTrue)
			})
		})
	})
}

func TestRegistryGetAgent(t *testing.T) {
	registry := NewRegistry()

	Convey("Given a registry with an agent", t, func() {
		agentCard := a2a.AgentCard{Name: "test-agent"}
		registry.AddAgent(agentCard)

		Convey("When getting an existing agent", func() {
			agent := registry.GetAgent("test-agent")

			Convey("It should return the agent", func() {
				So(agent.Name, ShouldEqual, "test-agent")
			})
		})

		Convey("When getting a non-existent agent", func() {
			agent := registry.GetAgent("non-existent")

			Convey("It should return an empty agent card", func() {
				So(agent.Name, ShouldBeEmpty)
			})
		})
	})
}
