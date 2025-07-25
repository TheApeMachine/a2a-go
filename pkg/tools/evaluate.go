package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

type EvaluateTool struct {
	tool *mcp.Tool
}

func NewEvaluateTool() *mcp.Tool {
	tool := mcp.NewTool(
		"evaluate_output",
		mcp.WithDescription("Evaluate if a task output meets completion requirements. Use before marking any task as completed."),
		mcp.WithString("original_task", mcp.Description("The original task that was requested."), mcp.Required()),
		mcp.WithString("agent_output", mcp.Description("The output produced by the agent."), mcp.Required()),
		mcp.WithString("executing_agent", mcp.Description("Name of the agent that produced the output."), mcp.Required()),
	)
	return &tool
}

func (et *EvaluateTool) RegisterEvaluateTools(srv *server.MCPServer) {
	srv.AddTool(*et.tool, et.Handle)
}

func (et *EvaluateTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("EvaluateTool: Received evaluation request")

	// Extract parameters - same pattern as delegate tool
	originalTask, ok := req.GetArguments()["original_task"].(string)
	if !ok {
		return mcp.NewToolResultError("original_task parameter is required"), nil
	}

	agentOutput, ok := req.GetArguments()["agent_output"].(string)
	if !ok {
		return mcp.NewToolResultError("agent_output parameter is required"), nil
	}

	executingAgent, ok := req.GetArguments()["executing_agent"].(string)
	if !ok {
		return mcp.NewToolResultError("executing_agent parameter is required"), nil
	}

	// Create evaluation prompt
	evaluationPrompt := fmt.Sprintf(`EVALUATION REQUEST

ORIGINAL TASK: %s

AGENT OUTPUT: %s

EXECUTING AGENT: %s

Evaluate if the output fully satisfies the original task. Consider:
- Does it directly address what was requested?
- Is the information complete and useful?
- Are there obvious failures or incomplete sections?
- Would this satisfy the user's original intent?

Respond with exactly: DECISION:[COMPLETE/ITERATE/ESCALATE] REASONING:[detailed explanation]

COMPLETE = Task is fully satisfied and ready to progress
ITERATE = Current agent should try again with improvements  
ESCALATE = Different approach or agent is needed`,
		originalTask, agentOutput, executingAgent)

	// Send to evaluator agent using standard A2A delegation - same as delegate tool
	agentURL := "http://evaluator:3210"
	// Convert Docker internal URLs to localhost when running locally
	if strings.Contains(agentURL, "evaluator:3210") {
		agentURL = "http://localhost:3213" // Evaluator will be mapped to port 3213
	}

	agentClient := a2a.NewClient(agentURL)

	taskID := uuid.New().String()
	response, err := agentClient.SendTask(a2a.TaskSendParams{
		ID:        taskID,
		SessionID: uuid.New().String(),
		Message:   *a2a.NewTextMessage("user", evaluationPrompt),
	})

	if err != nil {
		log.Error("EvaluateTool: Failed to send to evaluator", "error", err)
		return mcp.NewToolResultError("Failed to communicate with evaluator agent: " + err.Error()), nil
	}

	if response.Error != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Evaluator error: %s", response.Error.Message)), nil
	}

	// Extract result using same JSON parsing as our chat UI
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return mcp.NewToolResultError("Failed to parse evaluator response"), nil
	}

	var task a2a.Task
	if err := json.Unmarshal(resultBytes, &task); err != nil {
		return mcp.NewToolResultError("Failed to parse task data"), nil
	}

	// Get evaluator's response from task history
	var evaluatorResponse string
	for i := len(task.History) - 1; i >= 0; i-- {
		msg := task.History[i]
		if msg.Role == "assistant" && len(msg.Parts) > 0 {
			evaluatorResponse = msg.Parts[0].Text
			break
		}
	}

	if evaluatorResponse == "" {
		return mcp.NewToolResultError("No evaluation response received"), nil
	}

	log.Info("EvaluateTool: Evaluation completed", "response", evaluatorResponse)
	return mcp.NewToolResultText(evaluatorResponse), nil
}

func extractDecision(response string) string {
	if strings.Contains(response, "DECISION:COMPLETE") {
		return "COMPLETE"
	}
	if strings.Contains(response, "DECISION:ITERATE") {
		return "ITERATE"
	}
	if strings.Contains(response, "DECISION:ESCALATE") {
		return "ESCALATE"
	}
	return "UNKNOWN"
}

func extractReasoning(response string) string {
	parts := strings.Split(response, "REASONING:")
	if len(parts) > 1 {
		// Join all parts from index 1 onwards, preserving "REASONING:" separators
		reasoning := strings.Join(parts[1:], "REASONING:")
		return strings.TrimSpace(reasoning)
	}
	return response
}
