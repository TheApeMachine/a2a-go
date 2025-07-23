package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go/v2/client"
	"github.com/openai/openai-go/v2/openai"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
roleMap compresses convertMessages' switch.
*/
var roleMap = map[string]func(string) openai.ChatCompletionMessageParamUnion{
	"system":    openai.SystemMessage[string],
	"user":      openai.UserMessage[string],
	"developer": openai.UserMessage[string],
	"agent":     openai.AssistantMessage[string],
	"assistant": openai.AssistantMessage[string],
}

/*
OpenAIProvider is a provider for the OpenAI API.
*/
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

	// OpenAI-specific LLM tool response generator function
	openAIToolResponseGenerator := func(toolCallID string, content string, isError bool) any {
		return openai.ToolMessage(content, toolCallID)
	}

	go func() {
		defer close(ch)

		prvdr.params = &openai.ChatCompletionNewParams{
			Model:             openai.ChatModel(params.Model),
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

		isFinished := false

		for !isFinished {
			if params.Stream {
				fmt.Println(prvdr.String())
				stream := prvdr.client.Chat.Completions.NewStreaming(ctx, *prvdr.params)
				acc := openai.ChatCompletionAccumulator{}

				for stream.Next() {
					chunk := stream.Current()
					acc.AddChunk(chunk)

					if _, ok := acc.JustFinishedContent(); ok {
						ch <- a2a.NewArtifactResult(
							params.Task.ID,
							a2a.NewTextPart(chunk.Choices[0].Delta.Content),
						)
						params.Task.AddFinalPart(a2a.NewTextPart(chunk.Choices[0].Delta.Content))
						break
					}

					if refusal, ok := acc.JustFinishedRefusal(); ok {
						params.Task.ToStatus(
							a2a.TaskStateFailed,
							a2a.NewTextMessage("assistant", fmt.Sprintf("Error: %s", refusal)),
						)
						ch <- jsonrpc.Response{
							Result: a2a.TaskStatusUpdateResult{
								ID:     params.Task.ID,
								Status: params.Task.Status,
								Final:  true,
							},
						}
						break
					}

					if toolCall, ok := acc.JustFinishedToolCall(); ok { // toolCall is openai.FinishedChatCompletionToolCall
						updatedTask, llmToolMsg, toolExecErr := ExecuteAndProcessToolCall(
							ctx,
							toolCall.Name,
							toolCall.Arguments,
							toolCall.Id, // Use .Id for FinishedChatCompletionToolCall
							params.Task,
							openAIToolResponseGenerator,
						)
						params.Task = updatedTask // Persist changes to task
						prvdr.params.Messages = append(prvdr.params.Messages, llmToolMsg.(openai.ChatCompletionMessageParamUnion))

						if toolExecErr != nil {
							ch <- jsonrpc.Response{
								Result: params.Task, // Send updated task with error artifact
								Error: &jsonrpc.Error{
									Code:    errors.ErrInternal.Code,
									Message: fmt.Sprintf("Streaming: Error executing tool %s: %v", toolCall.Name, toolExecErr),
								},
							}
						} else {
							ch <- jsonrpc.Response{Result: params.Task} // Send updated task with success artifact
						}

						break
					}

					if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
						ch <- a2a.NewArtifactResult(
							params.Task.ID,
							a2a.NewTextPart(chunk.Choices[0].Delta.Content),
						)
					}
				}
			} else { // Non-streaming path
				log.Debug("non-streaming", "params", prvdr.params)
				completion, err := prvdr.client.Chat.Completions.New(ctx, *prvdr.params)
				if err != nil {
					ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: errors.ErrInternal.Code, Message: err.Error()}}
					break
				}
				if len(completion.Choices) == 0 {
					ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: errors.ErrInternal.Code, Message: "OpenAI completion returned no choices"}}
					break
				}

				messageFromAssistant := completion.Choices[0].Message
				llmToolCalls := messageFromAssistant.ToolCalls // These are openai.ChatCompletionMessageToolCall

				if len(llmToolCalls) == 0 {
					params.Task.AddFinalPart(a2a.NewTextPart(messageFromAssistant.Content))
					params.Task.ToStatus(a2a.TaskStateCompleted, a2a.NewTextMessage("assistant", messageFromAssistant.Content))
					ch <- jsonrpc.Response{Result: params.Task}
					break
				} else {
					prvdr.params.Messages = append(prvdr.params.Messages, messageFromAssistant.ToParam())
					anyToolFailed := false
					for _, toolCall := range llmToolCalls { // toolCall is openai.ChatCompletionMessageToolCall
						updatedTask, llmToolMsg, toolExecErr := ExecuteAndProcessToolCall(
							ctx,
							toolCall.Function.Name,
							toolCall.Function.Arguments,
							toolCall.ID, // Use .ID for ChatCompletionMessageToolCall
							params.Task,
							openAIToolResponseGenerator,
						)
						params.Task = updatedTask // Persist changes to task
						prvdr.params.Messages = append(prvdr.params.Messages, llmToolMsg.(openai.ChatCompletionMessageParamUnion))

						if toolExecErr != nil {
							// Send update about this specific tool failure
							ch <- jsonrpc.Response{
								Result: params.Task,
								Error: &jsonrpc.Error{
									Code:    errors.ErrInternal.Code,
									Message: fmt.Sprintf("Error executing tool %s: %v", toolCall.Function.Name, toolExecErr),
								},
							}
							// Optional: Decide if one tool error should stop processing other parallel tool calls from LLM
							// For now, we'll let it continue to add all tool results/errors for the LLM to see.
							// However, we mark that a failure occurred to prevent proceeding to next LLM call if critical.
							anyToolFailed = true
						} else {
							ch <- jsonrpc.Response{Result: params.Task} // Send updated task with success artifact
						}
					}
					if anyToolFailed {
						// If any tool failed, we might not want to proceed to the next LLM call immediately.
						// The task status would have been updated by the helper if we decide to fail it there.
						// For now, the loop will continue, and the LLM will receive all tool responses (including errors).
						// If we want to halt on first tool error, we'd `break` here.
						log.Warn("One or more tool calls failed in non-streaming mode. LLM will receive all results including errors.")
					}
					// Continue to the next iteration of the main loop to get LLM's response to tool results.
				}
			}
		}
	}()
	return ch
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

// func (prvdr *OpenAIProvider) TTS(ctx context.Context, text string) error {
// 	res, err := prvdr.client.Audio.Speech.New(ctx, openai.AudioSpeechNewParams{
// 		Model:          openai.SpeechModelTTS1,
// 		Input:          text,
// 		ResponseFormat: openai.AudioSpeechNewParamsResponseFormatPCM,
// 		Voice:          openai.AudioSpeechNewParamsVoiceAlloy,
// 	})

// 	if err != nil {
// 		return err
// 	}

// 	defer res.Body.Close()
// 	return utils.PlayPCM(res.Body)
// }

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

func (prvdr *OpenAIProvider) String() string {
	if prvdr.params == nil {
		return "OpenAIProvider params are not initialized."
	}

	var sb strings.Builder

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	// Indentation and box-drawing chars
	bullet := "│ "

	sb.WriteString(headerStyle.Render("OpenAIProvider Params") + "\n")

	sb.WriteString(bullet + labelStyle.Render("Model: ") + valueStyle.Render(string(prvdr.params.Model)) + "\n")

	sb.WriteString(bullet + labelStyle.Render("Messages:") + "\n")
	for i, msg := range prvdr.params.Messages {
		sb.WriteString(fmt.Sprintf("%s  Message %d:\n", bullet, i+1))
		if msg.OfSystem != nil {
			sb.WriteString(fmt.Sprintf("%s    Role: %s\n", bullet, msg.OfSystem.Role))
			sb.WriteString(fmt.Sprintf("%s    Content: %s\n", bullet, msg.OfSystem.Content))
		}
		if msg.OfUser != nil {
			sb.WriteString(fmt.Sprintf("%s    Role: %s\n", bullet, msg.OfUser.Role))
			if msg.OfUser.Content.OfString.IsPresent() {
				sb.WriteString(fmt.Sprintf("%s    Content: %s\n", bullet, msg.OfUser.Content.OfString))
			} else if len(msg.OfUser.Content.OfArrayOfContentParts) > 0 {
				sb.WriteString(fmt.Sprintf("%s    Content: [multipart (%d parts)]\n", bullet, len(msg.OfUser.Content.OfArrayOfContentParts)))
				// Example for iterating parts if needed:
				// for _, part := range msg.OfUserMessage.Content.OfContentPartArray {
				// 	sb.WriteString(fmt.Sprintf("%s      Part Type: %s\n", bullet, part.Type))
				// 	if part.Text != nil {
				// 		sb.WriteString(fmt.Sprintf("%s        Text: %s\n", bullet, *part.Text))
				// 	}
				// }
			} else {
				sb.WriteString(fmt.Sprintf("%s    Content: [empty user message]\n", bullet))
			}
		}
		if msg.OfAssistant != nil {
			sb.WriteString(fmt.Sprintf("%s    Role: %s\n", bullet, msg.OfAssistant.Role))
			sb.WriteString(fmt.Sprintf("%s    Content: %s\n", bullet, msg.OfAssistant.Content.OfString.Value))
			if len(msg.OfAssistant.ToolCalls) > 0 {
				sb.WriteString(fmt.Sprintf("%s    ToolCalls:\n", bullet))
				for j, tc := range msg.OfAssistant.ToolCalls {
					sb.WriteString(fmt.Sprintf("%s      ToolCall %d:\n", bullet, j+1))
					sb.WriteString(fmt.Sprintf("%s        ID: %s\n", bullet, tc.ID))
					sb.WriteString(fmt.Sprintf("%s        Type: %s\n", bullet, tc.Type))
					sb.WriteString(fmt.Sprintf("%s        Function Name: %s\n", bullet, tc.Function.Name))
					sb.WriteString(fmt.Sprintf("%s        Function Arguments: %s\n", bullet, tc.Function.Arguments))
				}
			}
		}
		if msg.OfTool != nil {
			sb.WriteString(fmt.Sprintf("%s    Role: %s\n", bullet, msg.OfTool.Role))
			sb.WriteString(fmt.Sprintf("%s    Content: %s\n", bullet, msg.OfTool.Content))
			sb.WriteString(fmt.Sprintf("%s    ToolCallID: %s\n", bullet, msg.OfTool.ToolCallID))
		}
	}

	if len(prvdr.params.Tools) > 0 {
		sb.WriteString(bullet + labelStyle.Render("Tools:") + "\n")
		for i, tool := range prvdr.params.Tools {
			sb.WriteString(fmt.Sprintf("%s  Tool %d:\n", bullet, i+1))
			sb.WriteString(fmt.Sprintf("%s    Type: %s\n", bullet, tool.Type))
			sb.WriteString(fmt.Sprintf("%s    Function Name: %s\n", bullet, tool.Function.Name))
			if tool.Function.Description.IsPresent() {
				sb.WriteString(fmt.Sprintf("%s    Function Description: %s\n", bullet, tool.Function.Description))
			}
			if tool.Function.Parameters != nil {
				sb.WriteString(fmt.Sprintf("%s    Function Parameters:\n", bullet))
				// Assuming tool.Function.Parameters is map[string]any or similar
				// The actual type is openai.FunctionParameters which is a type alias for a map.
				for k, v := range tool.Function.Parameters {
					sb.WriteString(fmt.Sprintf("%s      %s: %v\n", bullet, k, v))
				}
			}
		}
	}

	if prvdr.params.ParallelToolCalls.IsPresent() {
		sb.WriteString(bullet + labelStyle.Render("ParallelToolCalls: ") + valueStyle.Render(fmt.Sprintf("%t", prvdr.params.ParallelToolCalls.Value)) + "\n")
	}
	if prvdr.params.FrequencyPenalty.IsPresent() {
		sb.WriteString(bullet + labelStyle.Render("FrequencyPenalty: ") + valueStyle.Render(fmt.Sprintf("%.2f", prvdr.params.FrequencyPenalty.Value)) + "\n")
	}
	if prvdr.params.MaxTokens.IsPresent() {
		sb.WriteString(bullet + labelStyle.Render("MaxTokens: ") + valueStyle.Render(fmt.Sprintf("%d", prvdr.params.MaxTokens.Value)) + "\n")
	}
	if prvdr.params.TopP.IsPresent() {
		sb.WriteString(bullet + labelStyle.Render("TopP: ") + valueStyle.Render(fmt.Sprintf("%.2f", prvdr.params.TopP.Value)) + "\n")
	}
	if prvdr.params.Seed.IsPresent() {
		sb.WriteString(bullet + labelStyle.Render("Seed: ") + valueStyle.Render(fmt.Sprintf("%d", prvdr.params.Seed.Value)) + "\n")
	}

	if prvdr.params.Stop.OfChatCompletionNewsStopArray != nil && len(prvdr.params.Stop.OfChatCompletionNewsStopArray) > 0 {
		sb.WriteString(bullet + labelStyle.Render("Stop: ") + valueStyle.Render(strings.Join(prvdr.params.Stop.OfChatCompletionNewsStopArray, ", ")) + "\n")
	} else if prvdr.params.Stop.OfString.IsPresent() {
		sb.WriteString(bullet + labelStyle.Render("Stop: ") + valueStyle.Render(prvdr.params.Stop.OfString.Value))
	}

	return sb.String()
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
		if tool == nil {
			continue
		}

		out = append(out, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters: openai.FunctionParameters(map[string]any{
					"type":       tool.InputSchema.Type,
					"properties": tool.InputSchema.Properties,
				}),
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
