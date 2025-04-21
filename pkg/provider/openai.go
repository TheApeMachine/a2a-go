package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
OpenAIProvider is a provider for the OpenAI API.
*/
type OpenAIProvider struct {
	api     openai.Client
	Model   string
	Execute ToolExecutor
}

/*
NewOpenAIProvider returns a new OpenAIProvider with sensible defaults.
*/
func NewOpenAIProvider(executor ToolExecutor) *OpenAIProvider {
	v := viper.GetViper()

	return &OpenAIProvider{
		api:     openai.NewClient(),
		Model:   v.GetString("provider.openai.model"),
		Execute: executor,
	}
}

/*
Complete runs a synchronous (non‑streaming) chat completion for the given A2A
message history.  If the assistant returns a tool call it is executed via the
provided ToolExecutor and the conversation auto‑continues until the final
assistant reply no longer contains tool calls.
*/
func (prvdr *OpenAIProvider) Complete(
	ctx context.Context,
	task *types.Task,
	tools *map[string]*types.MCPClient,
) (err error) {
	var (
		resp   *openai.ChatCompletion
		params = openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(prvdr.Model),
			Messages: prvdr.convertMessages(task.History),
			Tools:    prvdr.convertTools(tools),
		}
	)

	prvdr.setSchema(task, &params)

	task.ToState(types.TaskStateWorking, "thinking...")

	for task.Status.State == types.TaskStateWorking {
		if resp, err = prvdr.api.Chat.Completions.New(ctx, params); err != nil {
			task.ToState(types.TaskStateFailed, err.Error())
			return err
		}

		msg := resp.Choices[0].Message

		if len(resp.Choices[0].Message.ToolCalls) == 0 {
			task.Artifacts = append(task.Artifacts, types.Artifact{
				Parts:    []types.Part{{Type: types.PartTypeText, Text: msg.Content}},
				Index:    0,
				Append:   utils.Ptr(true),
				Metadata: map[string]any{"role": "agent", "name": prvdr.Model},
			})
		}

		for _, tc := range msg.ToolCalls {
			if err := prvdr.handleToolCall(
				ctx, &params, task, tools, msg, tc,
			); err != nil {
				return err
			}
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
func (prvdr *OpenAIProvider) Stream(
	ctx context.Context,
	messages []types.Message,
	tools *map[string]*types.MCPClient,
	onDelta func(string),
) (string, error) {
	oaMsgs := prvdr.convertMessages(messages)
	oaTools := prvdr.convertTools(tools)

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(prvdr.Model),
		Messages: oaMsgs,
		Tools:    oaTools,
	}

	var finalContent string

	stream := prvdr.api.Chat.Completions.NewStreaming(ctx, params)

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

func (prvdr *OpenAIProvider) GenerateImage(
	ctx context.Context, prompt string,
) (string, error) {
	image, err := prvdr.api.Images.Generate(ctx, openai.ImageGenerateParams{
		Prompt:         prompt,
		Model:          openai.ImageModelDallE3,
		ResponseFormat: openai.ImageGenerateParamsResponseFormatB64JSON,
		N:              openai.Int(1),
	})

	if err != nil {
		panic(err)
	}

	return image.Data[0].URL, nil
}

func (prvdr *OpenAIProvider) FineTune(
	ctx context.Context,
	fileID string,
) {
	fineTune, err := prvdr.api.FineTuning.Jobs.New(ctx, openai.FineTuningJobNewParams{
		Model:        openai.FineTuningJobNewParamsModelGPT4oMini,
		TrainingFile: fileID,
	})

	if err != nil {
		panic(err)
	}

	events := make(map[string]openai.FineTuningJobEvent)

	for fineTune.Status == "running" || fineTune.Status == "queued" || fineTune.Status == "validating_files" {
		fineTune, err = prvdr.api.FineTuning.Jobs.Get(ctx, fineTune.ID)

		if err != nil {
			panic(err)
		}

		fmt.Println(fineTune.Status)

		page, err := prvdr.api.FineTuning.Jobs.ListEvents(ctx, fineTune.ID, openai.FineTuningJobListEventsParams{
			Limit: openai.Int(100),
		})

		if err != nil {
			panic(err)
		}

		for i := len(page.Data) - 1; i >= 0; i-- {
			event := page.Data[i]

			if _, exists := events[event.ID]; exists {
				continue
			}

			events[event.ID] = event
			timestamp := time.Unix(int64(event.CreatedAt), 0)
			fmt.Printf("- %s: %s\n", timestamp.Format(time.Kitchen), event.Message)
		}

		time.Sleep(5 * time.Second)
	}
}

func (prvdr *OpenAIProvider) handleToolCall(
	ctx context.Context,
	params *openai.ChatCompletionNewParams,
	task *types.Task,
	tools *map[string]*types.MCPClient,
	msg openai.ChatCompletionMessage,
	tc openai.ChatCompletionMessageToolCall,
) error {
	log.Info("tool call", "tool", tc.Function.Name)
	tool, ok := (*tools)[tc.Function.Name]

	if !ok {
		task.ToState(types.TaskStateFailed, fmt.Sprintf("unknown tool called: %s", tc.Function.Name))
		return fmt.Errorf("unknown tool called: %s", tc.Function.Name)
	}

	var args map[string]any

	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		task.ToState(types.TaskStateFailed, fmt.Sprintf("malformed tool args: %s", err))
		return fmt.Errorf("malformed tool args: %w", err)
	}

	tool.Toolcall = &mcp.CallToolRequest{
		Params: struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name:      tc.Function.Name,
			Arguments: args,
		},
	}

	result, err := prvdr.Execute(ctx, tool, args)

	task.History = append(task.History, types.Message{
		Role:     "agent",
		Parts:    []types.Part{{Type: types.PartTypeText, Text: result}},
		Metadata: map[string]any{"name": tool.Name},
	})

	if err != nil {
		task.ToState(types.TaskStateFailed, err.Error())
		return err
	}

	oaToolMsg := openai.ToolMessage(result, tc.ID)
	params.Messages = append(params.Messages, msg.ToParam(), oaToolMsg)

	return nil
}

func (prvdr *OpenAIProvider) convertMessages(
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

		switch m.Role {
		case "system":
			out = append(out, openai.SystemMessage(text))
		case "user", "developer":
			out = append(out, openai.UserMessage(text))
		case "agent", "assistant":
			out = append(out, openai.AssistantMessage(text))
		}
	}

	return out
}

func (prvdr *OpenAIProvider) convertTools(
	tools *map[string]*types.MCPClient,
) []openai.ChatCompletionToolParam {
	out := make([]openai.ChatCompletionToolParam, 0, len(*tools))

	for _, t := range *tools {
		// Create a proper OpenAI function parameters schema
		schema := map[string]any{
			"type":       t.Schema.Type,
			"properties": t.Schema.Properties,
			"required":   t.Schema.Required,
		}

		out = append(out, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  openai.FunctionParameters(schema),
			},
		})
	}

	return out
}

/*
Schema adheres to the structured outputs specification of the A2A Protocol,
and converts it to be compatible with OpenAI's structured outputs.
*/
func (prvdr *OpenAIProvider) setSchema(
	task *types.Task, params *openai.ChatCompletionNewParams,
) {
	lastMessage := task.History[len(task.History)-1]

	if schema, ok := lastMessage.Metadata["schema"].(map[string]any); ok {
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        schema["name"].(string),
					Description: openai.String(schema["description"].(string)),
					Schema:      schema["schema"].(map[string]any),
					Strict:      openai.Bool(true),
				},
			},
		}
	}
}

type OpenAIEmbedder struct {
	api   openai.Client
	Model string
}

func NewOpenAIEmbedder() *OpenAIEmbedder {
	v := viper.GetViper()

	return &OpenAIEmbedder{
		api:   openai.NewClient(),
		Model: v.GetString("provider.openai.embed"),
	}
}

func (prvdr *OpenAIEmbedder) Embed(
	ctx context.Context, text string,
) ([]float32, error) {
	resp, err := prvdr.api.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(prvdr.Model),
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: []string{text}},
	})
	if err != nil {
		return nil, err
	}

	src := resp.Data[0].Embedding
	dst := make([]float32, len(src))
	for i, v := range src {
		dst[i] = float32(v)
	}
	return dst, nil
}

func (prvdr *OpenAIEmbedder) EmbedBatch(
	ctx context.Context, texts []string,
) ([][]float32, error) {
	resp, err := prvdr.api.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(prvdr.Model),
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
	})
	if err != nil {
		return nil, err
	}

	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		src := d.Embedding
		dst := make([]float32, len(src))
		for j, v := range src {
			dst[j] = float32(v)
		}
		out[i] = dst
	}
	return out, nil
}
