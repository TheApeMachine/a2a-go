package provider

import (
	"context"
	"os"

	"github.com/charmbracelet/log"
	deepseek "github.com/cohesion-org/deepseek-go"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
)

/*
deepseekRoleMap compresses convertMessages' switch.
*/
var deepseekRoleMap = map[string]func(string) deepseek.ChatCompletionMessage{
	"system": func(text string) deepseek.ChatCompletionMessage {
		return deepseek.ChatCompletionMessage{
			Role:    deepseek.ChatMessageRoleSystem,
			Content: text,
		}
	},
	"user": func(text string) deepseek.ChatCompletionMessage {
		return deepseek.ChatCompletionMessage{
			Role:    deepseek.ChatMessageRoleUser,
			Content: text,
		}
	},
	"developer": func(text string) deepseek.ChatCompletionMessage {
		return deepseek.ChatCompletionMessage{
			Role:    deepseek.ChatMessageRoleUser,
			Content: text,
		}
	},
	"agent": func(text string) deepseek.ChatCompletionMessage {
		return deepseek.ChatCompletionMessage{
			Role:    deepseek.ChatMessageRoleAssistant,
			Content: text,
		}
	},
	"assistant": func(text string) deepseek.ChatCompletionMessage {
		return deepseek.ChatCompletionMessage{
			Role:    deepseek.ChatMessageRoleAssistant,
			Content: text,
		}
	},
}

/*
DeepseekProvider is a provider for the Deepseek API.
*/
type DeepseekProvider struct {
	client *deepseek.Client
	params *deepseek.ChatCompletionRequest
}

type DeepseekProviderOption func(*DeepseekProvider)

func NewDeepseekProvider(options ...DeepseekProviderOption) *DeepseekProvider {
	prvdr := &DeepseekProvider{}

	for _, option := range options {
		option(prvdr)
	}

	return prvdr
}

func (prvdr *DeepseekProvider) Generate(
	ctx context.Context, params *ProviderParams,
) chan jsonrpc.Response {
	ch := make(chan jsonrpc.Response)

	go func() {
		defer close(ch)

		prvdr.params = &deepseek.ChatCompletionRequest{
			Model:       deepseek.DeepSeekChat,
			Messages:    prvdr.convertMessages(params.Task),
			Tools:       prvdr.convertTools(params.Tools),
			Temperature: float32(params.Temperature),
			TopP:        float32(params.TopP),
			MaxTokens:   int(params.MaxTokens),
			Stop:        params.Stop,
		}

		isDone := false

		for !isDone {
			if params.Stream {
				streamReq := &deepseek.StreamChatCompletionRequest{
					Model:       prvdr.params.Model,
					Messages:    prvdr.params.Messages,
					Tools:       prvdr.params.Tools,
					Temperature: prvdr.params.Temperature,
					TopP:        prvdr.params.TopP,
					MaxTokens:   prvdr.params.MaxTokens,
					Stop:        prvdr.params.Stop,
					Stream:      true,
				}

				stream, err := prvdr.client.CreateChatCompletionStream(ctx, streamReq)
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
				defer stream.Close()

				var fullMessage string
				for {
					response, err := stream.Recv()
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

					for _, choice := range response.Choices {
						fullMessage += choice.Delta.Content
						ch <- a2a.NewArtifactResult(
							params.Task.ID,
							a2a.NewTextPart(choice.Delta.Content),
						)
					}
				}

				params.Task.AddFinalPart(a2a.NewTextPart(fullMessage))
				isDone = true
			} else {
				response, err := prvdr.client.CreateChatCompletion(ctx, prvdr.params)
				if err != nil {
					log.Error("failed to generate completion", "error", err)
					ch <- jsonrpc.Response{
						Error: &jsonrpc.Error{
							Code:    int(a2a.ErrorCodeInternalError),
							Message: err.Error(),
						},
					}
					return
				}

				if len(response.Choices) > 0 {
					content := response.Choices[0].Message.Content
					ch <- a2a.NewArtifactResult(
						params.Task.ID,
						a2a.NewTextPart(content),
					)
					params.Task.AddFinalPart(a2a.NewTextPart(content))
				}

				isDone = true
			}
		}
	}()

	return ch
}

func (prvdr *DeepseekProvider) convertMessages(
	task *a2a.Task,
) []deepseek.ChatCompletionMessage {
	out := make([]deepseek.ChatCompletionMessage, 0, len(task.History))

	for _, msg := range task.History {
		var text string

		for _, p := range msg.Parts {
			if p.Type == a2a.PartTypeText {
				text = p.Text
				break
			}
		}

		if fn, ok := deepseekRoleMap[msg.Role]; ok {
			out = append(out, fn(text))
		}
	}
	return out
}

func (prvdr *DeepseekProvider) convertTools(
	tools []*mcp.Tool,
) []deepseek.Tool {
	out := make([]deepseek.Tool, 0, len(tools))

	for _, tool := range tools {
		if tool == nil {
			continue
		}

		out = append(out, deepseek.Tool{
			Type: "function",
			Function: deepseek.Function{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters: &deepseek.FunctionParameters{
					Type:       tool.InputSchema.Type,
					Properties: tool.InputSchema.Properties,
				},
			},
		})
	}

	return out
}

type DeepseekEmbedder struct {
	api   *deepseek.Client
	Model string
}

type DeepseekEmbedderOption func(*DeepseekEmbedder)

func NewDeepseekEmbedder(options ...DeepseekEmbedderOption) *DeepseekEmbedder {
	embedder := &DeepseekEmbedder{}

	for _, option := range options {
		option(embedder)
	}

	return embedder
}

func (e *DeepseekEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Deepseek doesn't have a direct embedding API, so we'll use the chat completion API
	// to generate embeddings-like output
	request := &deepseek.ChatCompletionRequest{
		Model: e.Model,
		Messages: []deepseek.ChatCompletionMessage{
			{
				Role:    deepseek.ChatMessageRoleUser,
				Content: text,
			},
		},
	}

	response, err := e.api.CreateChatCompletion(ctx, request)
	if err != nil {
		return nil, err
	}

	// Convert the response to a vector
	// This is a placeholder - you might want to use a different approach
	// depending on your needs
	vector := make([]float32, 0)
	for _, r := range response.Choices[0].Message.Content {
		vector = append(vector, float32(r))
	}

	return vector, nil
}

func (e *DeepseekEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
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

func WithDeepseekClient() DeepseekProviderOption {
	return func(prvdr *DeepseekProvider) {
		client := deepseek.NewClient(os.Getenv("DEEPSEEK_API_KEY"))
		prvdr.client = client
	}
}

func WithDeepseekEmbedderModel(model string) DeepseekEmbedderOption {
	return func(e *DeepseekEmbedder) {
		e.Model = model
	}
}

func WithDeepseekEmbedderClient(client *deepseek.Client) DeepseekEmbedderOption {
	return func(e *DeepseekEmbedder) {
		e.api = client
	}
}
