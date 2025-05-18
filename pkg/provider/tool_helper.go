package provider

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/tools"
)

// LLMToolResponseGenerator creates a provider-specific message that represents
// the result of a tool call, to be sent back to the LLM.
// toolCallID is the ID of the tool call, as provided by the LLM.
// content is the string content of the tool's execution (result or error message).
// isError indicates if the content represents an error.
type LLMToolResponseGenerator func(toolCallID string, content string, isError bool) any

// ExecuteAndProcessToolCall centralizes the logic for executing a tool,
// updating the task with an artifact, and preparing the tool response message for the LLM.
// It modifies the input 'task' in place by adding an artifact.
// It returns the (modified) task, the generated LLM-specific tool response message,
// and any error encountered during tool execution.
func ExecuteAndProcessToolCall(
	ctx context.Context,
	toolName string,
	toolArguments string,
	toolCallID string, // The ID from the LLM's tool request, used by some providers for constructing the response.
	task *a2a.Task, // Input task, will be modified in place.
	generateLLMToolResponse LLMToolResponseGenerator,
) (updatedTask *a2a.Task, llmToolResponse any, executionError error) {

	log.Debug("Executing tool via helper", "tool_name", toolName, "arguments", toolArguments)

	resultContent, err := tools.NewExecutor(ctx, toolName, toolArguments)

	artifactName := toolName
	var artifactDescription string
	var artifactParts []a2a.Part

	if err != nil {
		log.Error("Error executing tool via helper", "tool_name", toolName, "error", err)
		errorMsg := fmt.Sprintf("Error: %s", err.Error())
		artifactDescription = "Tool execution failed."
		artifactParts = []a2a.Part{a2a.NewTextPart(errorMsg)}

		// Generate the LLM-specific response message indicating an error.
		llmToolResponse = generateLLMToolResponse(toolCallID, errorMsg, true)
		executionError = err // Preserve the original error from the executor.
	} else {
		log.Debug("Tool executed successfully via helper", "tool_name", toolName, "result_length", len(resultContent))
		artifactDescription = fmt.Sprintf("Output from %s tool.", toolName)
		artifactParts = []a2a.Part{a2a.NewTextPart(resultContent)}

		// Generate the LLM-specific response message with the successful result.
		llmToolResponse = generateLLMToolResponse(toolCallID, resultContent, false)
		executionError = nil
	}

	task.AddArtifact(a2a.Artifact{
		Name:        &artifactName,
		Description: &artifactDescription,
		Parts:       artifactParts,
	})

	// The task object passed in is modified in place, so we return it.
	updatedTask = task
	return
}
