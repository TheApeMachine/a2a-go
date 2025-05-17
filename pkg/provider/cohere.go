package provider

import (
	"context"
	"encoding/json"
	"os"

	"github.com/charmbracelet/log"
	cohere "github.com/cohere-ai/cohere-go/v2"
	cohereclient "github.com/cohere-ai/cohere-go/v2/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/tools"
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

	go func() {
		defer close(ch)

		model := params.Model
		maxTokens := int(params.MaxTokens)
		temperature := params.Temperature

		prvdr.params = &cohere.ChatRequest{
			Model:         &model,
			Message:       prvdr.convertMessages(params.Task),
			Tools:         prvdr.convertTools(params.Tools),
			MaxTokens:     &maxTokens,
			Temperature:   &temperature,
			StopSequences: params.Stop,
		}

		isDone := false

		for !isDone {
			if params.Stream {
				streamParams := &cohere.ChatStreamRequest{
					Model:         prvdr.params.Model,
					Message:       prvdr.params.Message,
					Tools:         prvdr.params.Tools,
					MaxTokens:     prvdr.params.MaxTokens,
					Temperature:   prvdr.params.Temperature,
					StopSequences: prvdr.params.StopSequences,
				}

				stream, err := prvdr.client.ChatStream(ctx, streamParams)
				if err != nil {
					log.Error("failed to create chat stream", "error", err)
					ch <- jsonrpc.Response{
						Error: &jsonrpc.Error{
							Code:    int(a2a.ErrorCodeInternalError),
							Message: err.Error(),
						},
					}
					return
				}

				for {
					message, err := stream.Recv()
					if err != nil {
						if err.Error() == "EOF" {
							break
						}
						log.Error("stream error", "error", err)
						ch <- jsonrpc.Response{
							Error: &jsonrpc.Error{
								Code:    int(a2a.ErrorCodeInternalError),
								Message: err.Error(),
							},
						}
						return
					}

					if message.GetTextGeneration() != nil {
						ch <- a2a.NewArtifactResult(
							params.Task.ID,
							a2a.NewTextPart(message.GetTextGeneration().GetText()),
						)
					}

					if message.GetToolCallsGeneration() != nil {
						for _, toolCall := range message.GetToolCallsGeneration().ToolCalls {
							err := prvdr.handleToolCall(ctx, toolCall, ch, params.Task)
							if err != nil {
								log.Error("error handling tool call", "error", err)
								continue
							}
						}
					}
				}
			} else {
				response, err := prvdr.client.Chat(ctx, prvdr.params)
				if err != nil {
					log.Error("failed to generate chat response", "error", err)
					ch <- jsonrpc.Response{
						Error: &jsonrpc.Error{
							Code:    int(a2a.ErrorCodeInternalError),
							Message: err.Error(),
						},
					}
					return
				}

				if response.GetText() != "" {
					ch <- a2a.NewArtifactResult(
						params.Task.ID,
						a2a.NewTextPart(response.GetText()),
					)
					params.Task.AddFinalPart(a2a.NewTextPart(response.GetText()))
				}

				if response.GetToolCalls() != nil {
					for _, toolCall := range response.GetToolCalls() {
						err := prvdr.handleToolCall(ctx, toolCall, ch, params.Task)
						if err != nil {
							log.Error("error handling tool call", "error", err)
							continue
						}
					}
				}

				isDone = true
			}
		}
	}()

	return ch
}

func (prvdr *CohereProvider) handleToolCall(
	ctx context.Context,
	toolCall *cohere.ToolCall,
	out chan jsonrpc.Response,
	task *a2a.Task,
) error {
	params, err := json.Marshal(toolCall.Parameters)
	if err != nil {
		return err
	}

	results, err := tools.NewExecutor(
		ctx, toolCall.Name, string(params),
	)

	if err != nil {
		log.Error("error executing tool", "error", err)

		prvdr.params.Message = prvdr.params.Message + "\n" + err.Error()

		return err
	}

	prvdr.params.Message = prvdr.params.Message + "\n" + results

	out <- jsonrpc.Response{
		Result: a2a.TaskStatusUpdateResult{
			ID:       task.ID,
			Status:   a2a.TaskStatus{State: a2a.TaskStateWorking},
			Final:    false,
			Metadata: map[string]any{},
		},
	}

	return nil
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
