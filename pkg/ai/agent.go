package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v3/client"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/auth"
	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/push"
	"github.com/theapemachine/a2a-go/pkg/registry"
	"github.com/theapemachine/a2a-go/pkg/stores/s3"
	"github.com/theapemachine/a2a-go/pkg/tools/docker"
	"github.com/theapemachine/a2a-go/pkg/types"
)

/*
Agent encapsulates a remote A2A‑speaking agent.  It stores the published
AgentCard for inspection and offers helper methods for the standard task
lifecycle.  All network traffic goes through the embedded RPCClient so the
behaviour is easily customisable by swapping the underlying *http.Client* or
adding an AuthHeader callback.
*/
type Agent struct {
	card          types.AgentCard
	rpc           *jsonrpc.RPCClient
	chatClient    *provider.OpenAIProvider
	toolExecutors map[string]registry.ToolExecutorFunc
	ssePublisher  types.SSEPublisher
	pushService   *push.Service
	authService   *auth.Service
	notifier      func(*types.Task)
	AuthHeader    func(*http.Request)
	Logger        func(string, ...any)
	taskStore     *s3.Conn
}

/*
NewAgentFromCard constructs an Agent from an already‑fetched AgentCard.
No network requests are performed.
*/
func NewAgentFromCard(card *types.AgentCard) *Agent {
	v := viper.GetViper()

	agent := &Agent{
		card:          *card,
		rpc:           jsonrpc.NewRPCClient(card.URL),
		taskStore:     s3.NewConn(),                               // Initialize taskStore
		toolExecutors: make(map[string]registry.ToolExecutorFunc), // Initialize tool registry
		pushService:   push.NewService(),                          // Initialize pushService
		authService:   auth.NewService(),                          // Initialize authService
	}

	// Initialize chatClient after agent is created
	agent.chatClient = provider.NewOpenAIProvider(agent.executeTool)

	// Register known tool executors based on skills in the card
	agent.registerToolExecutors()

	// Get the catalog URL - try environment variable first, then config
	catalogURL := os.Getenv("CATALOG_URL")
	if catalogURL == "" {
		catalogURL = v.GetString("server.catalogServer.host")
	}

	// Ensure catalogURL has the correct path for agent registration
	if catalogURL != "" {
		// Make sure the URL doesn't end with a trailing slash
		if catalogURL[len(catalogURL)-1] == '/' {
			catalogURL = catalogURL[:len(catalogURL)-1]
		}

		// Append the agent endpoint if it's not already there
		if !strings.HasSuffix(catalogURL, "/agent") {
			catalogURL += "/agent"
		}

		// Catalog registration
		resp, err := client.Post(
			catalogURL,
			client.Config{
				Header: map[string]string{
					"Content-Type": "application/json",
				},
				Body: card, // Should marshal the card to JSON
			},
		)

		if err != nil {
			log.Warn("failed to register agent with catalog", "error", err, "url", catalogURL)
			// Don't return early, agent might still be usable directly
		} else if resp.StatusCode() != http.StatusCreated {
			log.Warn("failed to register agent with catalog", "status", resp.StatusCode(), "body", string(resp.Body()), "url", catalogURL)
			// Don't return early
		} else {
			log.Info("registered agent with catalog", "url", catalogURL)
		}
	} else {
		log.Warn("no catalog URL configured, agent will not be registered")
	}

	return agent
}

func (a *Agent) SetNotifier(notifier func(*types.Task)) {
	a.notifier = notifier
}

// Card returns the agent's card.
func (a *Agent) Card() *types.AgentCard {
	return &a.card
}

// ID returns the agent's ID from its card.
func (a *Agent) ID() string {
	if a.card.Name != "" {
		return a.card.Name // Placeholder: Use Name if ID field doesn't exist
	}
	return "unknown-agent"
}

// registerToolExecutors iterates through agent skills and registers known executors.
func (agent *Agent) registerToolExecutors() {
	for _, skill := range agent.card.Skills {
		// Get the full tool definition from the registry
		toolDef, found := registry.GetToolDefinition(skill.ID)
		if found {
			if toolDef.Executor != nil {
				// Register the executor using the ToolName from the definition
				agent.toolExecutors[toolDef.ToolName] = toolDef.Executor
				log.Info("Registered tool executor", "toolName", toolDef.ToolName, "skillID", skill.ID)
			} else {
				log.Warn("Tool definition found, but no executor function provided", "toolName", toolDef.ToolName, "skillID", skill.ID)
			}
		} else {
			// This skill doesn't map to a known tool definition
			log.Debug("No tool definition found for skill", "skillID", skill.ID, "skillName", skill.Name)

			// For the "development" skill, register our built-in docker terminal executor
			if skill.ID == "development" {
				// Register our Docker terminal executor as a fallback
				log.Info("Registering built-in docker terminal executor for development skill")
				agent.toolExecutors["terminal"] = agent.executeDockerTerminal
			}
		}
	}
}

// executeDockerTerminal is the ToolExecutorFunc for the docker terminal skill.
// Note: This method is currently not directly used but is kept as a reference
// implementation for future tool executors and for documentation purposes.
func (agent *Agent) executeDockerTerminal(ctx context.Context, args map[string]any) (string, error) {
	log.Info("Executing docker terminal tool", "args", args)

	// --- Argument Parsing ---
	cmd, ok := args["command"].(string)
	if !ok || cmd == "" {
		return "", fmt.Errorf("missing or invalid 'command' argument")
	}

	// Use agent name as container name convention?
	containerName := agent.card.Name + "-dev-env"

	// Optional arguments with defaults
	// TODO: Define these args in the skill schema (mcp_bridge.go)
	// imageName, _ := args["imageName"].(string)
	// if imageName == "" {
	// 	imageName = "a2a-go" // Default image name
	// }

	// --- Execution Logic (using tools/docker) ---
	env, err := docker.NewEnvironment()
	if err != nil {
		log.Error("Failed to create docker environment", "error", err)
		return "", fmt.Errorf("failed to create docker environment: %w", err)
	}

	// Execute the command in the container
	// The Exec function handles container creation/finding implicitly
	result, err := env.Exec(ctx, cmd, containerName)
	if err != nil {
		// Don't return error directly, format output as expected
		log.Error("Docker exec failed", "command", cmd, "error", err)
		// Combine stdout/stderr for the result string
		output := result.Stdout.String() + "\n" + result.Stderr.String()
		return fmt.Sprintf("Execution failed: %s\nOutput:\n%s", err, output), nil
		// Note: Returning the error in the string might be better for the LLM
	}

	// --- Formatting Result ---
	stdout := result.Stdout.String()
	stderr := result.Stderr.String()

	// Combine stdout and stderr for the final result string
	output := stdout
	if stderr != "" {
		output += "\nstderr:\n" + stderr
	}

	log.Info("Docker terminal execution successful", "command", cmd, "stdout_len", len(stdout), "stderr_len", len(stderr))
	return output, nil
}

// executeTool is the central dispatcher for tool calls based on the registered executors.
// This method now matches the signature required by provider.ToolExecutor.
func (agent *Agent) executeTool(ctx context.Context, tool *registry.ToolDescriptor, args map[string]any) (string, error) {
	toolName := tool.ToolName // Get the name from the ToolDescriptor object
	log.Info("Received tool call request", "toolName", toolName, "args", args)

	executor, found := agent.toolExecutors[toolName]
	if !found {
		log.Warn("Executor not found for tool", "toolName", toolName)
		return fmt.Sprintf("Error: Agent does not have an implementation for tool '%s'", toolName), nil
	}

	// Execute the registered function (which has the registry.ToolExecutorFunc signature)
	result, err := executor(ctx, args)
	if err != nil {
		log.Error("Tool execution failed", "toolName", toolName, "error", err)
		return fmt.Sprintf("Error executing tool '%s': %v", toolName, err), nil
	}

	log.Info("Tool execution successful", "toolName", toolName)
	return result, nil
}

// SetSSEPublisher sets the SSE publisher for real-time task updates
func (a *Agent) SetSSEPublisher(publisher types.SSEPublisher) {
	a.ssePublisher = publisher
}

// --- IdentifiableTaskManager Interface Implementation ---

// SendTask handles a non-streaming task request.
func (agent *Agent) SendTask(
	ctx context.Context,
	params types.Task,
) (types.Task, *errors.RpcError) {
	log.Info("task received", "agent", agent.card.Name, "task", params.ID)

	// Store the task early so we could interrupt it, and continue at
	// some point in the future.
	task, err := agent.storeTask(ctx, params)
	if err != nil {
		return types.Task{}, err
	}

	// Convert skills into tools (MCPClient format expected by provider).
	tools := agent.card.Tools()

	// Send the task to the LLM.
	llmErr := agent.chatClient.Complete(ctx, &params, &tools) // Only returns error
	if llmErr != nil {
		log.Error("LLM completion failed", "taskID", task.ID, "error", llmErr)
		task.ToState(types.TaskStateFailed, llmErr.Error())
		agent.storeTask(ctx, task)

		// Broadcast task status update if SSE notifier is configured
		if agent.ssePublisher != nil {
			statusEvent := types.TaskStatusUpdateEvent{
				ID:     task.ID,
				Status: task.Status,
				Final:  true, // This is a final update since it failed
			}
			if err := agent.ssePublisher.BroadcastToTask(task.ID, statusEvent); err != nil {
				log.Error("Failed to broadcast task status update", "taskID", task.ID, "error", err)
			}
		}

		return task, errors.ErrInternal.WithMessagef("LLM interaction failed: %v", llmErr)
	}

	// The task object (`params`) should have been updated by the provider
	// including the final assistant message in its history.
	task = params // Update local task variable with the potentially modified one
	task.ToState(types.TaskStateCompleted, "Task completed successfully")

	// Store the task again, so it updates to the latest state.
	task, err = agent.storeTask(ctx, task)
	if err != nil {
		log.Error("failed to store final task state", "taskID", task.ID, "error", err)
	}

	// Broadcast task completion status if SSE notifier is configured
	if agent.ssePublisher != nil {
		statusEvent := types.TaskStatusUpdateEvent{
			ID:     task.ID,
			Status: task.Status,
			Final:  true, // This is a final update since it completed
		}
		if err := agent.ssePublisher.BroadcastToTask(task.ID, statusEvent); err != nil {
			log.Error("Failed to broadcast task completion", "taskID", task.ID, "error", err)
		}
	}

	return task, nil
}

// StreamTask handles a streaming task request.
func (agent *Agent) StreamTask(
	ctx context.Context,
	params types.Task,
) (types.Task, *errors.RpcError) {
	log.Info("task received for streaming", "agent", agent.card.Name, "task", params.ID)

	// Store the task early so we could interrupt it, and continue at
	// some point in the future.
	task, err := agent.storeTask(ctx, params)
	if err != nil {
		return types.Task{}, err
	}

	// Notify clients that the task is starting
	if agent.ssePublisher != nil {
		statusEvent := types.TaskStatusUpdateEvent{
			ID:     task.ID,
			Status: task.Status,
			Final:  false,
		}
		if err := agent.ssePublisher.BroadcastToTask(task.ID, statusEvent); err != nil {
			log.Error("Failed to broadcast task start status", "taskID", task.ID, "error", err)
		}
	}

	// Convert skills into tools.
	tools := agent.card.Tools()

	// Create a notifier that broadcasts updates via SSE
	taskNotifier := func(updatedTask *types.Task) {
		agent.notifier(updatedTask) // Call the original notifier

		// Also broadcast updates via SSE if available
		if agent.ssePublisher != nil {
			// Send status update
			statusEvent := types.TaskStatusUpdateEvent{
				ID:     updatedTask.ID,
				Status: updatedTask.Status,
				Final: updatedTask.Status.State == types.TaskStateCompleted ||
					updatedTask.Status.State == types.TaskStateFailed ||
					updatedTask.Status.State == types.TaskStateCanceled,
			}

			if err := agent.ssePublisher.BroadcastToTask(updatedTask.ID, statusEvent); err != nil {
				log.Error("Failed to broadcast status update", "taskID", updatedTask.ID, "error", err)
			}

			// If there are new artifacts, broadcast them too
			if len(updatedTask.Artifacts) > 0 {
				latestArtifact := updatedTask.Artifacts[len(updatedTask.Artifacts)-1]
				artifactEvent := types.TaskArtifactUpdateEvent{
					ID:       updatedTask.ID,
					Artifact: latestArtifact,
				}

				if err := agent.ssePublisher.BroadcastToTask(updatedTask.ID, artifactEvent); err != nil {
					log.Error("Failed to broadcast artifact update", "taskID", updatedTask.ID, "error", err)
				}
			}
		}
	}

	// Send the task to the LLM with streaming.
	go func() {
		ctxBg := context.Background()         // Use background context for the goroutine
		streamErr := agent.chatClient.Stream( // Only returns error
			ctxBg, &params, &tools, taskNotifier,
		)

		if streamErr != nil {
			log.Error("LLM streaming failed", "taskID", params.ID, "error", streamErr)
			// The provider should have called the notifier with the failed state already.
			// We still need to store the final state.
			task = params // Update local task var with the latest state from params
			_, storeErr := agent.storeTask(ctxBg, task)
			if storeErr != nil {
				log.Error("failed to store final task state after stream error", "taskID", task.ID, "error", storeErr)
			}

			// Send final status update via SSE
			if agent.ssePublisher != nil {
				statusEvent := types.TaskStatusUpdateEvent{
					ID:     task.ID,
					Status: task.Status,
					Final:  true,
				}
				if err := agent.ssePublisher.BroadcastToTask(task.ID, statusEvent); err != nil {
					log.Error("Failed to broadcast final error status", "taskID", task.ID, "error", err)
				}
			}
			return
		}

		// If stream finished without error, provider should have set Completed state via notifier.
		// Store the final successful state.
		task = params // Update local task var
		_, storeErr := agent.storeTask(ctxBg, task)
		if storeErr != nil {
			log.Error("failed to store final task state after stream success", "taskID", task.ID, "error", storeErr)
		}

		// Send final status update via SSE
		if agent.ssePublisher != nil {
			statusEvent := types.TaskStatusUpdateEvent{
				ID:     task.ID,
				Status: task.Status,
				Final:  true,
			}
			if err := agent.ssePublisher.BroadcastToTask(task.ID, statusEvent); err != nil {
				log.Error("Failed to broadcast final completion status", "taskID", task.ID, "error", err)
			}

			// Close the task broker since we're done
			agent.ssePublisher.CloseTaskBroker(task.ID)
		}

		log.Info("StreamTask processing finished", "taskID", task.ID)
	}()

	// Return the initial task state immediately for streaming RPC
	log.Info("StreamTask initial response sent", "taskID", task.ID)
	return task, nil
}

// GetTask retrieves the current state of a task.
func (agent *Agent) GetTask(
	ctx context.Context,
	id string,
	historyLength int,
) (types.Task, *errors.RpcError) {
	task, err := agent.loadTask(ctx, id)
	if err != nil {
		// Assuming loadTask returns RpcError
		return types.Task{}, err
	}

	// Get the last N messages.
	if historyLength > 0 && len(task.History) > historyLength {
		task.History = task.History[len(task.History)-historyLength:]
	}

	return task, nil
}

// CancelTask attempts to cancel an ongoing task.
func (agent *Agent) CancelTask(
	ctx context.Context, id string,
) (types.Task, *errors.RpcError) {
	// TODO: Implement actual cancellation signalling if possible (e.g., context cancellation for ongoing streams/LLM calls)
	log.Info("received CancelTask request", "taskID", id)

	task, err := agent.loadTask(ctx, id)
	if err != nil {
		// If task not found, maybe return a specific state or error?
		// For now, return the error from loadTask.
		return types.Task{}, err
	}

	// Check if task is already in a final state
	if task.Status.State == types.TaskStateCompleted || task.Status.State == types.TaskStateFailed || task.Status.State == types.TaskStateCanceled {
		log.Warn("task already in final state, cannot cancel", "taskID", id, "state", task.Status.State)
		// Return current state without modification
		return task, nil
	}

	task.ToState(types.TaskStateCanceled, "Task cancellation requested")
	// TODO: Signal cancellation to any active processing (e.g., chatClient.Stream)

	// Store the updated (canceled) state.
	task, err = agent.storeTask(ctx, task)
	if err != nil {
		// Log error but return the task state we tried to set
		log.Error("failed to store canceled task state", "taskID", id, "error", err)
	}

	return task, nil
}

// ResubscribeTask allows a client to resubscribe to task events.
func (agent *Agent) ResubscribeTask(
	ctx context.Context,
	id string,
	historyLength int,
) (<-chan any, *errors.RpcError) {
	log.Info("received ResubscribeTask request", "taskID", id)
	// This method primarily signals intent or retrieves initial state.
	// The actual event stream connection (SSE) is managed by the server/transport.
	// Retrieving the task state might still be useful, but the interface requires a channel.
	// For now, return nil channel as the agent itself doesn't manage the live subscription.
	_, rpcErr := agent.GetTask(ctx, id, historyLength)
	if rpcErr != nil {
		// If task not found, should we still return nil chan or the error?
		// Let's return the error.
		return nil, rpcErr
	}

	log.Warn("ResubscribeTask called, but agent doesn't manage the event channel. Returning nil channel.", "taskID", id)
	return nil, nil // Return nil channel and no error
}

// SetPushNotification configures push notifications for a task.
func (agent *Agent) SetPushNotification(
	ctx context.Context,
	config types.TaskPushNotificationConfig,
) (types.TaskPushNotificationConfig, *errors.RpcError) {
	log.Info("received SetPushNotification request", "taskID", config.ID)
	// TODO: Need a persistent way to store push config associated with the task ID.
	// Storing it directly IN the task object might make it too large over time.
	// Consider a separate store or a dedicated field if schema allows.

	// For now, let's try loading the task and storing in metadata, then saving.
	task, rpcErr := agent.loadTask(ctx, config.ID)
	// Allow setting config even if task doesn't exist yet? Or require task exists?
	// Let's assume task should exist.
	if rpcErr != nil {
		return types.TaskPushNotificationConfig{}, rpcErr
	}

	if task.Metadata == nil {
		task.Metadata = make(map[string]any)
	}
	task.Metadata["pushConfig"] = config // Store the config

	// Save the updated task
	_, rpcErr = agent.storeTask(ctx, task)
	if rpcErr != nil {
		log.Error("failed to store task with updated push config", "taskID", config.ID, "error", rpcErr)
		return types.TaskPushNotificationConfig{}, rpcErr
	}

	log.Info("SetPushNotification config stored in task metadata", "taskID", config.ID)
	return config, nil // Return the config value
}

// GetPushNotification retrieves push notification config for a task.
func (agent *Agent) GetPushNotification(
	ctx context.Context,
	id string,
) (types.TaskPushNotificationConfig, *errors.RpcError) {
	log.Info("received GetPushNotification request", "taskID", id)

	task, rpcErr := agent.loadTask(ctx, id)
	if rpcErr != nil {
		return types.TaskPushNotificationConfig{}, rpcErr
	}

	if task.Metadata != nil {
		if cfgData, ok := task.Metadata["pushConfig"]; ok {
			// Attempt to convert stored data back to config struct
			var cfg types.TaskPushNotificationConfig
			// Use JSON marshaling/unmarshaling for conversion between map[string]any and struct
			cfgBytes, err := json.Marshal(cfgData)
			if err != nil {
				log.Error("failed to marshal stored push config data", "taskID", id, "error", err)
				return types.TaskPushNotificationConfig{}, errors.ErrInternal.WithMessagef("failed to marshal push config: %v", err)
			}
			err = json.Unmarshal(cfgBytes, &cfg)
			if err != nil {
				log.Error("failed to unmarshal stored push config data", "taskID", id, "error", err)
				return types.TaskPushNotificationConfig{}, errors.ErrInternal.WithMessagef("failed to unmarshal push config: %v", err)
			}
			return cfg, nil // Return the config value
		}
	}

	log.Warn("GetPushNotification config not found in task metadata", "taskID", id)
	return types.TaskPushNotificationConfig{}, errors.ErrPushNotificationConfigNotFound.WithMessagef("push config not found for task %s", id)
}

// --- Helper Methods ---

func (agent *Agent) storeTask(
	ctx context.Context,
	task types.Task,
) (types.Task, *errors.RpcError) {
	// Ensure task has an ID if it's missing (might happen on initial store)
	if task.ID == "" {
		task.ID = uuid.NewString()
		log.Warn("Task had no ID during store, assigned new one", "assignedID", task.ID)
	}
	// Ensure task has a status if missing (e.g., first store)
	if task.Status.State == "" {
		task.ToState(types.TaskStateSubmitted, "Task received")
	}

	prefix := fmt.Sprintf(
		"%s/%s.json",
		agent.card.Name, // Use agent name for partitioning
		task.ID,
	)

	p, err := json.MarshalIndent(task, "", "  ") // Use MarshalIndent for readability in storage
	if err != nil {
		log.Error("failed to marshal task for storage", "taskID", task.ID, "error", err)
		return task, errors.ErrInternal.WithMessagef("failed to marshal task %s: %v", task.ID, err)
	}

	// Assuming taskStore.Put handles context cancellation etc.
	if putErr := agent.taskStore.Put(ctx, "tasks", prefix, bytes.NewReader(p)); putErr != nil {
		log.Error("failed to put task to store", "taskID", task.ID, "prefix", prefix, "error", putErr)
		// Wrap the underlying storage error if possible, otherwise return generic internal error
		return task, errors.ErrInternal.WithMessagef("failed to store task %s: %v", task.ID, putErr)
	}

	log.Debug("task stored successfully", "taskID", task.ID, "prefix", prefix)
	return task, nil
}

func (agent *Agent) loadTask(
	ctx context.Context,
	id string,
) (types.Task, *errors.RpcError) {
	if id == "" {
		log.Error("loadTask called with empty ID")
		return types.Task{}, errors.ErrInvalidParams.WithMessagef("task ID cannot be empty")
	}

	prefix := fmt.Sprintf(
		"%s/%s.json",
		agent.card.Name, // Use agent name for partitioning
		id,
	)

	taskBytes, err := agent.taskStore.Get(ctx, "tasks", prefix)
	if err != nil {
		log.Error("failed to get task from store", "taskID", id, "prefix", prefix, "error", err)
		// Check if the error indicates "not found" specifically
		// This depends on the s3.Conn implementation. Assuming it might return a standard error for now.
		// TODO: Map storage errors (like S3 NoSuchKey) to ErrTaskNotFound
		return types.Task{}, errors.ErrTaskNotFound.WithMessagef("failed to load task %s: %v", id, err) // Assume not found for now
	}

	if taskBytes == nil || taskBytes.Len() == 0 {
		log.Error("task data loaded from store is empty", "taskID", id, "prefix", prefix)
		return types.Task{}, errors.ErrTaskNotFound.WithMessagef("task %s not found or empty in store", id)
	}

	var task types.Task
	if unmarshalErr := json.Unmarshal(taskBytes.Bytes(), &task); unmarshalErr != nil {
		log.Error("failed to unmarshal task data from store", "taskID", id, "prefix", prefix, "error", unmarshalErr)
		return types.Task{}, errors.ErrInternal.WithMessagef("failed to parse stored task %s: %v", id, unmarshalErr)
	}

	log.Debug("task loaded successfully", "taskID", task.ID, "prefix", prefix)
	return task, nil
}
