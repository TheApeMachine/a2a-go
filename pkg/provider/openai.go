package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ebitengine/oto/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared/constant"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

// newOpenAIClient centralises construction so proxy/retry settings stay in sync.
func newOpenAIClient() openai.Client { return openai.NewClient() }

// f64To32 converts an OpenAI float64 embedding to float32 – avoids dup loops.
func f64To32(src []float64) []float32 {
	dst := make([]float32, len(src))
	for i, v := range src {
		dst[i] = float32(v)
	}
	return dst
}

// playPCM abstracts the Oto boilerplate in TTS.
func playPCM(r io.Reader) error {
	op := &oto.NewContextOptions{SampleRate: 24000, ChannelCount: 1, Format: oto.FormatSignedInt16LE}
	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		return fmt.Errorf("oto context: %w", err)
	}
	<-ready

	p := ctx.NewPlayer(r)
	p.Play()
	for p.IsPlaying() {
		time.Sleep(time.Millisecond)
	}
	return p.Close()
}

// roleMap compresses convertMessages' switch.
var roleMap = map[string]func(string) openai.ChatCompletionMessageParamUnion{
	"system":    openai.SystemMessage[string],
	"user":      openai.UserMessage[string],
	"developer": openai.UserMessage[string],
	"agent":     openai.AssistantMessage[string],
	"assistant": openai.AssistantMessage[string],
}

type OpenAIProvider struct {
	api     openai.Client
	Model   string
	Execute ToolExecutor
}

func NewOpenAIProvider(executor ToolExecutor) *OpenAIProvider {
	return &OpenAIProvider{
		api:     newOpenAIClient(),
		Model:   viper.GetString("provider.openai.model"),
		Execute: executor,
	}
}

// Complete executes a non‑streaming chat interaction, recursively handling
// tool‑calls until a final assistant reply is produced.
func (p *OpenAIProvider) Complete(ctx context.Context, task *types.Task, tools *map[string]*types.MCPClient) error {
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(p.Model),
		Messages: p.convertMessages(task.History),
		Tools:    p.convertTools(tools),
	}
	p.applySchema(task, &params)
	task.ToState(types.TaskStateWorking, "thinking...")

	for task.Status.State == types.TaskStateWorking {
		resp, err := p.api.Chat.Completions.New(ctx, params)
		if err != nil {
			task.ToState(types.TaskStateFailed, err.Error())
			return err
		}

		msg := resp.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			task.Artifacts = append(task.Artifacts, types.Artifact{
				Parts:    []types.Part{{Type: types.PartTypeText, Text: msg.Content}},
				Index:    0,
				Append:   utils.Ptr(true),
				Metadata: map[string]any{"role": "agent", "name": p.Model},
			})
			task.ToState(types.TaskStateCompleted, "completed")
			break
		}

		for _, tc := range msg.ToolCalls {
			if err := p.handleToolCall(ctx, &params, task, tools, msg, tc); err != nil {
				return err
			}
		}
	}
	return nil
}

// Stream runs a streaming completion. Tool‑calls are resolved once the first
// assistant message finishes streaming.
func (p *OpenAIProvider) Stream(
	ctx context.Context,
	task *types.Task,
	tools *map[string]*types.MCPClient,
	onDelta func(*types.Task),
) error {
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(p.Model),
		Messages: p.convertMessages(task.History),
		Tools:    p.convertTools(tools),
	}

	p.applySchema(task, &params)
	task.ToState(types.TaskStateWorking, "thinking...")

	stream := p.api.Chat.Completions.NewStreaming(ctx, params)
	acc := openai.ChatCompletionAccumulator{}

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if tool, ok := acc.JustFinishedToolCall(); ok {
			p.handleStreamToolCall(ctx, &chunk, task, tools, &params, tool)
		}

		if content, ok := acc.JustFinishedContent(); ok {
			task.History[len(task.History)-1].Parts = append(
				task.History[len(task.History)-1].Parts,
				types.Part{Type: types.PartTypeText, Text: content},
			)
			onDelta(task)
			task.ToState(types.TaskStateCompleted, "completed")
		}

		if refusal, ok := acc.JustFinishedRefusal(); ok {
			task.ToState(types.TaskStateFailed, refusal)
			return errors.New(refusal)
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			task.History[len(task.History)-1].Parts = append(
				task.History[len(task.History)-1].Parts,
				types.Part{Type: types.PartTypeText, Text: chunk.Choices[0].Delta.Content},
			)
			onDelta(task)
		}
	}
	return stream.Err()
}

// GenerateImage delegates to DALL‑E 3 and returns the URL.
func (p *OpenAIProvider) GenerateImage(ctx context.Context, prompt string) (string, error) {
	img, err := p.api.Images.Generate(ctx, openai.ImageGenerateParams{
		Prompt:         prompt,
		Model:          openai.ImageModelDallE3,
		ResponseFormat: openai.ImageGenerateParamsResponseFormatURL,
		N:              openai.Int(1),
	})

	if err != nil {
		return "", err
	}

	return img.Data[0].URL, nil
}

func (p *OpenAIProvider) AudioTranscript(ctx context.Context, audio []byte) (string, error) {
	tr, err := p.api.Audio.Transcriptions.New(ctx, openai.AudioTranscriptionNewParams{
		Model: openai.AudioModelWhisper1,
		File:  bytes.NewReader(audio),
	})
	if err != nil {
		return "", err
	}
	return tr.Text, nil
}

func (p *OpenAIProvider) TTS(ctx context.Context, text string) error {
	res, err := p.api.Audio.Speech.New(ctx, openai.AudioSpeechNewParams{
		Model:          openai.SpeechModelTTS1,
		Input:          text,
		ResponseFormat: openai.AudioSpeechNewParamsResponseFormatPCM,
		Voice:          openai.AudioSpeechNewParamsVoiceAlloy,
	})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return playPCM(res.Body)
}

func (p *OpenAIProvider) FineTune(ctx context.Context, fileID string) error {
	job, err := p.api.FineTuning.Jobs.New(ctx, openai.FineTuningJobNewParams{
		Model:        openai.FineTuningJobNewParamsModelGPT4oMini,
		TrainingFile: fileID,
	})
	if err != nil {
		return err
	}

	eventsSeen := make(map[string]struct{})
	for job.Status == "running" || job.Status == "queued" || job.Status == "validating_files" {
		job, err = p.api.FineTuning.Jobs.Get(ctx, job.ID)
		if err != nil {
			return err
		}
		log.Info("fine‑tune status", "status", job.Status)

		page, err := p.api.FineTuning.Jobs.ListEvents(ctx, job.ID, openai.FineTuningJobListEventsParams{Limit: openai.Int(100)})
		if err != nil {
			return err
		}
		for i := len(page.Data) - 1; i >= 0; i-- {
			e := page.Data[i]
			if _, ok := eventsSeen[e.ID]; ok {
				continue
			}
			eventsSeen[e.ID] = struct{}{}
			ts := time.Unix(int64(e.CreatedAt), 0)
			fmt.Printf("- %s: %s\n", ts.Format(time.Kitchen), e.Message)
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (p *OpenAIProvider) handleStreamToolCall(
	ctx context.Context,
	chunk *openai.ChatCompletionChunk,
	task *types.Task,
	tools *map[string]*types.MCPClient,
	params *openai.ChatCompletionNewParams,
	f openai.FinishedChatCompletionToolCall,
) {
	log.Info("tool call", "tool", f.Name)

	if chunk == nil || len(chunk.Choices) == 0 {
		return
	}

	delta := chunk.Choices[0].Delta
	if delta.ToolCalls == nil || len(delta.ToolCalls) <= f.Index {
		return
	}

	tc := delta.ToolCalls[f.Index]
	err := p.handleToolCall(ctx, params, task, tools,
		openai.ChatCompletionMessage{Role: "assistant", Content: f.Arguments},
		openai.ChatCompletionMessageToolCall{
			ID:       f.Id,
			Function: openai.ChatCompletionMessageToolCallFunction(tc.Function),
			Type:     constant.Function(tc.Type),
		})

	if err != nil {
		log.Error("failed to handle tool call", "error", err)
	}
}

func (p *OpenAIProvider) handleToolCall(
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
		err := fmt.Errorf("unknown tool called: %s", tc.Function.Name)
		task.ToState(types.TaskStateFailed, err.Error())
		return err
	}

	var args map[string]any

	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		task.ToState(types.TaskStateFailed, "malformed tool args")
		return err
	}

	tool.Toolcall = &mcp.CallToolRequest{Params: struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments,omitempty"`
		Meta      *struct {
			ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
		} `json:"_meta,omitempty"`
	}{Name: tc.Function.Name, Arguments: args}}

	result, err := p.Execute(ctx, tool, args)

	if err != nil {
		task.ToState(types.TaskStateFailed, err.Error())
		return err
	}

	task.History = append(task.History, types.Message{
		Role:  "agent",
		Parts: []types.Part{{Type: types.PartTypeText, Text: result}},
		Metadata: map[string]any{
			"name": tool.Name,
		},
	})

	params.Messages = append(params.Messages, msg.ToParam(), openai.ToolMessage(result, tc.ID))
	return nil
}

func (p *OpenAIProvider) convertMessages(mm []types.Message) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(mm))
	for _, m := range mm {
		var text string
		for _, p := range m.Parts {
			if p.Type == types.PartTypeText {
				text = p.Text
				break
			}
		}
		if fn, ok := roleMap[m.Role]; ok {
			out = append(out, fn(text))
		}
	}
	return out
}

func (p *OpenAIProvider) convertTools(tools *map[string]*types.MCPClient) []openai.ChatCompletionToolParam {
	out := make([]openai.ChatCompletionToolParam, 0, len(*tools))
	for _, t := range *tools {
		schema := map[string]any{"type": t.Schema.Type, "properties": t.Schema.Properties, "required": t.Schema.Required}
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

func (p *OpenAIProvider) applySchema(task *types.Task, params *openai.ChatCompletionNewParams) {
	if len(task.History) == 0 {
		return
	}
	if schema, ok := task.History[len(task.History)-1].Metadata["schema"].(map[string]any); ok {
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
	return &OpenAIEmbedder{api: newOpenAIClient(), Model: viper.GetString("provider.openai.embed")}
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.api.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(e.Model),
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: []string{text}},
	})
	if err != nil {
		return nil, err
	}
	return f64To32(resp.Data[0].Embedding), nil
}

func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := e.api.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(e.Model),
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
	})
	if err != nil {
		return nil, err
	}

	out := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		out[i] = f64To32(d.Embedding)
	}
	return out, nil
}
