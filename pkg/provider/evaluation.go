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

	// Avoid recursive self-evaluation for evaluator agent or when disabled
	if task.Metadata != nil {
		if disable, ok := task.Metadata["disableEvaluation"].(bool); ok && disable {
			return true, "Evaluation disabled by task metadata", nil
		}
		if currentAgent, ok := task.Metadata["agentName"].(string); ok {
			lower := strings.ToLower(currentAgent)
			if strings.Contains(lower, "evaluator") {
				return true, "Evaluation disabled for evaluator agent to avoid recursion", nil
			}
		}
	}

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
	reason := extractReasoning(evaluationResponse)

	switch decision {
	case "COMPLETE":
		return true, reason, nil
	case "ITERATE":
		return false, reason, nil
	case "ESCALATE":
		// For now treat ESCALATE as complete and let calling code decide next steps
		return true, reason, nil
	default:
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
	parts := strings.Split(response, "REASONING:")
	if len(parts) > 1 {
		// Join all parts from index 1 onwards, preserving "REASONING:" separators
		reasoning := strings.Join(parts[1:], "REASONING:")
		return strings.TrimSpace(reasoning)
	}
	return response
}
