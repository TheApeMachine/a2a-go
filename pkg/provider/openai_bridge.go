package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/theapemachine/a2a-go/pkg/types"
)

// DefaultModel is used when the caller does not specify a model explicitly.
const DefaultModel = openai.ChatModelGPT4oMini

// ToolExecutor abstracts the execution of an MCP tool.  Users should implement
// this to wire in their own business logic / data sources.
type ToolExecutor func(
	ctx context.Context, tool mcp.Tool, args map[string]any,
) (string, error)

// ChatClient wraps an *openai.Client and provides convenience methods for
// executing non‑streaming or streaming chat completions while automatically
// converting between A2A objects, MCP tools, and the OpenAI function‑calling
// interface.
type ChatClient struct {
	OpenAI  openai.Client
	Model   string
	Execute ToolExecutor
}

// NewChatClient returns a new ChatClient with sensible defaults.
func NewChatClient(executor ToolExecutor) *ChatClient {
	return &ChatClient{
		OpenAI:  openai.NewClient(),
		Model:   DefaultModel,
		Execute: executor,
	}
}

// Complete runs a synchronous (non‑streaming) chat completion for the given A2A
// message history.  If the assistant returns a tool call it is executed via the
// provided ToolExecutor and the conversation auto‑continues until the final
// assistant reply no longer contains tool calls.
func (c *ChatClient) Complete(
	ctx context.Context, messages []types.Message, tools []mcp.Tool,
) (string, error) {
	oaMsgs := convertMessages(messages)
	oaTools := convertTools(tools)

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(c.modelName()),
		Messages: oaMsgs,
		Tools:    oaTools,
	}

	for {
		resp, err := c.OpenAI.Chat.Completions.New(ctx, params)

		if err != nil {
			return "", err
		}

		msg := resp.Choices[0].Message

		// No tool call? return text.
		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		// Otherwise execute each tool call serially and continue the loop.
		for _, tc := range msg.ToolCalls {
			tool, ok := findTool(tools, tc.Function.Name)

			if !ok {
				return "", fmt.Errorf("unknown tool called: %s", tc.Function.Name)
			}

			var args map[string]any

			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return "", fmt.Errorf("malformed tool args: %w", err)
			}

			if c.Execute == nil {
				return "", errors.New("tool executor not configured")
			}

			result, err := c.Execute(ctx, tool, args)

			if err != nil {
				return "", err
			}

			oaToolMsg := openai.ToolMessage(result, tc.ID)
			params.Messages = append(params.Messages, msg.ToParam(), oaToolMsg)
		}
	}
}

// Stream executes a streaming chat completion.  Tokens are delivered through
// the provided callback.  Once the stream completes the final content string is
// returned.  Tool‑calling is handled after the first assistant message is fully
// streamed (OpenAI currently does not stream function call arguments token by
// token but sends them in a single delta once finished).
func (c *ChatClient) Stream(
	ctx context.Context,
	messages []types.Message,
	tools []mcp.Tool,
	onDelta func(string),
) (string, error) {
	oaMsgs := convertMessages(messages)
	oaTools := convertTools(tools)

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(c.modelName()),
		Messages: oaMsgs,
		Tools:    oaTools,
	}

	var finalContent string

	stream := c.OpenAI.Chat.Completions.NewStreaming(ctx, params)

	for stream.Next() {
		evt := stream.Current()

		if len(evt.Choices) == 0 {
			continue
		}

		delta := evt.Choices[0].Delta

		if delta.Content != "" {
			onDelta(delta.Content)
			finalContent += delta.Content
		}
	}

	if err := stream.Err(); err != nil {
		return "", err
	}

	return finalContent, nil
}

func (c *ChatClient) modelName() string {
	if c.Model == "" {
		return DefaultModel
	}

	return c.Model
}

func convertMessages(
	mm []types.Message,
) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(mm))

	for _, m := range mm {
		text := ""

		for _, p := range m.Parts {
			if p.Type == types.PartTypeText {
				text = p.Text
				break
			}
		}

		if m.Role == "agent" {
			out = append(out, openai.AssistantMessage(text))
		} else {
			out = append(out, openai.UserMessage(text))
		}
	}

	return out
}

func convertTools(
	tools []mcp.Tool,
) []openai.ChatCompletionToolParam {
	out := make([]openai.ChatCompletionToolParam, 0, len(tools))

	for _, t := range tools {
		var paramSchema map[string]any

		if t.RawInputSchema != nil {
			_ = json.Unmarshal(t.RawInputSchema, &paramSchema)
		} else {
			b, _ := json.Marshal(t.InputSchema)
			_ = json.Unmarshal(b, &paramSchema)
		}

		out = append(out, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  openai.FunctionParameters(paramSchema),
			},
		})
	}

	return out
}

func findTool(tools []mcp.Tool, name string) (mcp.Tool, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}

	return mcp.Tool{}, false
}
