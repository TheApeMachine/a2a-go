package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v3/client"
	"github.com/mark3labs/mcp-go/mcp"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/tools"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

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
	client *openai.Client
	params *openai.ChatCompletionNewParams
}

type OpenAIProviderOption func(*OpenAIProvider)

func NewOpenAIProvider(options ...OpenAIProviderOption) *OpenAIProvider {
	prvdr := &OpenAIProvider{}

	for _, option := range options {
		option(prvdr)
	}

	return prvdr
}

func (prvdr *OpenAIProvider) Generate(
	ctx context.Context, params *ProviderParams,
) chan jsonrpc.Response {
	ch := make(chan jsonrpc.Response)

	go func() {
		defer close(ch)

		prvdr.params = &openai.ChatCompletionNewParams{
			Model:             openai.ChatModel(prvdr.params.Model),
			Messages:          prvdr.convertMessages(params.Task),
			Tools:             prvdr.convertTools(params.Tools),
			ParallelToolCalls: openai.Bool(params.ParallelToolCalls),
			Temperature:       openai.Float(params.Temperature),
			FrequencyPenalty:  openai.Float(params.FrequencyPenalty),
			PresencePenalty:   openai.Float(params.PresencePenalty),
			MaxTokens:         openai.Int(params.MaxTokens),
			TopP:              openai.Float(params.TopP),
			Seed:              openai.Int(params.Seed),
			Stop: openai.ChatCompletionNewParamsStopUnion{
				OfChatCompletionNewsStopArray: params.Stop,
			},
		}

		schema := params.Task.History[len(params.Task.History)-1].Metadata["schema"]

		if schema != nil {
			prvdr.params.ResponseFormat = prvdr.applySchema(params.Task)
		}

		isDone := false

		for !isDone {
			if params.Stream {
				stream := prvdr.client.Chat.Completions.NewStreaming(ctx, *prvdr.params)
				acc := openai.ChatCompletionAccumulator{}

				for stream.Next() {
					chunk := stream.Current()

					acc.AddChunk(chunk)

					// When this fires, the current chunk value will not contain content data
					if _, ok := acc.JustFinishedContent(); ok {
						ch <- a2a.NewArtifactResult(
							params.Task.ID,
							a2a.NewTextPart(chunk.Choices[0].Delta.Content),
						)

						params.Task.AddFinalPart(a2a.NewTextPart(chunk.Choices[0].Delta.Content))
						isDone = true
					}

					if refusal, ok := acc.JustFinishedRefusal(); ok {
						params.Task.ToStatus(
							a2a.TaskStateFailed,
							a2a.NewTextMessage(
								"assistant",
								fmt.Sprintf("Error: %s", refusal),
							),
						)

						ch <- jsonrpc.Response{
							Result: a2a.TaskStatusUpdateResult{
								ID:       params.Task.ID,
								Status:   a2a.TaskStatus{State: a2a.TaskStateFailed},
								Final:    true,
								Metadata: map[string]any{},
							},
						}
					}

					if tool, ok := acc.JustFinishedToolCall(); ok {
						prvdr.params.Messages = append(
							prvdr.params.Messages,
							acc.ChatCompletion.Choices[0].Message.ToParam(),
						)

						tools.NewOpenAIExecutor(ctx, tool.Name, tool.Arguments)
					}

					if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
						ch <- a2a.NewArtifactResult(
							params.Task.ID,
							a2a.NewTextPart(chunk.Choices[0].Delta.Content),
						)
					}
				}
			} else {
				completion, err := prvdr.client.Chat.Completions.New(ctx, *prvdr.params)

				if err != nil {
					log.Error("failed to generate completion", "error", err)

					ch <- jsonrpc.Response{
						Error: &jsonrpc.Error{
							Code:    int(a2a.ErrorCodeInternalError),
							Message: err.Error(),
						},
					}
				}

				toolCalls := completion.Choices[0].Message.ToolCalls

				if len(toolCalls) == 0 {
					ch <- a2a.NewArtifactResult(
						params.Task.ID,
						a2a.NewTextPart(completion.Choices[0].Message.Content),
					)

					params.Task.AddFinalPart(a2a.NewTextPart(completion.Choices[0].Message.Content))
					isDone = true
				}

				prvdr.params.Messages = append(
					prvdr.params.Messages,
					completion.Choices[0].Message.ToParam(),
				)

				for _, toolCall := range toolCalls {
					err := prvdr.handleToolCall(ctx, toolCall, ch, params.Task)

					if err != nil {
						log.Error("error executing tool", "error", err)
						continue
					}
				}
			}
		}
	}()

	return ch
}

func (prvdr *OpenAIProvider) handleToolCall(
	ctx context.Context,
	toolCall openai.ChatCompletionMessageToolCall,
	out chan jsonrpc.Response,
	task *a2a.Task,
) error {
	results, err := tools.NewOpenAIExecutor(
		ctx, toolCall.Function.Name, toolCall.Function.Arguments,
	)

	if err != nil {
		log.Error("error executing tool", "error", err)
		return err
	}

	prvdr.params.Messages = append(
		prvdr.params.Messages,
		openai.ToolMessage(results, toolCall.ID),
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

/*
GenerateImage delegates to DALL‑E 3 and returns the URL.
*/
func (prvdr *OpenAIProvider) GenerateImage(
	ctx context.Context, task *a2a.Task,
) *a2a.Task {
	prompt := task.LastMessage().String()

	img, err := prvdr.client.Images.Generate(ctx, openai.ImageGenerateParams{
		Prompt:         prompt,
		Model:          openai.ImageModelDallE3,
		ResponseFormat: openai.ImageGenerateParamsResponseFormatURL,
		N:              openai.Int(1),
	})

	if err != nil {
		task.ToStatus(
			a2a.TaskStateFailed,
			a2a.NewTextMessage(
				"assistant",
				fmt.Sprintf("Error generating image: %s", err),
			),
		)
	}

	cc := client.New()
	res, err := cc.Get(img.Data[0].URL)

	if err != nil || res.StatusCode() < 200 || res.StatusCode() >= 300 {
		task.ToStatus(
			a2a.TaskStateFailed,
			a2a.NewTextMessage(
				"assistant",
				fmt.Sprintf("Error downloading image: %s", err),
			),
		)
	}

	task.AddArtifact(a2a.NewFileArtifact(
		"image",
		"image/png",
		base64.StdEncoding.EncodeToString(res.Body()),
	))

	return task
}

func (prvdr *OpenAIProvider) AudioTranscript(ctx context.Context, audio []byte) (string, error) {
	tr, err := prvdr.client.Audio.Transcriptions.New(ctx, openai.AudioTranscriptionNewParams{
		Model: openai.AudioModelWhisper1,
		File:  bytes.NewReader(audio),
	})

	if err != nil {
		return "", err
	}

	return tr.Text, nil
}

func (prvdr *OpenAIProvider) TTS(ctx context.Context, text string) error {
	res, err := prvdr.client.Audio.Speech.New(ctx, openai.AudioSpeechNewParams{
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

func (prvdr *OpenAIProvider) FineTune(ctx context.Context, fileID string) error {
	job, err := prvdr.client.FineTuning.Jobs.New(ctx, openai.FineTuningJobNewParams{
		Model:        openai.FineTuningJobNewParamsModelGPT4oMini,
		TrainingFile: fileID,
	})

	if err != nil {
		log.Error("failed to create fine‑tune job", "error", err)
		return err
	}

	eventsSeen := make(map[string]struct{})

	for job.Status == "running" || job.Status == "queued" || job.Status == "validating_files" {
		job, err = prvdr.client.FineTuning.Jobs.Get(ctx, job.ID)
		if err != nil {
			log.Error("failed to get fine‑tune job", "error", err)
			return err
		}
		log.Info("fine‑tune status", "status", job.Status)

		page, err := prvdr.client.FineTuning.Jobs.ListEvents(
			ctx, job.ID, openai.FineTuningJobListEventsParams{Limit: openai.Int(100)},
		)

		if err != nil {
			log.Error("failed to list fine‑tune events", "error", err)
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

func (prvdr *OpenAIProvider) convertMessages(
	task *a2a.Task,
) []openai.ChatCompletionMessageParamUnion {
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(task.History))

	for _, msg := range task.History {
		var text string

		for _, p := range msg.Parts {
			if p.Type == a2a.PartTypeText {
				text = p.Text
				break
			}
		}

		if fn, ok := roleMap[msg.Role]; ok {
			out = append(out, fn(text))
		}
	}
	return out
}

func (prvdr *OpenAIProvider) convertTools(
	tools []*mcp.Tool,
) []openai.ChatCompletionToolParam {
	out := make([]openai.ChatCompletionToolParam, 0, len(tools))

	for _, tool := range tools {
		out = append(out, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  openai.FunctionParameters(tool.InputSchema.Properties),
			},
		})
	}

	return out
}

func (p *OpenAIProvider) applySchema(
	task *a2a.Task,
) openai.ChatCompletionNewParamsResponseFormatUnion {
	return openai.ChatCompletionNewParamsResponseFormatUnion{
		OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
			JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
				Name:        "schema",
				Description: openai.String("The schema to use for your response"),
				Schema:      task.Metadata["schema"].(map[string]any),
				Strict:      openai.Bool(true),
			},
		},
	}
}

type OpenAIEmbedder struct {
	api   openai.Client
	Model string
}

type OpenAIEmbedderOption func(*OpenAIEmbedder)

func NewOpenAIEmbedder(options ...OpenAIEmbedderOption) *OpenAIEmbedder {
	embedder := &OpenAIEmbedder{}

	for _, option := range options {
		option(embedder)
	}

	return embedder
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := e.api.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(e.Model),
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: []string{text}},
	})
	if err != nil {
		return nil, err
	}
	return utils.ConvertToFloat32(resp.Data[0].Embedding), nil
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
		out[i] = utils.ConvertToFloat32(d.Embedding)
	}
	return out, nil
}

func WithOpenAIClient() OpenAIProviderOption {
	return func(prvdr *OpenAIProvider) {
		client := openai.NewClient(
			option.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
		)

		prvdr.client = &client
	}
}

func WithOpenAIEmbedderModel(model string) OpenAIEmbedderOption {
	return func(e *OpenAIEmbedder) {
		e.Model = model
	}
}

func WithOpenAIEmbedderClient(client *openai.Client) OpenAIEmbedderOption {
	return func(e *OpenAIEmbedder) {
		e.api = *client
	}
}
