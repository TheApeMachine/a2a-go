package provider

import (
	"context"
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

func TestEvaluateBeforeCompletion(t *testing.T) {
	convey.Convey("Given an evaluation system", t, func() {
		ctx := context.Background()
		
		convey.Convey("When evaluating a task with no original task in history", func() {
			task := &a2a.Task{
				ID: "test-task-no-user",
				History: []a2a.Message{
					{
						Role: "assistant",
						Parts: []a2a.Part{
							{Type: a2a.PartTypeText, Text: "Some response"},
						},
					},
				},
			}
			
			shouldComplete, reason, err := EvaluateBeforeCompletion(ctx, task, "test output", "test-agent")
			
			convey.So(err, convey.ShouldBeNil)
			convey.So(shouldComplete, convey.ShouldBeTrue)
			convey.So(reason, convey.ShouldEqual, "No original task found for evaluation")
		})
		
		convey.Convey("When evaluation fails due to connection error", func() {
			task := &a2a.Task{
				ID: "test-task-connection-fail",
				History: []a2a.Message{
					{
						Role: "user",
						Parts: []a2a.Part{
							{Type: a2a.PartTypeText, Text: "Write a function that adds two numbers"},
						},
					},
				},
			}
			
			agentOutput := "func add(a, b int) int { return a + b }"
			
			// This will fail to connect to evaluator, but should gracefully fallback
			shouldComplete, reason, err := EvaluateBeforeCompletion(ctx, task, agentOutput, "test-agent")
			
			// Should allow completion to avoid blocking the system
			convey.So(err, convey.ShouldBeNil)
			convey.So(shouldComplete, convey.ShouldBeTrue)
			convey.So(reason, convey.ShouldContainSubstring, "evaluation")
		})
		
		convey.Convey("When evaluating a task with empty agent output", func() {
			task := &a2a.Task{
				ID: "test-task-empty-output",
				History: []a2a.Message{
					{
						Role: "user",
						Parts: []a2a.Part{
							{Type: a2a.PartTypeText, Text: "Do something useful"},
						},
					},
				},
			}
			
			shouldComplete, reason, err := EvaluateBeforeCompletion(ctx, task, "", "test-agent")
			
			convey.So(err, convey.ShouldBeNil)
			convey.So(shouldComplete, convey.ShouldBeTrue) // Fallback behavior
			convey.So(reason, convey.ShouldNotBeEmpty)
		})
		
		convey.Convey("When task has multiple user messages", func() {
			task := &a2a.Task{
				ID: "test-task-multiple-user",
				History: []a2a.Message{
					{
						Role: "user",
						Parts: []a2a.Part{
							{Type: a2a.PartTypeText, Text: "First request"},
						},
					},
					{
						Role: "assistant",
						Parts: []a2a.Part{
							{Type: a2a.PartTypeText, Text: "First response"},
						},
					},
					{
						Role: "user",
						Parts: []a2a.Part{
							{Type: a2a.PartTypeText, Text: "Follow-up request"},
						},
					},
				},
			}
			
			shouldComplete, reason, err := EvaluateBeforeCompletion(ctx, task, "Follow-up response", "test-agent")
			
			convey.So(err, convey.ShouldBeNil)
			// Should find the first user message as original task
			convey.So(shouldComplete, convey.ShouldBeTrue) // Due to connection failure fallback
			convey.So(reason, convey.ShouldNotBeEmpty)
		})
	})
}

func TestExtractDecision(t *testing.T) {
	convey.Convey("Given decision extraction functions", t, func() {
		
		convey.Convey("When extracting COMPLETE decision", func() {
			response := "DECISION:COMPLETE REASONING:Task is fully satisfied"
			decision := extractDecision(response)
			convey.So(decision, convey.ShouldEqual, "COMPLETE")
		})
		
		convey.Convey("When extracting ITERATE decision", func() {
			response := "DECISION:ITERATE REASONING:Needs improvement"
			decision := extractDecision(response)
			convey.So(decision, convey.ShouldEqual, "ITERATE")
		})
		
		convey.Convey("When extracting ESCALATE decision", func() {
			response := "DECISION:ESCALATE REASONING:Different approach needed"
			decision := extractDecision(response)
			convey.So(decision, convey.ShouldEqual, "ESCALATE")
		})
		
		convey.Convey("When extracting unknown decision", func() {
			response := "Some unclear response"
			decision := extractDecision(response)
			convey.So(decision, convey.ShouldEqual, "UNKNOWN")
		})
		
		convey.Convey("When extracting decision with lowercase", func() {
			response := "decision:complete reasoning:all good"
			decision := extractDecision(response)
			convey.So(decision, convey.ShouldEqual, "UNKNOWN") // Should be case-sensitive
		})
		
		convey.Convey("When extracting decision with extra text", func() {
			response := "Here is my evaluation. DECISION:ITERATE REASONING:More work needed. End of evaluation."
			decision := extractDecision(response)
			convey.So(decision, convey.ShouldEqual, "ITERATE")
		})
	})
}

func TestExtractReasoning(t *testing.T) {
	convey.Convey("Given reasoning extraction functions", t, func() {
		
		convey.Convey("When extracting reasoning from structured response", func() {
			response := "DECISION:COMPLETE REASONING:The function correctly implements addition with proper parameters and return type"
			reasoning := extractReasoning(response)
			convey.So(reasoning, convey.ShouldEqual, "The function correctly implements addition with proper parameters and return type")
		})
		
		convey.Convey("When extracting reasoning from response without REASONING prefix", func() {
			response := "This is just a plain response"
			reasoning := extractReasoning(response)
			convey.So(reasoning, convey.ShouldEqual, "This is just a plain response")
		})
		
		convey.Convey("When extracting reasoning with multiple REASONING occurrences", func() {
			response := "DECISION:ITERATE REASONING:First part REASONING:Second part"
			reasoning := extractReasoning(response)
			convey.So(reasoning, convey.ShouldEqual, "First part REASONING:Second part")
		})
		
		convey.Convey("When extracting reasoning from empty REASONING section", func() {
			response := "DECISION:COMPLETE REASONING:"
			reasoning := extractReasoning(response)
			convey.So(reasoning, convey.ShouldEqual, "")
		})
		
		convey.Convey("When extracting reasoning with whitespace", func() {
			response := "DECISION:ITERATE REASONING:   The output needs more detail   "
			reasoning := extractReasoning(response)
			convey.So(reasoning, convey.ShouldEqual, "The output needs more detail")
		})
		
		convey.Convey("When extracting reasoning with newlines", func() {
			response := "DECISION:COMPLETE REASONING:Good work.\nAll requirements met.\nReady to proceed."
			reasoning := extractReasoning(response)
			convey.So(reasoning, convey.ShouldEqual, "Good work.\nAll requirements met.\nReady to proceed.")
		})
	})
}
