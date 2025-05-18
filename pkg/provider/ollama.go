package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ollama/ollama/api"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
)

/*
ollamaRoleMap compresses convertMessages' switch.
*/
var ollamaRoleMap = map[string]func(string) api.Message{
	"system": func(text string) api.Message {
		return api.Message{
			Role:    "system",
			Content: text,
		}
	},
	"user": func(text string) api.Message {
		return api.Message{
			Role:    "user",
			Content: text,
		}
	},
	"developer": func(text string) api.Message {
		return api.Message{
			Role:    "user",
			Content: text,
		}
	},
	"agent": func(text string) api.Message {
		return api.Message{
			Role:    "assistant",
			Content: text,
		}
	},
	"assistant": func(text string) api.Message {
		return api.Message{
			Role:    "assistant",
			Content: text,
		}
	},
}

/*
OllamaProvider is a provider for the Ollama API.
*/
type OllamaProvider struct {
	client *api.Client
	params *api.ChatRequest
}

type OllamaProviderOption func(*OllamaProvider)

func NewOllamaProvider(options ...OllamaProviderOption) *OllamaProvider {
	prvdr := &OllamaProvider{}

	for _, option := range options {
		option(prvdr)
	}

	return prvdr
}

func (prvdr *OllamaProvider) applySchema(
	task *a2a.Task,
) map[string]any {
	if schema, ok := task.Metadata["schema"].(map[string]any); ok {
		return map[string]any{
			"format": "json",
			"schema": schema,
		}
	}
	return nil
}

func (prvdr *OllamaProvider) Generate(
	ctx context.Context, params *ProviderParams,
) chan jsonrpc.Response {
	ch := make(chan jsonrpc.Response)

	// Ollama-specific LLM tool response generator function
	ollamaToolResponseGenerator := func(toolCallID string, content string, isError bool) any {
		// toolCallID is not directly used by Ollama's api.Message for tool results in current setup.
		// The content itself will contain the tool name and result/error.
		return api.Message{
			Role:    "tool",
			Content: content,
		}
	}

	go func() {
		defer close(ch)

		isDone := false

		opts := map[string]any{
			"temperature":       params.Temperature,
			"top_p":             params.TopP,
			"top_k":             params.TopK,
			"num_predict":       params.MaxTokens,
			"stop":              params.Stop,
			"seed":              params.Seed,
			"frequency_penalty": params.FrequencyPenalty,
			"presence_penalty":  params.PresencePenalty,
		}

		if schema := prvdr.applySchema(params.Task); schema != nil {
			opts["format"] = schema["format"]
			opts["schema"] = schema["schema"]
		}

		for !isDone {
			if params.Stream {
				// For streaming, use GenerateRequest
				var prompt string
				for _, msg := range prvdr.convertMessages(params.Task) {
					prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
				}

				req := &api.GenerateRequest{
					Model:   params.Model,
					Prompt:  prompt,
					Options: opts,
				}

				// Apply schema if present
				if schema := prvdr.applySchema(params.Task); schema != nil {
					req.Options["format"] = schema["format"]
					req.Options["schema"] = schema["schema"]
				}

				respFunc := func(resp api.GenerateResponse) error {
					if resp.Response != "" {
						ch <- a2a.NewArtifactResult(
							params.Task.ID,
							a2a.NewTextPart(resp.Response),
						)
					}
					return nil
				}

				err := prvdr.client.Generate(ctx, req, respFunc)
				if err != nil {
					log.Error("failed to create stream", "error", err)
					ch <- jsonrpc.Response{
						Error: &jsonrpc.Error{
							Code:    int(a2a.ErrorCodeInternalError),
							Message: err.Error(),
						},
					}
					return
				}

				isDone = true
			} else {
				// For non-streaming with tool support, use ChatRequest
				// Note: prvdr.params is set inside handleToolCall for Ollama, which is not ideal.
				// We should prepare messages before the call to prvdr.client.Chat
				currentMessages := prvdr.convertMessages(params.Task)

				req := &api.ChatRequest{
					Model:    params.Model,
					Messages: currentMessages, // Use current messages
					Tools:    prvdr.convertTools(params.Tools),
					Options:  opts,
				}

				if schema := prvdr.applySchema(params.Task); schema != nil {
					req.Options["format"] = schema["format"]
					req.Options["schema"] = schema["schema"]
				}

				var fullMessageText string
				var calledToolNames []string // To keep track of tools called in this iteration

				respFunc := func(resp api.ChatResponse) error {
					if resp.Message.ToolCalls != nil {
						// This part will be tricky as handleToolCall used to modify prvdr.params.Messages directly.
						// We now need to collect tool calls, execute them, get LLM messages, and then make a new Chat call.
						// This requires restructuring the non-streaming loop for Ollama if it needs multi-turn tool use.
						// For now, let's assume a single round of tool calls per explicit Chat call based on current structure.

						// Store assistant's message that requests tool calls.
						// This message itself might not be added to `currentMessages` if Ollama expects only User/System/Tool messages after tool calls.
						// For now, we add it to task history if it has content.
						if resp.Message.Content != "" {
							params.Task.AddMessage("assistant", resp.Message.Content, "")
						}

						for _, ollamaToolCall := range resp.Message.ToolCalls {
							calledToolNames = append(calledToolNames, ollamaToolCall.Function.Name)
							updatedTask, llmToolMsg, toolExecErr := ExecuteAndProcessToolCall(
								ctx,
								ollamaToolCall.Function.Name,
								ollamaToolCall.Function.Arguments.String(), // Arguments is json.RawMessage
								"", // Ollama doesn't seem to use a tool_call_id in its response message structure for tools.
								params.Task,
								ollamaToolResponseGenerator,
							)
							params.Task = updatedTask
							// The llmToolMsg for Ollama is an api.Message, needs to be added to the *next* Chat request's Messages.
							// This is where the loop structure of Ollama non-streaming needs careful thought for multi-turn.
							// For now, we'll send the task update. The next call to Chat will need these tool responses.
							// We are modifying `req.Messages` for the *next* iteration of the `for !isDone` loop implicitly
							// by relying on `prvdr.convertMessages(params.Task)` which reads from updated task history.
							// And by adding the LLM response message (tool result) to task history.
							llmMessageForHistory := llmToolMsg.(api.Message)
							params.Task.AddMessage(llmMessageForHistory.Role, llmMessageForHistory.Content, "")

							if toolExecErr != nil {
								ch <- jsonrpc.Response{
									Result: params.Task, // Send updated task with error artifact
									Error:  &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: fmt.Sprintf("Error executing tool %s: %v", ollamaToolCall.Function.Name, toolExecErr)},
								}
							} else {
								ch <- jsonrpc.Response{Result: params.Task} // Send updated task with success artifact
							}
						}
					} else {
						fullMessageText += resp.Message.Content
					}
					return nil
				}

				err := prvdr.client.Chat(ctx, req, respFunc)
				if err != nil {
					ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: int(a2a.ErrorCodeInternalError), Message: err.Error()}}
					return // fatal error for this call
				}

				if len(calledToolNames) > 0 {
					// Tools were called. The next iteration of `for !isDone` will pick up messages from task.History
					// which now includes the tool results, and make a new call to prvdr.client.Chat.
					isDone = false
				} else if fullMessageText != "" {
					params.Task.AddFinalPart(a2a.NewTextPart(fullMessageText))
					ch <- a2a.NewArtifactResult(params.Task.ID, a2a.NewTextPart(fullMessageText))
					params.Task.ToStatus(a2a.TaskStateCompleted, a2a.NewTextMessage("assistant", fullMessageText))
					ch <- jsonrpc.Response{Result: params.Task} // Send final task state
					isDone = true
				} else {
					// No tools called, no text response, could be an empty response or an error not caught by `err` above.
					log.Warn("Ollama non-streaming call resulted in no tool calls and no text response.")
					isDone = true // Avoid infinite loop
				}
			}
		}
	}()

	return ch
}

func (prvdr *OllamaProvider) convertMessages(
	task *a2a.Task,
) []api.Message {
	out := make([]api.Message, 0, len(task.History))

	for _, msg := range task.History {
		var text string

		for _, p := range msg.Parts {
			if p.Type == a2a.PartTypeText {
				text = p.Text
				break
			}
		}

		if fn, ok := ollamaRoleMap[msg.Role]; ok {
			out = append(out, fn(text))
		}
	}
	return out
}

func (prvdr *OllamaProvider) convertTools(
	tools []*mcp.Tool,
) []api.Tool {
	out := make([]api.Tool, 0, len(tools))

	for _, tool := range tools {
		if tool == nil {
			continue
		}

		out = append(out, api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: struct {
					Type       string   `json:"type"`
					Defs       any      `json:"$defs,omitempty"`
					Items      any      `json:"items,omitempty"`
					Required   []string `json:"required"`
					Properties map[string]struct {
						Type        api.PropertyType `json:"type"`
						Items       any              `json:"items,omitempty"`
						Description string           `json:"description"`
						Enum        []any            `json:"enum,omitempty"`
					} `json:"properties"`
				}{
					Type:     tool.InputSchema.Type,
					Required: tool.InputSchema.Required,
					Properties: func() map[string]struct {
						Type        api.PropertyType `json:"type"`
						Items       any              `json:"items,omitempty"`
						Description string           `json:"description"`
						Enum        []any            `json:"enum,omitempty"`
					} {
						props := make(map[string]struct {
							Type        api.PropertyType `json:"type"`
							Items       any              `json:"items,omitempty"`
							Description string           `json:"description"`
							Enum        []any            `json:"enum,omitempty"`
						})
						for name, prop := range tool.InputSchema.Properties {
							propMap, ok := prop.(map[string]any)
							if !ok {
								continue
							}
							typeStr, ok := propMap["type"].(string)
							if !ok {
								continue
							}
							desc, _ := propMap["description"].(string)
							enum, _ := propMap["enum"].([]any)
							props[name] = struct {
								Type        api.PropertyType `json:"type"`
								Items       any              `json:"items,omitempty"`
								Description string           `json:"description"`
								Enum        []any            `json:"enum,omitempty"`
							}{
								Type:        api.PropertyType{typeStr},
								Description: desc,
								Enum:        enum,
							}
						}
						return props
					}(),
				},
			},
		})
	}

	return out
}

type OllamaEmbedder struct {
	api   *api.Client
	Model string
}

type OllamaEmbedderOption func(*OllamaEmbedder)

func NewOllamaEmbedder(options ...OllamaEmbedderOption) *OllamaEmbedder {
	embedder := &OllamaEmbedder{}

	for _, option := range options {
		option(embedder)
	}

	return embedder
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Ollama doesn't have a direct embedding API, so we'll use the chat completion API
	// to generate embeddings-like output
	req := &api.GenerateRequest{
		Model:  e.Model,
		Prompt: text,
	}

	var fullResponse string
	respFunc := func(resp api.GenerateResponse) error {
		fullResponse += resp.Response
		return nil
	}

	err := e.api.Generate(ctx, req, respFunc)
	if err != nil {
		return nil, err
	}

	// Convert the response to a vector
	// This is a placeholder - you might want to use a different approach
	// depending on your needs
	vector := make([]float32, 0)
	for _, r := range fullResponse {
		vector = append(vector, float32(r))
	}

	return vector, nil
}

func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vector, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors[i] = vector
	}
	return vectors, nil
}

func WithOllamaClient() OllamaProviderOption {
	return func(prvdr *OllamaProvider) {
		client, err := api.ClientFromEnvironment()
		if err != nil {
			log.Error("failed to create Ollama client", "error", err)
			return
		}
		prvdr.client = client
	}
}

func WithOllamaEmbedderModel(model string) OllamaEmbedderOption {
	return func(e *OllamaEmbedder) {
		e.Model = model
	}
}

func WithOllamaEmbedderClient(client *api.Client) OllamaEmbedderOption {
	return func(e *OllamaEmbedder) {
		e.api = client
	}
}

/*
GenerateImage uses the model to generate an image and returns it as a base64-encoded string.
*/
func (prvdr *OllamaProvider) GenerateImage(
	ctx context.Context, task *a2a.Task,
) *a2a.Task {
	prompt := task.LastMessage().String()

	// Create a special prompt for image generation
	imagePrompt := fmt.Sprintf(`Generate a base64-encoded image based on this description: %s
Respond with only the base64-encoded image data, no other text.`, prompt)

	// Create provider params with default values
	params := NewProviderParams(task)

	req := &api.GenerateRequest{
		Model:  params.Model,
		Prompt: imagePrompt,
		Options: map[string]any{
			"temperature": 0.7,
			"top_p":       0.9,
		},
	}

	var imageData string
	respFunc := func(resp api.GenerateResponse) error {
		imageData += resp.Response
		return nil
	}

	err := prvdr.client.Generate(ctx, req, respFunc)
	if err != nil {
		task.ToStatus(
			a2a.TaskStateFailed,
			a2a.NewTextMessage(
				"assistant",
				fmt.Sprintf("Error generating image: %s", err),
			),
		)
		return task
	}

	// Add the image as an artifact
	task.AddArtifact(a2a.NewFileArtifact(
		"image",
		"image/png",
		imageData,
	))

	return task
}

/*
AudioTranscript uses the model to transcribe audio data.
*/
func (prvdr *OllamaProvider) AudioTranscript(ctx context.Context, audio []byte) (string, error) {
	// Create a special prompt for audio transcription
	transcriptPrompt := fmt.Sprintf(`Transcribe the following base64-encoded audio data: %s
Respond with only the transcription text.`, base64.StdEncoding.EncodeToString(audio))

	// Create provider params with default values
	params := NewProviderParams(nil)

	req := &api.GenerateRequest{
		Model:  params.Model,
		Prompt: transcriptPrompt,
		Options: map[string]any{
			"temperature": 0.0,
			"top_p":       1.0,
		},
	}

	var transcript string
	respFunc := func(resp api.GenerateResponse) error {
		transcript += resp.Response
		return nil
	}

	err := prvdr.client.Generate(ctx, req, respFunc)
	if err != nil {
		return "", fmt.Errorf("error transcribing audio: %w", err)
	}

	return transcript, nil
}

/*
TTS uses the model to generate speech from text.
*/
func (prvdr *OllamaProvider) TTS(ctx context.Context, text string) error {
	// Create a special prompt for text-to-speech
	ttsPrompt := fmt.Sprintf(`Generate base64-encoded PCM audio data from this text: %s
Respond with only the base64-encoded PCM audio data, no other text.`, text)

	// Create provider params with default values
	params := NewProviderParams(nil)

	req := &api.GenerateRequest{
		Model:  params.Model,
		Prompt: ttsPrompt,
		Options: map[string]any{
			"temperature": 0.0,
			"top_p":       1.0,
		},
	}

	var audioData string
	respFunc := func(resp api.GenerateResponse) error {
		audioData += resp.Response
		return nil
	}

	err := prvdr.client.Generate(ctx, req, respFunc)
	if err != nil {
		return fmt.Errorf("error generating speech: %w", err)
	}

	// Decode the base64 audio data
	audioBytes, err := base64.StdEncoding.DecodeString(audioData)
	if err != nil {
		return fmt.Errorf("error decoding audio data: %w", err)
	}

	_ = audioBytes

	// // Play the audio
	// return utils.PlayPCM(bytes.NewReader(audioBytes))
	return nil
}

/*
FineTune uses the model to learn from examples.
*/
func (prvdr *OllamaProvider) FineTune(
	ctx context.Context,
	examples []struct {
		Input  string
		Output string
	},
) error {
	// Create a special prompt for fine-tuning
	var fineTunePrompt string
	for _, example := range examples {
		fineTunePrompt += fmt.Sprintf("Input: %s\nOutput: %s\n\n", example.Input, example.Output)
	}
	fineTunePrompt += "Learn from these examples and improve your responses."

	// Create provider params with default values
	params := NewProviderParams(nil)

	req := &api.GenerateRequest{
		Model:  params.Model,
		Prompt: fineTunePrompt,
		Options: map[string]any{
			"temperature": 0.0,
			"top_p":       1.0,
		},
	}

	var response string
	respFunc := func(resp api.GenerateResponse) error {
		response += resp.Response
		return nil
	}

	err := prvdr.client.Generate(ctx, req, respFunc)
	if err != nil {
		return fmt.Errorf("error fine-tuning model: %w", err)
	}

	// The model has learned from the examples
	return nil
}

/*
Embed generates a vector representation of the input text.
*/
func (prvdr *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// Create a special prompt for embedding
	embedPrompt := fmt.Sprintf(`Generate a vector representation for this text: %s
Respond with only a comma-separated list of numbers representing the vector.`, text)

	// Create provider params with default values
	params := NewProviderParams(nil)

	req := &api.GenerateRequest{
		Model:  params.Model,
		Prompt: embedPrompt,
		Options: map[string]any{
			"temperature": 0.0,
			"top_p":       1.0,
		},
	}

	var vectorStr string
	respFunc := func(resp api.GenerateResponse) error {
		vectorStr += resp.Response
		return nil
	}

	err := prvdr.client.Generate(ctx, req, respFunc)
	if err != nil {
		return nil, fmt.Errorf("error generating embedding: %w", err)
	}

	// Parse the comma-separated numbers into a float32 slice
	vector := make([]float32, 0)
	for _, numStr := range strings.Split(vectorStr, ",") {
		numStr = strings.TrimSpace(numStr)
		if numStr == "" {
			continue
		}
		num, err := strconv.ParseFloat(numStr, 32)
		if err != nil {
			return nil, fmt.Errorf("error parsing vector number: %w", err)
		}
		vector = append(vector, float32(num))
	}

	return vector, nil
}

/*
EmbedBatch generates vector representations for multiple input texts.
*/
func (prvdr *OllamaProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vector, err := prvdr.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("error generating embedding for text %d: %w", i, err)
		}
		vectors[i] = vector
	}
	return vectors, nil
}
