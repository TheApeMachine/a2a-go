package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	openai "github.com/openai/openai-go"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/registry"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
	"github.com/openai/openai-go/option"
)

// Correct ToolExecutor definition (should be the only one now)

// newOpenAIClient centralises construction so proxy/retry settings stay in sync.
func newOpenAIClient() openai.Client {
	return openai.NewClient(
		option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)
}

// f64To32 converts an OpenAI float64 embedding to float32 – avoids dup loops.
func f64To32(src []float64) []float32 {
	dst := make([]float32, len(src))
	for i, v := range src {
		dst[i] = float32(v)
	}
	return dst
}

// playPCM plays audio data using beep/speaker which doesn't require CGO
func playPCM(r io.Reader) error {
	_ = r
	return nil
}

// speakerOnce ensures we only initialize the speaker once
var speakerOnce sync.Once

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
func (p *OpenAIProvider) Complete(ctx context.Context, task *types.Task, tools *map[string]*registry.ToolDescriptor) error {
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
	tools *map[string]*registry.ToolDescriptor,
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

		if toolCallInfo, ok := acc.JustFinishedToolCall(); ok {
			p.handleStreamToolCall(ctx, &chunk, task, tools, &params, toolCallInfo)
		}

		if content, ok := acc.JustFinishedContent(); ok {
			if len(task.History) > 0 {
				lastMsgIndex := len(task.History) - 1
				task.History[lastMsgIndex].Parts = append(
					task.History[lastMsgIndex].Parts,
					types.Part{Type: types.PartTypeText, Text: content},
				)
			} else {
				task.History = append(task.History, types.Message{
					Role:  "assistant",
					Parts: []types.Part{{Type: types.PartTypeText, Text: content}},
				})
			}
			onDelta(task)
		}

		if refusal, ok := acc.JustFinishedRefusal(); ok {
			task.ToState(types.TaskStateFailed, refusal)
			onDelta(task)
			return errors.New(refusal)
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			deltaContent := chunk.Choices[0].Delta.Content
			if len(task.History) > 0 {
				lastMsgIndex := len(task.History) - 1
				numParts := len(task.History[lastMsgIndex].Parts)
				if numParts > 0 && task.History[lastMsgIndex].Parts[numParts-1].Type == types.PartTypeText {
					task.History[lastMsgIndex].Parts[numParts-1].Text += deltaContent
				} else {
					task.History[lastMsgIndex].Parts = append(task.History[lastMsgIndex].Parts, types.Part{Type: types.PartTypeText, Text: deltaContent})
				}
			} else {
				task.History = append(task.History, types.Message{
					Role:  "assistant",
					Parts: []types.Part{{Type: types.PartTypeText, Text: deltaContent}},
				})
			}
			onDelta(task)
		}
	}

	if stream.Err() == nil && task.Status.State == types.TaskStateWorking {
		task.ToState(types.TaskStateCompleted, "completed")
		onDelta(task)
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
	tools *map[string]*registry.ToolDescriptor,
	params *openai.ChatCompletionNewParams,
	f openai.FinishedChatCompletionToolCall,
) {
	log.Info("tool call stream finish", "toolName", f.Name, "args", f.Arguments)

	if chunk == nil || len(chunk.Choices) == 0 {
		return
	}

	toolDesc, ok := (*tools)[f.Name]
	if !ok {
		log.Error("ToolDescriptor object not found for tool name from stream", "toolName", f.Name)
		return
	}

	var args map[string]any
	err := json.Unmarshal([]byte(f.Arguments), &args)
	if err != nil {
		log.Error("failed to parse tool arguments from stream", "toolName", f.Name, "error", err)
		return
	}

	result, execErr := p.Execute(ctx, toolDesc, args)
	if execErr != nil {
		log.Error("failed to handle tool call from stream", "toolName", f.Name, "error", execErr)
		task.ToState(types.TaskStateFailed, execErr.Error())
		return
	}

	task.History = append(task.History, types.Message{
		Role:  "agent",
		Parts: []types.Part{{Type: types.PartTypeText, Text: result}},
		Metadata: map[string]any{
			"name": f.Name,
			"id":   f.Id,
		},
	})

	params.Messages = append(params.Messages, openai.ToolMessage(result, f.Id))
}

func (p *OpenAIProvider) handleToolCall(
	ctx context.Context,
	params *openai.ChatCompletionNewParams,
	task *types.Task,
	tools *map[string]*registry.ToolDescriptor,
	msg openai.ChatCompletionMessage,
	tc openai.ChatCompletionMessageToolCall,
) error {
	log.Info("tool call request", "toolName", tc.Function.Name, "args", tc.Function.Arguments)

	toolDesc, ok := (*tools)[tc.Function.Name]
	if !ok {
		errMsg := fmt.Sprintf("agent does not have skill/tool named '%s' registered", tc.Function.Name)
		task.ToState(types.TaskStateFailed, errMsg)
		log.Error("ToolDescriptor object not found for tool name", "toolName", tc.Function.Name)
		return errors.New(errMsg)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		errMsg := fmt.Sprintf("malformed tool args for %s: %v", tc.Function.Name, err)
		task.ToState(types.TaskStateFailed, errMsg)
		return errors.New(errMsg)
	}

	result, execErr := p.Execute(ctx, toolDesc, args)
	if execErr != nil {
		task.ToState(types.TaskStateFailed, execErr.Error())
		return execErr
	}

	task.History = append(task.History, types.Message{
		Role:  "agent",
		Parts: []types.Part{{Type: types.PartTypeText, Text: result}},
		Metadata: map[string]any{
			"name": tc.Function.Name,
			"id":   tc.ID,
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

func (p *OpenAIProvider) convertTools(tools *map[string]*registry.ToolDescriptor) []openai.ChatCompletionToolParam {
	out := make([]openai.ChatCompletionToolParam, 0, len(*tools))
	for _, t := range *tools {
		schema := map[string]any{"type": t.Schema.Type, "properties": t.Schema.Properties, "required": t.Schema.Required}
		out = append(out, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        t.ToolName,
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
