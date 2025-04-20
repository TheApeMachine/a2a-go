package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/tasks"
	"github.com/theapemachine/a2a-go/pkg/types"
)

var (
	portFlag      int
	hostFlag      string
	agentNameFlag string
	mcpModeFlag   bool

	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Run A2A and MCP services",
		Long:  longServe,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Serve an A2A agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serveAgent()
		},
	}

	mcpCmd = &cobra.Command{
		Use:   "mcp",
		Short: "Serve an MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serveMCP()
		},
	}
)

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.AddCommand(agentCmd)
	serveCmd.AddCommand(mcpCmd)

	serveCmd.PersistentFlags().IntVarP(&portFlag, "port", "p", 3210, "Port to serve on")
	serveCmd.PersistentFlags().StringVarP(&hostFlag, "host", "H", "0.0.0.0", "Host address to bind to")

	agentCmd.Flags().StringVarP(&agentNameFlag, "name", "n", "A2A-Go Agent", "Name for the agent")
	mcpCmd.Flags().BoolVar(&mcpModeFlag, "with-agent", false, "Serve with a builtin agent")
}

func serveAgent() error {
	url := fmt.Sprintf("http://%s:%d", hostFlag, portFlag)
	log.Printf("Starting A2A agent server at %s", url)

	// Read agent configuration from viper
	card := types.AgentCard{
		Name:    agentNameFlag,
		URL:     url,
		Version: viper.GetString("agent.version"),
		Capabilities: types.AgentCapabilities{
			Streaming:              viper.GetBool("agent.capabilities.streaming"),
			PushNotifications:      viper.GetBool("agent.capabilities.pushNotifications"),
			StateTransitionHistory: viper.GetBool("agent.capabilities.stateTransitionHistory"),
		},
		Skills: []types.AgentSkill{{ID: "echo", Name: "Echo"}},
	}

	server := service.NewA2AServer(card, tasks.NewEchoTaskManager(nil))
	mux := http.NewServeMux()

	for path, handler := range server.Handlers() {
		mux.Handle(path, handler)
	}

	// Add .well-known/agent.json endpoint
	mux.HandleFunc("/.well-known/agent.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(card); err != nil {
			log.Printf("Error writing agent card: %v", err)
		}
	})

	// Add a health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Handle graceful shutdown
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", hostFlag, portFlag),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("A2A agent server listening on %s:%d", hostFlag, portFlag)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down A2A agent server...")

	// Create a context with a timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		return err
	}

	log.Println("A2A agent server stopped")
	return nil
}

func serveMCP() error {
	url := fmt.Sprintf("http://%s:%d", hostFlag, portFlag)
	log.Printf("Starting MCP server at %s", url)

	mux := http.NewServeMux()

	// If configured to serve with an agent
	if mcpModeFlag {
		card := types.AgentCard{
			Name:    "Embedded Agent",
			URL:     url + "/agent",
			Version: viper.GetString("agent.version"),
			Capabilities: types.AgentCapabilities{
				Streaming:              viper.GetBool("agent.capabilities.streaming"),
				PushNotifications:      viper.GetBool("agent.capabilities.pushNotifications"),
				StateTransitionHistory: viper.GetBool("agent.capabilities.stateTransitionHistory"),
			},
			Skills: []types.AgentSkill{{ID: "echo", Name: "Echo"}},
		}

		// Create and mount the agent
		agentServer := service.NewA2AServer(card, tasks.NewEchoTaskManager(nil))
		for path, handler := range agentServer.Handlers() {
			mux.Handle("/agent"+path, handler)
		}

		// Agent discovery endpoint
		mux.HandleFunc("/agent/.well-known/agent.json", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(card); err != nil {
				log.Printf("Error writing agent card: %v", err)
			}
		})

		log.Printf("MCP server includes embedded agent at %s/agent", url)
	}

	// Add MCP API endpoints here
	// TODO: Implement full MCP API endpoints

	// Add /mcp/tools endpoint for MCP
	mux.HandleFunc("/mcp/tools", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		tools := []map[string]interface{}{
			{
				"id":          "echo",
				"name":        "Echo Tool",
				"description": "Echoes the input back to the caller",
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"text": map[string]interface{}{
							"type":        "string",
							"description": "Text to echo",
						},
					},
					"required": []string{"text"},
				},
			},
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tools": tools,
		})
	})

	// Add a basic health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	// Handle graceful shutdown
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", hostFlag, portFlag),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("MCP server listening on %s:%d", hostFlag, portFlag)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down MCP server...")

	// Create a context with a timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		return err
	}

	log.Println("MCP server stopped")
	return nil
}

var longServe = `
Serve an A2A agent or MCP server with various configurations.

Examples:
  # Serve an A2A agent on port 8080
  caramba serve agent --port 8080

  # Serve an MCP server on port 3000
  caramba serve mcp --port 3000

  # Serve an MCP server with an embedded agent
  caramba serve mcp --with-agent --port 3000
`
