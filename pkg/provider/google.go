package provider

import (
	"context"
	"os"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/a2a"
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
	params *genai.GenerateContentConfig
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

	go func() {
		defer close(ch)

		prvdr.params = &genai.GenerateContentConfig{
			Temperature:     genai.Ptr[float32](float32(params.Temperature)),
			MaxOutputTokens: int32(params.MaxTokens),
			TopP:            genai.Ptr[float32](float32(params.TopP)),
			TopK:            genai.Ptr[float32](float32(params.TopK)),
		}

		isDone := false

		for !isDone {
			if params.Stream {
				stream := prvdr.client.Models.GenerateContentStream(ctx, params.Model, prvdr.convertMessages(params.Task), prvdr.params)
				for result, err := range stream {
					if err != nil {
						log.Error("stream error", "error", err)
						ch <- jsonrpc.Response{
							Error: &jsonrpc.Error{
								Code:    int(a2a.ErrorCodeInternalError),
								Message: err.Error(),
							},
						}
						return
					}

					if result.Text() != "" {
						ch <- a2a.NewArtifactResult(
							params.Task.ID,
							a2a.NewTextPart(result.Text()),
						)
					}

					if result.Candidates[0].FinishReason == "STOP" {
						isDone = true
					}
				}
			} else {
				result, err := prvdr.client.Models.GenerateContent(ctx, params.Model, prvdr.convertMessages(params.Task), prvdr.params)
				if err != nil {
					log.Error("failed to generate content", "error", err)
					ch <- jsonrpc.Response{
						Error: &jsonrpc.Error{
							Code:    int(a2a.ErrorCodeInternalError),
							Message: err.Error(),
						},
					}
					return
				}

				if result.Text() != "" {
					ch <- a2a.NewArtifactResult(
						params.Task.ID,
						a2a.NewTextPart(result.Text()),
					)
					params.Task.AddFinalPart(a2a.NewTextPart(result.Text()))
				}

				isDone = true
			}
		}
	}()

	return ch
}

func (prvdr *GoogleProvider) convertMessages(
	task *a2a.Task,
) []*genai.Content {
	out := make([]*genai.Content, 0, len(task.History))

	for _, msg := range task.History {
		var text string

		for _, p := range msg.Parts {
			if p.Type == a2a.PartTypeText {
				text = p.Text
				break
			}
		}

		if fn, ok := googleRoleMap[msg.Role]; ok {
			out = append(out, fn(text))
		}
	}
	return out
}

func (prvdr *GoogleProvider) convertTools(
	tools []*mcp.Tool,
) []*genai.Tool {
	out := make([]*genai.Tool, 0, len(tools))

	for _, tool := range tools {
		if tool == nil {
			continue
		}

		// Convert the properties to a schema
		properties := make(map[string]*genai.Schema)
		for k, v := range tool.InputSchema.Properties {
			// Get the type from the property
			propMap, ok := v.(map[string]any)
			if !ok {
				continue
			}

			// Map the type to the appropriate schema type
			var schemaType genai.Type
			switch propMap["type"] {
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
			default:
				schemaType = genai.TypeString // Default to string if type is unknown
			}

			// Create the schema with all possible fields
			schema := &genai.Schema{
				Type:        schemaType,
				Description: propMap["description"].(string),
				Title:       propMap["title"].(string),
				Format:      propMap["format"].(string),
				Pattern:     propMap["pattern"].(string),
			}

			// Handle enum values
			if enum, ok := propMap["enum"].([]any); ok {
				enumStrings := make([]string, len(enum))
				for i, e := range enum {
					enumStrings[i] = e.(string)
				}
				schema.Enum = enumStrings
			}

			// Handle nullable
			if nullable, ok := propMap["nullable"].(bool); ok {
				schema.Nullable = &nullable
			}

			// Handle numeric constraints
			if min, ok := propMap["minimum"].(float64); ok {
				schema.Minimum = &min
			}
			if max, ok := propMap["maximum"].(float64); ok {
				schema.Maximum = &max
			}

			// Handle string constraints
			if minLen, ok := propMap["minLength"].(float64); ok {
				minLenInt := int64(minLen)
				schema.MinLength = &minLenInt
			}
			if maxLen, ok := propMap["maxLength"].(float64); ok {
				maxLenInt := int64(maxLen)
				schema.MaxLength = &maxLenInt
			}

			// Handle array constraints
			if schemaType == genai.TypeArray {
				if minItems, ok := propMap["minItems"].(float64); ok {
					minItemsInt := int64(minItems)
					schema.MinItems = &minItemsInt
				}
				if maxItems, ok := propMap["maxItems"].(float64); ok {
					maxItemsInt := int64(maxItems)
					schema.MaxItems = &maxItemsInt
				}
				if items, ok := propMap["items"].(map[string]any); ok {
					schema.Items = prvdr.convertSchema(items)
				}
			}

			// Handle object constraints
			if schemaType == genai.TypeObject {
				if minProps, ok := propMap["minProperties"].(float64); ok {
					minPropsInt := int64(minProps)
					schema.MinProperties = &minPropsInt
				}
				if maxProps, ok := propMap["maxProperties"].(float64); ok {
					maxPropsInt := int64(maxProps)
					schema.MaxProperties = &maxPropsInt
				}
				if props, ok := propMap["properties"].(map[string]any); ok {
					schema.Properties = make(map[string]*genai.Schema)
					for pk, pv := range props {
						if propMap, ok := pv.(map[string]any); ok {
							schema.Properties[pk] = prvdr.convertSchema(propMap)
						}
					}
				}
				if required, ok := propMap["required"].([]any); ok {
					requiredStrings := make([]string, len(required))
					for i, r := range required {
						requiredStrings[i] = r.(string)
					}
					schema.Required = requiredStrings
				}
			}

			// Handle anyOf
			if anyOf, ok := propMap["anyOf"].([]any); ok {
				schema.AnyOf = make([]*genai.Schema, len(anyOf))
				for i, a := range anyOf {
					if anyOfMap, ok := a.(map[string]any); ok {
						schema.AnyOf[i] = prvdr.convertSchema(anyOfMap)
					}
				}
			}

			properties[k] = schema
		}

		out = append(out, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters: &genai.Schema{
						Type:       genai.TypeObject,
						Properties: properties,
					},
				},
			},
		})
	}

	return out
}

// Helper function to convert a map to a Schema
func (prvdr *GoogleProvider) convertSchema(propMap map[string]any) *genai.Schema {
	var schemaType genai.Type
	switch propMap["type"] {
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
	default:
		schemaType = genai.TypeString
	}

	schema := &genai.Schema{
		Type:        schemaType,
		Description: propMap["description"].(string),
		Title:       propMap["title"].(string),
		Format:      propMap["format"].(string),
		Pattern:     propMap["pattern"].(string),
	}

	// Handle enum values
	if enum, ok := propMap["enum"].([]any); ok {
		enumStrings := make([]string, len(enum))
		for i, e := range enum {
			enumStrings[i] = e.(string)
		}
		schema.Enum = enumStrings
	}

	// Handle nullable
	if nullable, ok := propMap["nullable"].(bool); ok {
		schema.Nullable = &nullable
	}

	// Handle numeric constraints
	if min, ok := propMap["minimum"].(float64); ok {
		schema.Minimum = &min
	}
	if max, ok := propMap["maximum"].(float64); ok {
		schema.Maximum = &max
	}

	// Handle string constraints
	if minLen, ok := propMap["minLength"].(float64); ok {
		minLenInt := int64(minLen)
		schema.MinLength = &minLenInt
	}
	if maxLen, ok := propMap["maxLength"].(float64); ok {
		maxLenInt := int64(maxLen)
		schema.MaxLength = &maxLenInt
	}

	// Handle array constraints
	if schemaType == genai.TypeArray {
		if minItems, ok := propMap["minItems"].(float64); ok {
			minItemsInt := int64(minItems)
			schema.MinItems = &minItemsInt
		}
		if maxItems, ok := propMap["maxItems"].(float64); ok {
			maxItemsInt := int64(maxItems)
			schema.MaxItems = &maxItemsInt
		}
		if items, ok := propMap["items"].(map[string]any); ok {
			schema.Items = prvdr.convertSchema(items)
		}
	}

	// Handle object constraints
	if schemaType == genai.TypeObject {
		if minProps, ok := propMap["minProperties"].(float64); ok {
			minPropsInt := int64(minProps)
			schema.MinProperties = &minPropsInt
		}
		if maxProps, ok := propMap["maxProperties"].(float64); ok {
			maxPropsInt := int64(maxProps)
			schema.MaxProperties = &maxPropsInt
		}
		if props, ok := propMap["properties"].(map[string]any); ok {
			schema.Properties = make(map[string]*genai.Schema)
			for pk, pv := range props {
				if propMap, ok := pv.(map[string]any); ok {
					schema.Properties[pk] = prvdr.convertSchema(propMap)
				}
			}
		}
		if required, ok := propMap["required"].([]any); ok {
			requiredStrings := make([]string, len(required))
			for i, r := range required {
				requiredStrings[i] = r.(string)
			}
			schema.Required = requiredStrings
		}
	}

	// Handle anyOf
	if anyOf, ok := propMap["anyOf"].([]any); ok {
		schema.AnyOf = make([]*genai.Schema, len(anyOf))
		for i, a := range anyOf {
			if anyOfMap, ok := a.(map[string]any); ok {
				schema.AnyOf[i] = prvdr.convertSchema(anyOfMap)
			}
		}
	}

	return schema
}

type GoogleEmbedder struct {
	api   *genai.Client
	Model string
}

type GoogleEmbedderOption func(*GoogleEmbedder)

func NewGoogleEmbedder(options ...GoogleEmbedderOption) *GoogleEmbedder {
	embedder := &GoogleEmbedder{}

	for _, option := range options {
		option(embedder)
	}

	return embedder
}

func (e *GoogleEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	result, err := e.api.Models.EmbedContent(ctx, e.Model, genai.Text(text), &genai.EmbedContentConfig{})
	if err != nil {
		return nil, err
	}
	return result.Embeddings[0].Values, nil
}

func (e *GoogleEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result, err := e.api.Models.EmbedContent(ctx, e.Model, genai.Text(texts[0]), &genai.EmbedContentConfig{})
	if err != nil {
		return nil, err
	}

	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = result.Embeddings[0].Values
	}
	return out, nil
}

func WithGoogleClient() GoogleProviderOption {
	return func(prvdr *GoogleProvider) {
		// Check if we should use Vertex AI or Gemini API
		useVertexAI := os.Getenv("GOOGLE_GENAI_USE_VERTEXAI") == "true"
		var backend genai.Backend
		if useVertexAI {
			backend = genai.BackendVertexAI
		} else {
			backend = genai.BackendGeminiAPI
		}

		client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
			Backend: backend,
		})
		if err != nil {
			log.Error("failed to create Google client", "error", err)
			return
		}

		prvdr.client = client
	}
}

func WithGoogleEmbedderModel(model string) GoogleEmbedderOption {
	return func(e *GoogleEmbedder) {
		e.Model = model
	}
}

func WithGoogleEmbedderClient(client *genai.Client) GoogleEmbedderOption {
	return func(e *GoogleEmbedder) {
		e.api = client
	}
}
