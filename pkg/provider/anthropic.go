package provider

import (
	"context"
	"fmt"
	"os"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
)

/*
anthropicRoleMap compresses convertMessages' switch.
*/
var anthropicRoleMap = map[string]func(string) anthropic.MessageParam{
	"user": func(text string) anthropic.MessageParam {
		return anthropic.NewUserMessage(anthropic.NewTextBlock(text))
	},
	"developer": func(text string) anthropic.MessageParam {
		return anthropic.NewUserMessage(anthropic.NewTextBlock(text))
	},
	"agent": func(text string) anthropic.MessageParam {
		return anthropic.NewAssistantMessage(anthropic.NewTextBlock(text))
	},
	"assistant": func(text string) anthropic.MessageParam {
		return anthropic.NewAssistantMessage(anthropic.NewTextBlock(text))
	},
}

/*
AnthropicProvider is a provider for the Anthropic API.
*/
type AnthropicProvider struct {
	client *anthropic.Client
	params *anthropic.MessageNewParams
}

type AnthropicProviderOption func(*AnthropicProvider)

func NewAnthropicProvider(options ...AnthropicProviderOption) *AnthropicProvider {
	prvdr := &AnthropicProvider{}

	for _, option := range options {
		option(prvdr)
	}

	return prvdr
}

func (prvdr *AnthropicProvider) Generate(
	ctx context.Context, params *ProviderParams,
) chan jsonrpc.Response {
	ch := make(chan jsonrpc.Response)

	// Anthropic-specific LLM tool response generator function
	anthropicToolResponseGenerator := func(toolCallID string, content string, isError bool) any {
		return anthropic.NewUserMessage(anthropic.NewToolResultBlock(toolCallID, content, isError))
	}

	go func() {
		defer close(ch)

		prvdr.params = &anthropic.MessageNewParams{
			Model: anthropic.Model(params.Model),
			System: []anthropic.TextBlockParam{anthropic.TextBlockParam{
				Text: params.Task.History[0].Parts[0].Text,
			}},
			Messages:      prvdr.convertMessages(params.Task),
			Tools:         prvdr.convertTools(params.Tools),
			MaxTokens:     params.MaxTokens,
			StopSequences: params.Stop,
		}

		isDone := false

		for !isDone {
			if params.Stream {
				stream := prvdr.client.Messages.NewStreaming(ctx, *prvdr.params)
				message := anthropic.Message{} // Used by accumulator

				for stream.Next() {
					event := stream.Current()
					if err := message.Accumulate(event); err != nil { // Accumulate first
						log.Error("failed to accumulate message event", "error", err)
						continue
					}

					switch event := event.AsAny().(type) { // then switch on the event type
					case anthropic.ContentBlockDeltaEvent:
						if event.Delta.Text != "" {
							ch <- a2a.NewArtifactResult(params.Task.ID, a2a.NewTextPart(event.Delta.Text))
						}
					case anthropic.ToolUseBlock: // This is a specific event type from Anthropic SDK when a tool is requested
						// The message accumulator would have added the assistant's request for tool use.
						// We now need to execute it.
						prvdr.params.Messages = append(prvdr.params.Messages, message.ToParam()) // Add current assistant msg to history for LLM

						updatedTask, llmToolMsg, toolExecErr := ExecuteAndProcessToolCall(
							ctx,
							event.Name,          // ToolUseBlock has Name
							string(event.Input), // ToolUseBlock has Input (json.RawMessage)
							event.ID,            // ToolUseBlock has ID
							params.Task,
							anthropicToolResponseGenerator,
						)
						params.Task = updatedTask // Persist changes to task
						prvdr.params.Messages = append(prvdr.params.Messages, llmToolMsg.(anthropic.MessageParam))

						if toolExecErr != nil {
							ch <- jsonrpc.Response{
								Result: params.Task, // Send updated task with error artifact
								Error:  &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: fmt.Sprintf("Error executing tool %s: %v", event.Name, toolExecErr)},
							}
						} else {
							ch <- jsonrpc.Response{Result: params.Task} // Send updated task with success artifact
						}
						// Stream continues, LLM will get the tool response via updated prvdr.params.Messages
					case anthropic.MessageStopEvent:
						isDone = true
						// Potentially send final task state if not already covered by other events
						// For now, assume final content/artifacts are handled by ContentBlockDelta or completion.
						ch <- jsonrpc.Response{Result: a2a.TaskStatusUpdateResult{ID: params.Task.ID, Status: params.Task.Status, Final: true}}
					}
				}
				if stream.Err() != nil {
					ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: stream.Err().Error()}}
				}
				isDone = true // Ensure loop terminates after stream or if stream.Next() finishes

			} else { // Non-streaming path
				llmResponse, err := prvdr.client.Messages.New(ctx, *prvdr.params)
				if err != nil {
					ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: err.Error()}}
					return // Use return for non-streaming fatal error
				}

				prvdr.params.Messages = append(prvdr.params.Messages, llmResponse.ToParam())
				assistantCalledTool := false
				var assistantTextResponse string // Accumulate text here

				for _, block := range llmResponse.Content {
					switch contentBlock := block.AsAny().(type) {
					case anthropic.TextBlock:
						assistantTextResponse += contentBlock.Text // Accumulate text
						// Only add text part and send artifact if no tool is being called in this turn by assistant.
						// If a tool is called, the text is usually just a preamble to the tool call.
						// The final text response will come after the tool results are processed by the LLM.
						// We will check assistantCalledTool *after* processing all blocks.
					case anthropic.ToolUseBlock:
						assistantCalledTool = true
						updatedTask, llmToolMsg, toolExecErr := ExecuteAndProcessToolCall(
							ctx,
							contentBlock.Name,
							string(contentBlock.Input),
							contentBlock.ID,
							params.Task,
							anthropicToolResponseGenerator,
						)
						params.Task = updatedTask
						prvdr.params.Messages = append(prvdr.params.Messages, llmToolMsg.(anthropic.MessageParam))

						if toolExecErr != nil {
							ch <- jsonrpc.Response{
								Result: params.Task,
								Error:  &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: fmt.Sprintf("Error executing tool %s: %v", contentBlock.Name, toolExecErr)},
							}
						} else {
							ch <- jsonrpc.Response{Result: params.Task}
						}
					}
				}

				if !assistantCalledTool {
					// If no tools were called, then any accumulated text is the final response for this turn.
					if assistantTextResponse != "" {
						params.Task.AddFinalPart(a2a.NewTextPart(assistantTextResponse))
						ch <- a2a.NewArtifactResult(params.Task.ID, a2a.NewTextPart(assistantTextResponse))
					}

					// Evaluate before completion
					shouldComplete, evaluationReason, evalErr := EvaluateBeforeCompletion(
						ctx,
						params.Task,
						assistantTextResponse,
						"anthropic",
					)

					if evalErr != nil {
						log.Warn("Anthropic: Evaluation error, proceeding with completion", "error", evalErr)
						params.Task.ToStatus(a2a.TaskStateCompleted, a2a.NewTextMessage("assistant", assistantTextResponse))
						ch <- jsonrpc.Response{Result: params.Task}
						isDone = true
					} else if shouldComplete {
						log.Info("Anthropic: Task approved for completion", "reason", evaluationReason)
						params.Task.ToStatus(a2a.TaskStateCompleted, a2a.NewTextMessage("assistant", assistantTextResponse))
						ch <- jsonrpc.Response{Result: params.Task}
						isDone = true
					} else {
						log.Info("Anthropic: Task needs iteration", "reason", evaluationReason)
						// Add evaluation feedback to conversation and continue
						iterationPrompt := fmt.Sprintf("The evaluator reviewed your response and determined it needs improvement. Feedback: %s\n\nPlease revise your response to better address the original task.", evaluationReason)
						prvdr.params.Messages = append(prvdr.params.Messages, anthropic.NewUserMessage(anthropic.NewTextBlock(iterationPrompt)))
						isDone = false
					}
				} else {
					// Tools were called. The loop will continue to make another call to the LLM
					// with the tool results included in prvdr.params.Messages.
					isDone = false
				}
			}
		}
	}()

	return ch
}

func (prvdr *AnthropicProvider) convertMessages(
	task *a2a.Task,
) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(task.History))

	for _, msg := range task.History {
		var text string

		for _, p := range msg.Parts {
			if p.Type == a2a.PartTypeText {
				text = p.Text
				break
			}
		}

		if fn, ok := anthropicRoleMap[msg.Role]; ok {
			out = append(out, fn(text))
		}
	}
	return out
}

func (prvdr *AnthropicProvider) convertTools(
	tools []*mcp.Tool,
) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))

	for _, tool := range tools {
		if tool == nil {
			continue
		}

		toolParam := anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: tool.InputSchema.Properties,
			},
		}

		out = append(out, anthropic.ToolUnionParam{OfTool: &toolParam})
	}

	return out
}

type AnthropicEmbedder struct {
	api   anthropic.Client
	Model string
}

type AnthropicEmbedderOption func(*AnthropicEmbedder)

func NewAnthropicEmbedder(options ...AnthropicEmbedderOption) *AnthropicEmbedder {
	embedder := &AnthropicEmbedder{}

	for _, option := range options {
		option(embedder)
	}

	return embedder
}

func (e *AnthropicEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Note: Anthropic doesn't have a direct embedding API like OpenAI
	// This is a placeholder implementation
	return nil, fmt.Errorf("embeddings not supported by Anthropic")
}

func (e *AnthropicEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Note: Anthropic doesn't have a direct embedding API like OpenAI
	// This is a placeholder implementation
	return nil, fmt.Errorf("embeddings not supported by Anthropic")
}

func WithAnthropicClient() AnthropicProviderOption {
	return func(prvdr *AnthropicProvider) {
		client := anthropic.NewClient(
			option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
		)

		prvdr.client = &client
	}
}

func WithAnthropicEmbedderModel(model string) AnthropicEmbedderOption {
	return func(e *AnthropicEmbedder) {
		e.Model = model
	}
}

func WithAnthropicEmbedderClient(client *anthropic.Client) AnthropicEmbedderOption {
	return func(e *AnthropicEmbedder) {
		e.api = *client
	}
}
