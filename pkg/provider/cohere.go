package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	cohere "github.com/cohere-ai/cohere-go/v2"
	cohereclient "github.com/cohere-ai/cohere-go/v2/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
cohereRoleMap compresses convertMessages' switch.
*/
var cohereRoleMap = map[string]func(string) *cohere.ChatMessage{
	"system":    func(text string) *cohere.ChatMessage { return &cohere.ChatMessage{Message: text} },
	"user":      func(text string) *cohere.ChatMessage { return &cohere.ChatMessage{Message: text} },
	"developer": func(text string) *cohere.ChatMessage { return &cohere.ChatMessage{Message: text} },
	"agent":     func(text string) *cohere.ChatMessage { return &cohere.ChatMessage{Message: text} },
	"assistant": func(text string) *cohere.ChatMessage { return &cohere.ChatMessage{Message: text} },
}

/*
CohereProvider is a provider for the Cohere API.
*/
type CohereProvider struct {
	client *cohereclient.Client
	params *cohere.ChatRequest
}

type CohereProviderOption func(*CohereProvider)

func NewCohereProvider(options ...CohereProviderOption) *CohereProvider {
	prvdr := &CohereProvider{}

	for _, option := range options {
		option(prvdr)
	}

	return prvdr
}

func (prvdr *CohereProvider) Generate(
	ctx context.Context, params *ProviderParams,
) chan jsonrpc.Response {
	ch := make(chan jsonrpc.Response)

	// Cohere-specific LLM tool response generator function
	cohereToolResponseGenerator := func(toolCallID string, content string, isError bool) any {
		// For Cohere, the tool result (or error string) is directly appended to the chat history/message.
		// The toolCallID and isError are implicitly part of the `content` string if formatted that way.
		return content
	}

	go func() {
		defer close(ch)

		model := params.Model
		maxTokens := int(params.MaxTokens)
		temperature := params.Temperature

		// Initialize prvdr.params for the first call, or if it doesn't persist across tool calls.
		// Cohere's ChatRequest takes the whole message string, so it's built up.
		currentMessage := prvdr.convertMessages(params.Task) // Start with history

		isDone := false
		for !isDone {
			prvdr.params = &cohere.ChatRequest{
				Model:         &model,
				Message:       currentMessage, // Built-up message string
				Tools:         prvdr.convertTools(params.Tools),
				MaxTokens:     &maxTokens,
				Temperature:   &temperature,
				StopSequences: params.Stop,
			}

			if params.Stream {
				streamParams := &cohere.ChatStreamRequest{
					Model:         prvdr.params.Model,
					Message:       prvdr.params.Message, // Use current built-up message
					Tools:         prvdr.params.Tools,
					MaxTokens:     prvdr.params.MaxTokens,
					Temperature:   prvdr.params.Temperature,
					StopSequences: prvdr.params.StopSequences,
				}

				stream, err := prvdr.client.ChatStream(ctx, streamParams)
				if err != nil {
					ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: err.Error()}}
					return // Fatal error for stream setup
				}

				var streamTextResponse string
				var streamCalledTools []*cohere.ToolCall

				for {
					streamEvent, recvErr := stream.Recv()
					if recvErr != nil {
						if recvErr.Error() == "EOF" {
							break // End of stream
						}
						ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: recvErr.Error()}}
						streamTextResponse = ""
						streamCalledTools = nil
						break
					}

					if tg := streamEvent.GetTextGeneration(); tg != nil {
						textChunk := tg.GetText()
						streamTextResponse += textChunk
						ch <- a2a.NewArtifactResult(params.Task.ID, a2a.NewTextPart(textChunk))
					}

					if tcg := streamEvent.GetToolCallsGeneration(); tcg != nil {
						streamCalledTools = append(streamCalledTools, tcg.GetToolCalls()...)
					}

					if streamEvent.EventType == "stream-end" {
						break // StreamEnd event signals the end of the current LLM turn's stream.
					}
				}
				// Stream finished for this turn. Process collected tool calls.
				currentMessage += streamTextResponse // Add assistant's text to overall message for next turn
				params.Task.AddMessage("assistant", streamTextResponse, "")

				if len(streamCalledTools) > 0 {
					for _, toolCall := range streamCalledTools {
						toolParamsJSON, _ := json.Marshal(toolCall.Parameters)
						updatedTask, llmToolResultStr, toolExecErr := ExecuteAndProcessToolCall(
							ctx,
							toolCall.Name,
							string(toolParamsJSON),
							"", // Cohere doesn't use tool_call_id in its response like OpenAI
							params.Task,
							cohereToolResponseGenerator,
						)
						params.Task = updatedTask
						currentMessage += "\n" + llmToolResultStr.(string)                       // Append tool result to message for next LLM call
						params.Task.AddMessage("tool", llmToolResultStr.(string), toolCall.Name) // Log tool interaction in task history

						if toolExecErr != nil {
							ch <- jsonrpc.Response{Result: params.Task, Error: &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: fmt.Sprintf("Error executing tool %s: %v", toolCall.Name, toolExecErr)}}
						} else {
							ch <- jsonrpc.Response{Result: params.Task}
						}
					}
					isDone = false // Need to make another call to LLM with tool results
				} else {
					if streamTextResponse != "" { // Final text from stream if no tools were called
						params.Task.AddFinalPart(a2a.NewTextPart(streamTextResponse))
						params.Task.ToStatus(a2a.TaskStateCompleted, a2a.NewTextMessage("assistant", streamTextResponse))
						ch <- jsonrpc.Response{Result: params.Task}
					}
					isDone = true
				}

			} else { // Non-streaming path
				response, err := prvdr.client.Chat(ctx, prvdr.params)
				if err != nil {
					ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: err.Error()}}
					return // Fatal error
				}

				assistantResponseText := response.GetText()
				currentMessage += "\n" + assistantResponseText // Add assistant's text to overall message
				params.Task.AddMessage("assistant", assistantResponseText, "")

				llmToolCalls := response.GetToolCalls()
				if len(llmToolCalls) > 0 {
					for _, toolCall := range llmToolCalls {
						toolParamsJSON, _ := json.Marshal(toolCall.Parameters)
						updatedTask, llmToolResultStr, toolExecErr := ExecuteAndProcessToolCall(
							ctx,
							toolCall.Name,
							string(toolParamsJSON),
							"",
							params.Task,
							cohereToolResponseGenerator,
						)
						params.Task = updatedTask
						currentMessage += "\n" + llmToolResultStr.(string)                       // Append tool result for next LLM call
						params.Task.AddMessage("tool", llmToolResultStr.(string), toolCall.Name) // Log tool interaction

						if toolExecErr != nil {
							ch <- jsonrpc.Response{Result: params.Task, Error: &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: fmt.Sprintf("Error executing tool %s: %v", toolCall.Name, toolExecErr)}}
						} else {
							ch <- jsonrpc.Response{Result: params.Task}
						}
					}
					isDone = false // Loop again to send tool results to Cohere
				} else {
					if assistantResponseText != "" { // Final text response if no tools
						params.Task.AddFinalPart(a2a.NewTextPart(assistantResponseText))
						ch <- a2a.NewArtifactResult(params.Task.ID, a2a.NewTextPart(assistantResponseText))
					}
					params.Task.ToStatus(a2a.TaskStateCompleted, a2a.NewTextMessage("assistant", assistantResponseText))
					ch <- jsonrpc.Response{Result: params.Task}
					isDone = true
				}
			}
		}
	}()

	return ch
}

func (prvdr *CohereProvider) convertMessages(
	task *a2a.Task,
) string {
	var messages string
	for _, msg := range task.History {
		var text string
		for _, p := range msg.Parts {
			if p.Type == a2a.PartTypeText {
				text = p.Text
				break
			}
		}
		messages += text + "\n"
	}
	return messages
}

func (prvdr *CohereProvider) convertTools(
	tools []*mcp.Tool,
) []*cohere.Tool {
	out := make([]*cohere.Tool, 0, len(tools))

	for _, tool := range tools {
		if tool == nil {
			continue
		}

		paramDefs := make(map[string]*cohere.ToolParameterDefinitionsValue)
		for name, prop := range tool.InputSchema.Properties {
			propMap := prop.(map[string]any)
			desc := propMap["description"].(string)
			paramDefs[name] = &cohere.ToolParameterDefinitionsValue{
				Description: cohere.String(desc),
				Type:        propMap["type"].(string),
			}
		}

		out = append(out, &cohere.Tool{
			Name:                 tool.Name,
			Description:          tool.Description,
			ParameterDefinitions: paramDefs,
		})
	}

	return out
}

type CohereEmbedder struct {
	api   cohereclient.Client
	Model string
}

type CohereEmbedderOption func(*CohereEmbedder)

func NewCohereEmbedder(options ...CohereEmbedderOption) *CohereEmbedder {
	embedder := &CohereEmbedder{}

	for _, option := range options {
		option(embedder)
	}

	return embedder
}

func (e *CohereEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	model := e.Model
	resp, err := e.api.Embed(ctx, &cohere.EmbedRequest{
		Model: &model,
		Texts: []string{text},
	})
	if err != nil {
		return nil, err
	}
	return utils.ConvertToFloat32(resp.GetEmbeddingsFloats().Embeddings[0]), nil
}

func (e *CohereEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	model := e.Model
	resp, err := e.api.Embed(ctx, &cohere.EmbedRequest{
		Model: &model,
		Texts: texts,
	})
	if err != nil {
		return nil, err
	}

	embeddings := resp.GetEmbeddingsFloats().Embeddings
	out := make([][]float32, len(embeddings))
	for i, embedding := range embeddings {
		out[i] = utils.ConvertToFloat32(embedding)
	}
	return out, nil
}

func WithCohereClient() CohereProviderOption {
	return func(prvdr *CohereProvider) {
		client := cohereclient.NewClient(
			cohereclient.WithToken(os.Getenv("COHERE_API_KEY")),
		)

		prvdr.client = client
	}
}

func WithCohereEmbedderModel(model string) CohereEmbedderOption {
	return func(e *CohereEmbedder) {
		e.Model = model
	}
}

func WithCohereEmbedderClient(client *cohereclient.Client) CohereEmbedderOption {
	return func(e *CohereEmbedder) {
		e.api = *client
	}
}
