package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/tools"
)

// EvaluateBeforeCompletion evaluates task output before marking it complete
// Returns true if task should be marked complete, false if it needs iteration
func EvaluateBeforeCompletion(ctx context.Context, task *a2a.Task, agentOutput string, agentName string) (bool, string, error) {
	log.Info("EvaluateBeforeCompletion: Starting evaluation", "agentName", agentName, "taskID", task.ID)

	// Get original task request from history
	var originalTask string
	for _, msg := range task.History {
		if msg.Role == "user" && len(msg.Parts) > 0 {
			originalTask = msg.Parts[0].Text
			break
		}
	}

	if originalTask == "" {
		log.Warn("EvaluateBeforeCompletion: No original task found in history")
		return true, "No original task found for evaluation", nil
	}

	// Create evaluate tool and execute it
	evaluateTool := &tools.EvaluateTool{}
	arguments := map[string]any{
		"original_task":   originalTask,
		"agent_output":    agentOutput,
		"executing_agent": agentName,
	}

	callRequest := mcp.CallToolRequest{}
	callRequest.Params.Name = "evaluate_output"
	callRequest.Params.Arguments = arguments

	result, err := evaluateTool.Handle(ctx, callRequest)
	if err != nil {
		log.Error("EvaluateBeforeCompletion: Evaluation failed", "error", err)
		// If evaluation fails, allow completion to avoid blocking
		return true, fmt.Sprintf("Evaluation failed: %v", err), nil
	}

	var evaluationResponse string
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			evaluationResponse = textContent.Text
		}
	}

	if evaluationResponse == "" {
		log.Warn("EvaluateBeforeCompletion: Empty evaluation response")
		return true, "Empty evaluation response", nil
	}

	log.Info("EvaluateBeforeCompletion: Evaluation completed", "response", evaluationResponse)

	// Parse evaluation decision
	decision := extractDecision(evaluationResponse)
	reasoning := extractReasoning(evaluationResponse)

	switch decision {
	case "COMPLETE":
		log.Info("EvaluateBeforeCompletion: Task approved for completion", "reasoning", reasoning)
		return true, reasoning, nil
	case "ITERATE":
		log.Info("EvaluateBeforeCompletion: Task needs iteration", "reasoning", reasoning)
		return false, reasoning, nil
	case "ESCALATE":
		log.Info("EvaluateBeforeCompletion: Task needs escalation", "reasoning", reasoning)
		// For now, treat escalation as completion - could be enhanced later
		return true, fmt.Sprintf("ESCALATION NEEDED: %s", reasoning), nil
	default:
		log.Warn("EvaluateBeforeCompletion: Unknown decision", "decision", decision)
		// Default to completion if decision is unclear
		return true, fmt.Sprintf("Unknown evaluation decision: %s", evaluationResponse), nil
	}
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
	parts := strings.SplitN(response, "REASONING:", 2)
	if len(parts) > 1 {
		return strings.TrimSpace(parts[1])
	}
	return response
}
