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
	"github.com/theapemachine/a2a-go/pkg/tools"
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
				message := anthropic.Message{}

				for stream.Next() {
					event := stream.Current()
					err := message.Accumulate(event)
					if err != nil {
						log.Error("failed to accumulate message", "error", err)
						continue
					}

					switch event := event.AsAny().(type) {
					case anthropic.ContentBlockDeltaEvent:
						if event.Delta.Text != "" {
							ch <- a2a.NewArtifactResult(
								params.Task.ID,
								a2a.NewTextPart(event.Delta.Text),
							)
						}
					case anthropic.ToolUseBlock:
						prvdr.params.Messages = append(
							prvdr.params.Messages,
							message.ToParam(),
						)

						err := prvdr.handleToolCall(ctx, event, ch, params.Task)
						if err != nil {
							log.Error("error handling tool call", "error", err)
							continue
						}
					case anthropic.MessageStopEvent:
						isDone = true
					}
				}

				if stream.Err() != nil {
					log.Error("stream error", "error", stream.Err())
					ch <- jsonrpc.Response{
						Error: &jsonrpc.Error{
							Code:    int(a2a.ErrorCodeInternalError),
							Message: stream.Err().Error(),
						},
					}
				}
			} else {
				message, err := prvdr.client.Messages.New(ctx, *prvdr.params)
				if err != nil {
					log.Error("failed to generate message", "error", err)
					ch <- jsonrpc.Response{
						Error: &jsonrpc.Error{
							Code:    int(a2a.ErrorCodeInternalError),
							Message: err.Error(),
						},
					}
					return
				}

				for _, block := range message.Content {
					switch block := block.AsAny().(type) {
					case anthropic.TextBlock:
						ch <- a2a.NewArtifactResult(
							params.Task.ID,
							a2a.NewTextPart(block.Text),
						)
						params.Task.AddFinalPart(a2a.NewTextPart(block.Text))
					case anthropic.ToolUseBlock:
						prvdr.params.Messages = append(
							prvdr.params.Messages,
							message.ToParam(),
						)

						err := prvdr.handleToolCall(ctx, block, ch, params.Task)
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

func (prvdr *AnthropicProvider) handleToolCall(
	ctx context.Context,
	toolCall anthropic.ToolUseBlock,
	out chan jsonrpc.Response,
	task *a2a.Task,
) error {
	results, err := tools.NewExecutor(
		ctx, toolCall.Name, string(toolCall.Input),
	)

	if err != nil {
		log.Error("error executing tool", "error", err)

		prvdr.params.Messages = append(
			prvdr.params.Messages,
			anthropic.NewUserMessage(anthropic.NewToolResultBlock(toolCall.ID, err.Error(), true)),
		)

		return err
	}

	prvdr.params.Messages = append(
		prvdr.params.Messages,
		anthropic.NewUserMessage(anthropic.NewToolResultBlock(toolCall.ID, results, false)),
	)

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
