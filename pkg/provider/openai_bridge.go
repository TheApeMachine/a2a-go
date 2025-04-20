package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/openai/openai-go"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

// DefaultModel is used when the caller does not specify a model explicitly.
const DefaultModel = openai.ChatModelGPT4oMini

// ToolExecutor abstracts the execution of an MCP tool.  Users should implement
// this to wire in their own business logic / data sources.
type ToolExecutor func(
	ctx context.Context, tool *types.MCPClient, args map[string]any,
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
	ctx context.Context, task *types.Task, tools map[string]*types.MCPClient,
) (err error) {
	var (
		resp   *openai.ChatCompletion
		params = openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(c.modelName()),
			Messages: convertMessages(task.History),
			Tools:    convertTools(tools),
		}
	)

	task.ToState(types.TaskStateWorking, "thinking...")
	artifacts := make([]types.Artifact, 0)

	for task.Status.State == types.TaskStateWorking {
		if resp, err = c.OpenAI.Chat.Completions.New(ctx, params); err != nil {
			task.ToState(types.TaskStateFailed, err.Error())
			return err
		}

		msg := resp.Choices[0].Message
		artifacts = append(artifacts, types.Artifact{
			Parts:    []types.Part{{Type: types.PartTypeText, Text: msg.Content}},
			Index:    0,
			Append:   utils.Ptr(true),
			Metadata: map[string]any{"role": "assistant", "name": c.Model},
		})

		for _, tc := range msg.ToolCalls {
			if err := c.handleToolCall(ctx, &params, task, tools, msg, tc); err != nil {
				return err
			}
		}

		for _, a := range artifacts {
			a.LastChunk = utils.Ptr(true)
			task.AddArtifact(a)
		}

		task.ToState(types.TaskStateCompleted, "completed")
	}

	return nil
}

// Stream executes a streaming chat completion.  Tokens are delivered through
// the provided callback.  Once the stream completes the final content string is
// returned.  Tool‑calling is handled after the first assistant message is fully
// streamed (OpenAI currently does not stream function call arguments token by
// token but sends them in a single delta once finished).
func (c *ChatClient) Stream(
	ctx context.Context,
	messages []types.Message,
	tools map[string]*types.MCPClient,
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

func (client *ChatClient) handleToolCall(
	ctx context.Context,
	params *openai.ChatCompletionNewParams,
	task *types.Task,
	tools map[string]*types.MCPClient,
	msg openai.ChatCompletionMessage,
	tc openai.ChatCompletionMessageToolCall,
) error {
	log.Info("tool call", "tool", tc.Function.Name)
	tool, ok := tools[tc.Function.Name]

	if !ok {
		task.ToState(types.TaskStateFailed, fmt.Sprintf("unknown tool called: %s", tc.Function.Name))
		return fmt.Errorf("unknown tool called: %s", tc.Function.Name)
	}

	var args map[string]any

	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		task.ToState(types.TaskStateFailed, fmt.Sprintf("malformed tool args: %s", err))
		return fmt.Errorf("malformed tool args: %w", err)
	}

	if client.Execute == nil {
		task.ToState(types.TaskStateFailed, "tool executor not configured")
		return errors.New("tool executor not configured")
	}

	result, err := client.Execute(ctx, tool, args)

	if err != nil {
		task.ToState(types.TaskStateFailed, err.Error())
		return err
	}

	oaToolMsg := openai.ToolMessage(result, tc.ID)
	params.Messages = append(params.Messages, msg.ToParam(), oaToolMsg)

	return nil
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
	tools map[string]*types.MCPClient,
) []openai.ChatCompletionToolParam {
	out := make([]openai.ChatCompletionToolParam, 0, len(tools))

	for _, t := range tools {
		out = append(out, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        t.Schema.Properties["name"].(string),
				Description: openai.String(t.Schema.Properties["description"].(string)),
				Parameters:  openai.FunctionParameters(t.Schema.Properties),
			},
		})
	}

	return out
}
