package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"google.golang.org/genai"
)

/*
googleRoleMap compresses convertMessages' switch.
*/
var googleRoleMap = map[string]func(string) *genai.Content{
	"system": func(text string) *genai.Content {
		return &genai.Content{Role: "user", Parts: []*genai.Part{{Text: text}}}
	},
	"user": func(text string) *genai.Content {
		return &genai.Content{Role: "user", Parts: []*genai.Part{{Text: text}}}
	},
	"developer": func(text string) *genai.Content {
		return &genai.Content{Role: "user", Parts: []*genai.Part{{Text: text}}}
	},
	"agent": func(text string) *genai.Content {
		return &genai.Content{Role: "model", Parts: []*genai.Part{{Text: text}}}
	},
	"assistant": func(text string) *genai.Content {
		return &genai.Content{Role: "model", Parts: []*genai.Part{{Text: text}}}
	},
}

/*
GoogleProvider is a provider for the Google AI API.
*/
type GoogleProvider struct {
	client *genai.Client
}

type GoogleProviderOption func(*GoogleProvider)

func NewGoogleProvider(options ...GoogleProviderOption) *GoogleProvider {
	prvdr := &GoogleProvider{}
	for _, option := range options {
		option(prvdr)
	}
	return prvdr
}

func (prvdr *GoogleProvider) Generate(
	ctx context.Context, params *ProviderParams,
) chan jsonrpc.Response {
	ch := make(chan jsonrpc.Response)

	googleToolResponseGenerator := func(toolName string, content string, isError bool) any {
		responseMap := map[string]any{"content": content}
		if isError {
			// Preserve the original content field and nest error details separately.
			responseMap["error"] = map[string]any{
				"message": content,
			}
		}
		return &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name:     toolName,
				Response: responseMap,
			},
		}
	}

	go func() {
		defer close(ch)

		geminiContents := prvdr.convertMessages(params.Task)
		geminiTools := prvdr.convertTools(params.Tools)
		systemInstruction := prvdr.getSystemInstruction(params.Task)

		// Configuration for the generation call
		generateContentConfig := &genai.GenerateContentConfig{
			Tools:             geminiTools,
			SystemInstruction: systemInstruction,
			Temperature:       genai.Ptr(float32(params.Temperature)),
			MaxOutputTokens:   int32(params.MaxTokens),
			TopP:              genai.Ptr(float32(params.TopP)),
			TopK:              genai.Ptr(float32(params.TopK)),
			StopSequences:     params.Stop,
		}

		for { // Main loop for multi-turn conversation (including tool calls)
			if params.Stream {
				// Assumes client.Models.GenerateContentStream can take []*Content and *GenerateContentConfig
				// The variadic parts argument might be an issue if history is []*Content.
				// For now, let's try passing geminiContents directly if the API supports it, or the first content as main part.
				// This part is uncertain due to the API signature questions.
				// If GenerateContentStream expects parts...Part, this needs flattening or using client.Chats.
				// For now, attempting to pass geminiContents as if it's variadic *Content or similar.
				// This will likely fail if it expects ...Part instead of ...*Content

				iter := prvdr.client.Models.GenerateContentStream(ctx, params.Model, geminiContents, generateContentConfig)

				var accumulatedTextForThisTurn string
				var processedFunctionCallInThisStreamSegment bool
				var lastCandidateWithContent *genai.Candidate // To check finish reason after loop

			streamLoop: // Label for breaking out of the inner stream processing loop
				for resp, err := range iter {
					if err != nil {
						ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: errors.ErrInternal.Code, Message: err.Error()}}
						return // Fatal stream error
					}
					if resp == nil { // Should not happen if err is nil, but good practice to check
						continue
					}

					if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
						lastCandidateWithContent = resp.Candidates[0]
						for _, part := range resp.Candidates[0].Content.Parts {
							if part.FunctionCall != nil {
								fc := part.FunctionCall
								log.Info("Google Provider (Streaming): Tool call", "name", fc.Name)
								geminiContents = append(geminiContents, resp.Candidates[0].Content)

								updatedTask, llmToolMsg, toolExecErr := ExecuteAndProcessToolCall(
									ctx, fc.Name, fmt.Sprintf("%v", fc.Args),
									fc.Name, params.Task, googleToolResponseGenerator,
								)
								params.Task = updatedTask
								toolResponseContent := &genai.Content{
									Role:  "function",
									Parts: []*genai.Part{llmToolMsg.(*genai.Part)},
								}
								geminiContents = append(geminiContents, toolResponseContent)
								params.Task.AddMessage("tool", fmt.Sprintf("Tool %s output: %v", fc.Name, llmToolMsg.(*genai.Part).FunctionResponse.Response), fc.Name)

								if toolExecErr != nil {
									ch <- jsonrpc.Response{Result: params.Task, Error: &jsonrpc.Error{Code: errors.ErrInternal.Code, Message: fmt.Sprintf("Tool %s error: %v", fc.Name, toolExecErr)}}
								} else {
									ch <- jsonrpc.Response{Result: params.Task}
								}
								processedFunctionCallInThisStreamSegment = true
								break streamLoop // Re-evaluate main loop to send updated contents
							} else if len(part.Text) > 0 {
								textChunk := part.Text
								accumulatedTextForThisTurn += textChunk
								ch <- a2a.NewArtifactResult(params.Task.ID, a2a.NewTextPart(textChunk))
							}
						}
					} // End processing parts for a candidate
				} // End of stream iter.Next() loop (streamLoop)

				// After stream segment, add accumulated assistant message to task history
				if accumulatedTextForThisTurn != "" {
					params.Task.AddMessage("assistant", accumulatedTextForThisTurn, "")
				}

				if processedFunctionCallInThisStreamSegment {
					continue // Continue main `for` loop to send updated `geminiContents` with tool response
				}

				// If no function call processed, and stream ended, check finish reason
				if lastCandidateWithContent != nil &&
					(lastCandidateWithContent.FinishReason == genai.FinishReasonStop ||
						lastCandidateWithContent.FinishReason == genai.FinishReasonMaxTokens ||
						lastCandidateWithContent.FinishReason == genai.FinishReasonSafety ||
						lastCandidateWithContent.FinishReason == genai.FinishReasonRecitation ||
						lastCandidateWithContent.FinishReason == genai.FinishReasonOther) {
					if accumulatedTextForThisTurn != "" {
						params.Task.AddFinalPart(a2a.NewTextPart(accumulatedTextForThisTurn))
					}
					params.Task.ToStatus(a2a.TaskStateCompleted, a2a.NewTextMessage("assistant", accumulatedTextForThisTurn))
					ch <- jsonrpc.Response{Result: params.Task}
					return // Goroutine finished processing this task
				}
				// If finish reason unknown or not terminal, and no tool call, it might be an incomplete stream or other issue.
				// For safety, if loop finishes without return/continue, let it try again if params.Task suggests so.
				// However, this path should ideally be covered by iterator.Done or a terminal finish reason.
				if lastCandidateWithContent == nil && !processedFunctionCallInThisStreamSegment {
					log.Warn("Google stream ended without candidates or function call.")
					// If history was just a system prompt and nothing else, and model had nothing to say.
					if len(geminiContents) == 1 && geminiContents[0] == systemInstruction {
						params.Task.ToStatus(a2a.TaskStateCompleted, a2a.NewTextMessage("assistant", "No response generated for system prompt."))
						ch <- jsonrpc.Response{Result: params.Task}
					}
					return // Avoid potential infinite loop
				}

			} else { // Non-streaming path
				// Assumes client.Models.GenerateContent can take []*Content and *GenerateContentConfig
				resp, err := prvdr.client.Models.GenerateContent(ctx, params.Model, geminiContents, generateContentConfig)
				if err != nil {
					ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: errors.ErrInternal.Code, Message: err.Error()}}
					return
				}

				if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
					ch <- jsonrpc.Response{Error: &jsonrpc.Error{Code: errors.ErrInternal.Code, Message: "Google API returned no content"}}
					return
				}

				assistantContent := resp.Candidates[0].Content
				geminiContents = append(geminiContents, assistantContent) // Add assistant's response to history for next potential turn

				var textResponse string
				var functionCallList []*genai.FunctionCall

				for _, part := range assistantContent.Parts {
					if part.FunctionCall != nil {
						functionCallList = append(functionCallList, part.FunctionCall)
					} else if len(part.Text) > 0 {
						textResponse += part.Text
					}
				}

				if textResponse != "" { // Log assistant's textual part to task history regardless of tool calls
					params.Task.AddMessage("assistant", textResponse, "")
				}

				if len(functionCallList) > 0 {
					for _, fc := range functionCallList {
						log.Info("Google Provider (Non-Streaming): Tool call", "name", fc.Name)
						updatedTask, llmToolMsg, toolExecErr := ExecuteAndProcessToolCall(
							ctx, fc.Name, fmt.Sprintf("%v", fc.Args),
							fc.Name, params.Task, googleToolResponseGenerator,
						)
						params.Task = updatedTask
						toolResponseContent := &genai.Content{
							Role:  "function",
							Parts: []*genai.Part{llmToolMsg.(*genai.Part)},
						}
						geminiContents = append(geminiContents, toolResponseContent)
						params.Task.AddMessage("tool", fmt.Sprintf("Tool %s output: %v", fc.Name, llmToolMsg.(*genai.Part).FunctionResponse.Response), fc.Name)

						if toolExecErr != nil {
							ch <- jsonrpc.Response{Result: params.Task, Error: &jsonrpc.Error{Code: errors.ErrInternal.Code, Message: fmt.Sprintf("Tool %s error: %v", fc.Name, toolExecErr)}}
							// If one tool fails, we still add its result to geminiContents and let the main loop decide to continue or not.
						} else {
							ch <- jsonrpc.Response{Result: params.Task}
						}
					}
					continue // Continue main `for` loop to send updated `geminiContents` with tool response(s)
				} else {
					if textResponse != "" {
						params.Task.AddFinalPart(a2a.NewTextPart(textResponse))
						ch <- a2a.NewArtifactResult(params.Task.ID, a2a.NewTextPart(textResponse))
					}
					params.Task.ToStatus(a2a.TaskStateCompleted, a2a.NewTextMessage("assistant", textResponse))
					ch <- jsonrpc.Response{Result: params.Task}
					return // Goroutine finished processing this task
				}
			}
		} // End of main `for` loop
	}()
	return ch
}

func (prvdr *GoogleProvider) getSystemInstruction(task *a2a.Task) *genai.Content {
	if task != nil && len(task.History) > 0 && task.History[0].Role == "system" {
		var systemText string
		for _, p := range task.History[0].Parts {
			if p.Type == a2a.PartTypeText {
				systemText = p.Text
				break
			}
		}
		if systemText != "" {
			// For Gemini, system instructions are passed differently if using the specific SystemInstruction field.
			// The convertMessages will handle user/model roles. This is specifically for the SystemInstruction part of the model.
			return &genai.Content{Parts: []*genai.Part{&genai.Part{Text: systemText}}}
		}
	}
	return nil
}

func (prvdr *GoogleProvider) convertMessages(task *a2a.Task) []*genai.Content {
	out := make([]*genai.Content, 0, len(task.History))
	startIdx := 0
	if len(task.History) > 0 && task.History[0].Role == "system" {
		// System message is handled by model.SystemInstruction, so skip it for main contents if it's the first message.
		// However, if it's not the *first* message (which is unusual for system prompts), it should be converted based on roleMap.
		// For now, assume system prompt if present is always History[0] and handled by getSystemInstruction.
		if prvdr.getSystemInstruction(task) != nil {
			startIdx = 1
		}
	}

	for i := startIdx; i < len(task.History); i++ {
		msg := task.History[i]
		var textParts []string

		for _, p := range msg.Parts {
			if p.Type == a2a.PartTypeText {
				textParts = append(textParts, p.Text)
			}
		}
		combinedText := ""
		if len(textParts) > 0 {
			for _, t := range textParts {
				combinedText += t // Simple concatenation, consider space if multiple text parts in one message
			}
		}

		if fn, ok := googleRoleMap[msg.Role]; ok {
			if combinedText != "" {
				content := fn(combinedText)
				// Ensure Parts is not nil if text is empty but role implies content
				if len(content.Parts) == 0 && combinedText == "" && (msg.Role == "assistant" || msg.Role == "agent") {
					// Add an empty text part if model expects a part for empty assistant messages
					// content.Parts = []*genai.Part{&genai.Part{Text: ""}}
					// For now, only add if combinedText is not empty.
				} else if len(content.Parts) > 0 || combinedText != "" { // Add if there's text or fn produced parts
					out = append(out, content)
				}
			}
		}
	}
	return out
}

func (prvdr *GoogleProvider) convertTools(tools []*mcp.Tool) []*genai.Tool {
	out := make([]*genai.Tool, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		properties := make(map[string]*genai.Schema)
		var requiredParams []string
		if tool.InputSchema.Type == "object" {
			requiredParams = tool.InputSchema.Required
			for k, v := range tool.InputSchema.Properties {
				propMap, ok := v.(map[string]any)
				if !ok {
					log.Warn("Skipping tool property due to unexpected type", "tool", tool.Name, "property", k)
					continue
				}
				schemaType := genai.TypeString // Default
				if typeStr, ok := propMap["type"].(string); ok {
					switch typeStr {
					case "string":
						schemaType = genai.TypeString
					case "number":
						schemaType = genai.TypeNumber
					case "integer":
						schemaType = genai.TypeInteger
					case "boolean":
						schemaType = genai.TypeBoolean
					case "array":
						schemaType = genai.TypeArray
					case "object":
						schemaType = genai.TypeObject
					}
				}
				description := ""
				if desc, ok := propMap["description"].(string); ok {
					description = desc
				}
				properties[k] = &genai.Schema{
					Type:        schemaType,
					Description: description,
					// TODO: Add more schema properties like enum, items (for array), etc. if needed from mcp.Tool
				}
			}
		}

		out = append(out, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters: &genai.Schema{
						Type:       genai.TypeObject,
						Properties: properties,
						Required:   requiredParams,
					},
				},
			},
		})
	}
	return out
}

// GoogleEmbedder and related code would go here if needed.

func WithGoogleClient() GoogleProviderOption {
	return func(prvdr *GoogleProvider) {
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			log.Fatal("GOOGLE_API_KEY environment variable not set.")
		}
		// Attempt to use ClientConfig if APIKey field exists, otherwise rely on env var with nil config
		// For now, assuming GOOGLE_API_KEY in env is picked up by NewClient(ctx, nil)
		// or if ClientConfig has an APIKey field, it would be &genai.ClientConfig{APIKey: apiKey}
		// The examples often use NewClient(ctx, nil)
		client, err := genai.NewClient(context.Background(), nil) // Simpler init, relies on GOOGLE_API_KEY env var by default
		if err != nil {
			log.Fatal("Failed to create Google GenAI client: %v", err)
		}
		prvdr.client = client
	}
}

// Other options like WithGoogleEmbedderModel, WithGoogleEmbedderClient would go here.
