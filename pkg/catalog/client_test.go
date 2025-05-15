package catalog

import (
	"net/http"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

func TestNewCatalogClient(t *testing.T) {
	Convey("Given a new catalog client with a base URL", t, func() {
		client := NewCatalogClient("http://localhost:8080")

		Convey("It should have a configured fiber client", func() {
			So(client.conn, ShouldNotBeNil)
			So(client.conn.BaseURL(), ShouldEqual, "http://localhost:8080")
		})
	})
}

func TestRegister(t *testing.T) {
	Convey("Given a catalog client", t, func() {
		server := NewMockServer()
		defer server.Close()
		client := NewCatalogClient(server.URL)

		Convey("When registering a valid agent", func() {
			card := &a2a.AgentCard{
				Name:    "test-agent",
				URL:     "http://test-agent.example.com",
				Version: "1.0.0",
				Capabilities: a2a.AgentCapabilities{
					Streaming: true,
				},
			}

			err := client.Register(card)

			Convey("Then the registration should succeed", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When registering with invalid data", func() {
			server.customRegister = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			}

			card := &a2a.AgentCard{
				Name:    "test-agent",
				URL:     "http://test-agent.example.com",
				Version: "1.0.0",
				Capabilities: a2a.AgentCapabilities{
					Streaming: true,
				},
			}

			err := client.Register(card)

			Convey("Then a RegistrationError should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err, ShouldHaveSameTypeAs, &RegistrationError{})
			})
		})

		Convey("When the server is unreachable", func() {
			server.Close() // Close the server to simulate unreachable

			card := &a2a.AgentCard{
				Name:    "test-agent",
				URL:     "http://test-agent.example.com",
				Version: "1.0.0",
				Capabilities: a2a.AgentCapabilities{
					Streaming: true,
				},
			}

			err := client.Register(card)

			Convey("Then a ConnectionError should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err, ShouldHaveSameTypeAs, &ConnectionError{})
			})
		})
	})
}

func TestGetAgents(t *testing.T) {
	Convey("Given a catalog client", t, func() {
		server := NewMockServer()
		defer server.Close()
		client := NewCatalogClient(server.URL)

		Convey("When getting agents", func() {
			agents, err := client.GetAgents()

			Convey("Then the agents should be returned", func() {
				So(err, ShouldBeNil)
				So(agents, ShouldNotBeNil)
			})
		})

		Convey("When the server returns an error", func() {
			server.customGetAgents = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}

			agents, err := client.GetAgents()

			Convey("Then a ConnectionError should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err, ShouldHaveSameTypeAs, &ConnectionError{})
				So(agents, ShouldBeNil)
			})
		})

		Convey("When the response is invalid JSON", func() {
			server.customGetAgents = func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("invalid json"))
			}

			agents, err := client.GetAgents()

			Convey("Then a DecodingError should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err, ShouldHaveSameTypeAs, &DecodingError{})
				So(agents, ShouldBeNil)
			})
		})
	})
}

func TestGetAgent(t *testing.T) {
	Convey("Given a catalog client", t, func() {
		server := NewMockServer()
		defer server.Close()
		client := NewCatalogClient(server.URL)

		Convey("When getting an existing agent", func() {
			// First register an agent
			card := &a2a.AgentCard{
				Name:    "test-agent",
				URL:     "http://test-agent.example.com",
				Version: "1.0.0",
				Capabilities: a2a.AgentCapabilities{
					Streaming: true,
				},
			}
			err := client.Register(card)
			So(err, ShouldBeNil)

			agent, err := client.GetAgent("test-agent")

			Convey("Then the agent should be returned", func() {
				So(err, ShouldBeNil)
				So(agent, ShouldNotBeNil)
				So(agent.Name, ShouldEqual, "test-agent")
			})
		})

		Convey("When getting a non-existent agent", func() {
			agent, err := client.GetAgent("non-existent")

			Convey("Then a NotFoundError should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err, ShouldHaveSameTypeAs, &NotFoundError{})
				So(agent, ShouldBeNil)
			})
		})

		Convey("When the server returns an error", func() {
			server.customGetAgent = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}

			agent, err := client.GetAgent("test-agent")

			Convey("Then a ConnectionError should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err, ShouldHaveSameTypeAs, &ConnectionError{})
				So(agent, ShouldBeNil)
			})
		})

		Convey("When the response is invalid JSON", func() {
			server.customGetAgent = func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte("invalid json"))
			}

			agent, err := client.GetAgent("test-agent")

			Convey("Then a DecodingError should be returned", func() {
				So(err, ShouldNotBeNil)
				So(err, ShouldHaveSameTypeAs, &DecodingError{})
				So(agent, ShouldBeNil)
			})
		})
	})
}
