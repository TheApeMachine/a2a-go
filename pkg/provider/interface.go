package provider

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
)

type Interface interface {
	Generate(context.Context, *ProviderParams) chan jsonrpc.Response
	Embed(context.Context, string) ([]float32, error)
}

type ProviderParams struct {
	Task              *a2a.Task
	Model             string
	Tools             []*mcp.Tool
	Schema            interface{}
	Temperature       float64
	MaxTokens         int64
	TopP              float64
	TopK              int64
	FrequencyPenalty  float64
	PresencePenalty   float64
	Seed              int64
	Stop              []string
	Stream            bool
	ParallelToolCalls bool
}

type ProviderParamsOption func(*ProviderParams)

func NewProviderParams(
	task *a2a.Task, options ...ProviderParamsOption,
) *ProviderParams {
	params := &ProviderParams{
		Task:              task,
		Model:             "gpt-4o-mini",
		Schema:            nil,
		Temperature:       0.5,
		MaxTokens:         1000,
		TopP:              1.0,
		TopK:              100,
		FrequencyPenalty:  0.0,
		PresencePenalty:   0.0,
		Seed:              0,
		Stop:              []string{},
		Stream:            true,
		ParallelToolCalls: true,
	}

	for _, option := range options {
		option(params)
	}

	log.Info("new provider params", "params", params)

	return params
}

func WithModel(model string) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.Model = model
	}
}

func WithTools(tools ...*mcp.Tool) ProviderParamsOption {
	return func(params *ProviderParams) {
		log.Info("with tools", "tools", tools)
		params.Tools = tools
	}
}

func WithSchema(schema interface{}) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.Schema = schema
	}
}

func WithTemperature(temperature float64) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.Temperature = temperature
	}
}

func WithMaxTokens(maxTokens int64) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.MaxTokens = maxTokens
	}
}

func WithTopP(topP float64) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.TopP = topP
	}
}

func WithTopK(topK int64) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.TopK = topK
	}
}

func WithFrequencyPenalty(frequencyPenalty float64) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.FrequencyPenalty = frequencyPenalty
	}
}

func WithPresencePenalty(presencePenalty float64) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.PresencePenalty = presencePenalty
	}
}

func WithSeed(seed int64) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.Seed = seed
	}
}

func WithStop(stop []string) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.Stop = stop
	}
}

func WithStream(stream bool) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.Stream = stream
	}
}

func WithParallelToolCalls(parallelToolCalls bool) ProviderParamsOption {
	return func(params *ProviderParams) {
		params.ParallelToolCalls = parallelToolCalls
	}
}
