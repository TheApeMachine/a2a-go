package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/smartystreets/goconvey/convey"
)

func TestNewEvaluateTool(t *testing.T) {
	convey.Convey("Given the evaluate tool constructor", t, func() {

		convey.Convey("When creating a new evaluate tool", func() {
			tool := NewEvaluateTool()

			convey.So(tool, convey.ShouldNotBeNil)
			convey.So(tool.Name, convey.ShouldEqual, "evaluate_output")
			convey.So(tool.Description, convey.ShouldContainSubstring, "Evaluate if a task output meets completion requirements")
			convey.So(len(tool.InputSchema.Properties), convey.ShouldEqual, 3)
			convey.So(tool.InputSchema.Required, convey.ShouldContain, "original_task")
			convey.So(tool.InputSchema.Required, convey.ShouldContain, "agent_output")
			convey.So(tool.InputSchema.Required, convey.ShouldContain, "executing_agent")
		})
	})
}

func TestEvaluateToolParameterValidation(t *testing.T) {
	convey.Convey("Given an evaluate tool", t, func() {
		evaluateTool := &EvaluateTool{}
		ctx := context.Background()

		convey.Convey("When handling a request with missing original_task", func() {
			req := mcp.CallToolRequest{}
			req.Params.Name = "evaluate_output"
			req.Params.Arguments = map[string]any{
				"agent_output":    "some output",
				"executing_agent": "test-agent",
			}

			result, err := evaluateTool.Handle(ctx, req)

			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			convey.So(len(result.Content), convey.ShouldBeGreaterThan, 0)
			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				convey.So(textContent.Text, convey.ShouldContainSubstring, "original_task parameter is required")
			}
		})

		convey.Convey("When handling a request with missing agent_output", func() {
			req := mcp.CallToolRequest{}
			req.Params.Name = "evaluate_output"
			req.Params.Arguments = map[string]any{
				"original_task":   "Write a function",
				"executing_agent": "test-agent",
			}

			result, err := evaluateTool.Handle(ctx, req)

			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				convey.So(textContent.Text, convey.ShouldContainSubstring, "agent_output parameter is required")
			}
		})

		convey.Convey("When handling a request with missing executing_agent", func() {
			req := mcp.CallToolRequest{}
			req.Params.Name = "evaluate_output"
			req.Params.Arguments = map[string]any{
				"original_task": "Write a function",
				"agent_output":  "func add() {}",
			}

			result, err := evaluateTool.Handle(ctx, req)

			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				convey.So(textContent.Text, convey.ShouldContainSubstring, "executing_agent parameter is required")
			}
		})

		convey.Convey("When handling a request with wrong parameter types", func() {
			req := mcp.CallToolRequest{}
			req.Params.Name = "evaluate_output"
			req.Params.Arguments = map[string]any{
				"original_task":   123, // Should be string
				"agent_output":    "some output",
				"executing_agent": "test-agent",
			}

			result, err := evaluateTool.Handle(ctx, req)

			convey.So(err, convey.ShouldBeNil)
			convey.So(result, convey.ShouldNotBeNil)
			if textContent, ok := result.Content[0].(mcp.TextContent); ok {
				convey.So(textContent.Text, convey.ShouldContainSubstring, "original_task parameter is required")
			}
		})
	})
}

func TestExtractDecisionAndReasoning(t *testing.T) {
	convey.Convey("Given decision and reasoning extraction", t, func() {

		convey.Convey("When extracting from properly formatted COMPLETE response", func() {
			response := "DECISION:COMPLETE REASONING:The code correctly implements the requested functionality with proper syntax and logic."

			decision := extractDecision(response)
			reasoning := extractReasoning(response)

			convey.So(decision, convey.ShouldEqual, "COMPLETE")
			convey.So(reasoning, convey.ShouldEqual, "The code correctly implements the requested functionality with proper syntax and logic.")
		})

		convey.Convey("When extracting from ITERATE response with detailed feedback", func() {
			response := "DECISION:ITERATE REASONING:The function is missing error handling and parameter validation. Please add proper input validation."

			decision := extractDecision(response)
			reasoning := extractReasoning(response)

			convey.So(decision, convey.ShouldEqual, "ITERATE")
			convey.So(reasoning, convey.ShouldEqual, "The function is missing error handling and parameter validation. Please add proper input validation.")
		})

		convey.Convey("When extracting from ESCALATE response", func() {
			response := "DECISION:ESCALATE REASONING:This task requires specialized domain knowledge that exceeds my current capabilities."

			decision := extractDecision(response)
			reasoning := extractReasoning(response)

			convey.So(decision, convey.ShouldEqual, "ESCALATE")
			convey.So(reasoning, convey.ShouldEqual, "This task requires specialized domain knowledge that exceeds my current capabilities.")
		})

		convey.Convey("When extracting from response with extra text before decision", func() {
			response := "Let me evaluate this carefully. DECISION:COMPLETE REASONING:All requirements met successfully."

			decision := extractDecision(response)
			reasoning := extractReasoning(response)

			convey.So(decision, convey.ShouldEqual, "COMPLETE")
			convey.So(reasoning, convey.ShouldEqual, "All requirements met successfully.")
		})

		convey.Convey("When extracting from malformed response", func() {
			response := "Something went wrong during evaluation, no clear decision provided"

			decision := extractDecision(response)
			reasoning := extractReasoning(response)

			convey.So(decision, convey.ShouldEqual, "UNKNOWN")
			convey.So(reasoning, convey.ShouldEqual, "Something went wrong during evaluation, no clear decision provided")
		})

		convey.Convey("When extracting from response with no reasoning section", func() {
			response := "DECISION:COMPLETE"

			decision := extractDecision(response)
			reasoning := extractReasoning(response)

			convey.So(decision, convey.ShouldEqual, "COMPLETE")
			convey.So(reasoning, convey.ShouldEqual, "DECISION:COMPLETE")
		})

		convey.Convey("When extracting from response with multiple REASONING occurrences", func() {
			response := "DECISION:ITERATE REASONING:First issue found. REASONING:Second issue also present."

			decision := extractDecision(response)
			reasoning := extractReasoning(response)

			convey.So(decision, convey.ShouldEqual, "ITERATE")
			convey.So(reasoning, convey.ShouldEqual, "First issue found. REASONING:Second issue also present.")
		})
	})
}
