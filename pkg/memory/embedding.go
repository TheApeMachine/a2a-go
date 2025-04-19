package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"bytes"
)

// OpenAIEmbeddingService generates embeddings using OpenAI's API
type OpenAIEmbeddingService struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// OpenAIEmbeddingRequest represents a request to the OpenAI embedding API
type OpenAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// OpenAIEmbeddingResponse represents a response from the OpenAI embedding API
type OpenAIEmbeddingResponse struct {
	Data  []OpenAIEmbeddingData `json:"data"`
	Model string                `json:"model"`
	Usage OpenAIUsage           `json:"usage"`
}

// OpenAIEmbeddingData represents an embedding result
type OpenAIEmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
	Object    string    `json:"object"`
}

// OpenAIUsage tracks token usage
type OpenAIUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// NewOpenAIEmbeddingService creates a new embedding service using OpenAI
func NewOpenAIEmbeddingService(apiKey string) *OpenAIEmbeddingService {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	
	return &OpenAIEmbeddingService{
		APIKey: apiKey,
		Model:  "text-embedding-ada-002", // Default model, can be customized
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateEmbedding creates an embedding for a single text
func (s *OpenAIEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := s.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	
	return embeddings[0], nil
}

// GenerateEmbeddings creates embeddings for multiple texts in a batch
func (s *OpenAIEmbeddingService) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	
	// Prepare texts - trim and clean
	for i, text := range texts {
		texts[i] = strings.TrimSpace(text)
	}
	
	// Create the request
	reqData := OpenAIEmbeddingRequest{
		Model: s.Model,
		Input: texts,
	}
	
	reqBody, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.openai.com/v1/embeddings",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	
	// Send request
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return nil, fmt.Errorf("OpenAI API error (status %d)", resp.StatusCode)
		}
		
		return nil, fmt.Errorf("OpenAI API error: %s (%s)", errorResponse.Error.Message, errorResponse.Error.Type)
	}
	
	// Parse response
	var embeddingResponse OpenAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Extract embeddings in order
	result := make([][]float32, len(texts))
	for _, item := range embeddingResponse.Data {
		if item.Index >= len(result) {
			continue // Skip out-of-range indices
		}
		result[item.Index] = item.Embedding
	}
	
	return result, nil
}

// MockEmbeddingService generates mock embeddings for testing
type MockEmbeddingService struct{}

// NewMockEmbeddingService creates a new mock embedding service for testing
func NewMockEmbeddingService() *MockEmbeddingService {
	return &MockEmbeddingService{}
}

// GenerateEmbedding creates a mock embedding
func (s *MockEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Generate a deterministic mock embedding based on the text
	embedding := make([]float32, 4) // Small dimension for testing
	for i := 0; i < 4; i++ {
		// Use a simple hash of the text to generate mock values
		if len(text) > i {
			embedding[i] = float32(text[i % len(text)]) / 256.0
		} else {
			embedding[i] = 0.5 // Default value
		}
	}
	return embedding, nil
}

// GenerateEmbeddings creates mock embeddings for a batch of texts
func (s *MockEmbeddingService) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		embedding, _ := s.GenerateEmbedding(ctx, text)
		result[i] = embedding
	}
	return result, nil
}