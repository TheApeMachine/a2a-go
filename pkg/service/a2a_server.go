package service

// A2AServer bundles JSON‑RPC, SSE and a TaskManager to expose a fully
// functional A2A server with minimal glue code.  Callers create the server,
// register their TaskManager implementation and then mount the HTTP handlers
// returned by Handlers() on their preferred mux.

// For simple demos use NewA2AServerWithDefaults which wires an EchoTaskManager
// and an InMemoryTaskStore.

import (
	"context"
	"encoding/json"
	"net/http"

	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/prompts"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/resources"
	"github.com/theapemachine/a2a-go/pkg/roots"
	"github.com/theapemachine/a2a-go/pkg/sampling"
	"github.com/theapemachine/a2a-go/pkg/types"
)

// A2AServer is safe for concurrent use by default because RPCServer &
// SSEBroker are.
type A2AServer struct {
	Card        types.AgentCard
	TaskManager TaskManager

	PromptManager   prompts.PromptManager
	ResourceManager resources.ResourceManager
	SamplingManager sampling.Manager
	RootManager     *roots.Manager

	rpc    *RPCServer
	broker *SSEBroker
}

// chooseSamplingManager returns OpenAI manager if API key available otherwise
// falls back to dummy echo manager.
func chooseSamplingManager() sampling.Manager {
	if os.Getenv("OPENAI_API_KEY") != "" {
		return provider.NewOpenAISamplingManager(nil)
	}
	return sampling.NewDefaultManager()
}

// NewA2AServer constructs a server with the supplied TaskManager.  The caller
// must later mount Handlers().  This decouples protocol concerns from HTTP
// routing frameworks (std net/http, gin, chi, …).
func NewA2AServer(card types.AgentCard, tm TaskManager) *A2AServer {
	srv := &A2AServer{
		Card:            card,
		TaskManager:     tm,
		PromptManager:   prompts.NewDefaultManager(),
		ResourceManager: resources.NewDefaultManager(),
		SamplingManager: chooseSamplingManager(),
		RootManager:     roots.NewManager(),
		rpc:             NewRPCServer(),
		broker:          NewSSEBroker(),
	}
	srv.registerRPCHandlers()
	return srv
}

// NewA2AServerWithDefaults returns a fully working server that echoes user
// input.  Great for smoke tests.
func NewA2AServerWithDefaults(url string) *A2AServer {
	card := types.AgentCard{
		Name:    "Echo Agent (Go)",
		URL:     url,
		Version: "0.1.0",
		Capabilities: types.AgentCapabilities{
			Streaming: true,
		},
		Skills: []types.AgentSkill{{ID: "echo", Name: "Echo"}},
	}
	return NewA2AServer(card, NewEchoTaskManager(nil))
}

// Handlers returns a map of path → http.Handler to be mounted by the host
// application.  By default two endpoints are exposed:
//
//	/rpc     – JSON‑RPC 2.0
//	/events  – SSE stream
func (s *A2AServer) Handlers() map[string]http.Handler {
	return map[string]http.Handler{
		"/rpc":    s.rpc,
		"/events": http.HandlerFunc(s.broker.Subscribe),
	}
}

// ---------------------------------------------------------------------------
// Internal wiring
// ---------------------------------------------------------------------------

func (s *A2AServer) registerRPCHandlers() {
	// tasks/send
	s.rpc.Register("tasks/send", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		var params types.TaskSendParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, errInvalidParams
		}
		task, rpcErr := s.TaskManager.SendTask(ctx, params)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return task, nil
	})

	// ---------------------------------------------------------------------
	// MCP‑Prompts namespace
	// ---------------------------------------------------------------------

	promptHandler := prompts.NewMCPHandler(s.PromptManager)

	s.rpc.Register("prompts/list", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		res, err := promptHandler.HandleListPrompts(ctx, &mcp.ListPromptsRequest{})
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		return res, nil
	})

	s.rpc.Register("prompts/get", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, errInvalidParams
		}
		req := &mcp.GetPromptRequest{}
		req.Params.Name = p.Name
		res, err := promptHandler.HandleGetPrompt(ctx, req)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		return res, nil
	})

	// ---------------------------------------------------------------------
	// MCP‑Resources namespace
	// ---------------------------------------------------------------------

	resHandler := resources.NewMCPHandler(s.ResourceManager)

	s.rpc.Register("resources/list", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		res, err := resHandler.HandleListResources(ctx, &mcp.ListResourcesRequest{})
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		return res, nil
	})

	s.rpc.Register("resources/read", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		var p struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, errInvalidParams
		}
		req := &mcp.ReadResourceRequest{}
		req.Params.URI = p.URI
		res, err := resHandler.HandleReadResource(ctx, req)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		return res, nil
	})

	// ---------------------------------------------------------------------
	// MCP‑Roots namespace (minimal subset list + create)
	// ---------------------------------------------------------------------

	rootsHandler := roots.NewMCPHandler(s.RootManager)

	s.rpc.Register("roots/list", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		res, err := rootsHandler.HandleListRoots(ctx)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		return res, nil
	})

	s.rpc.Register("roots/create", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		root, err := rootsHandler.HandleCreateRoot(ctx, raw)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		return root, nil
	})

	// ---------------------------------------------------------------------
	// MCP‑Sampling namespace (createMessage, no streaming yet)
	// ---------------------------------------------------------------------

	sampHandler := sampling.NewMCPHandler(s.SamplingManager)

	s.rpc.Register("sampling/createMessage", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		var req mcp.CreateMessageRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, errInvalidParams
		}
		res, err := sampHandler.HandleCreateMessage(ctx, &req)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		return res, nil
	})

	// sampling/createMessageStream – first delta returned, rest over SSE
	s.rpc.Register("sampling/createMessageStream", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		var req mcp.CreateMessageRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, errInvalidParams
		}

		stream, err := sampHandler.HandleStreamMessage(ctx, &req)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}

		// retrieve first result synchronously
		var first *mcp.CreateMessageResult
		select {
		case first = <-stream:
		default:
			// if none ready yet produce empty chunk so caller can start reading.
			first = &mcp.CreateMessageResult{}
		}

		// forward remainder asynchronously to SSE.
		go func() {
			for res := range stream {
				_ = s.broker.Broadcast(res)
			}
		}()

		return first, nil
	})

	// tasks/get
	s.rpc.Register("tasks/get", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		var qp struct {
			ID            string `json:"id"`
			HistoryLength int    `json:"historyLength,omitempty"`
		}
		if err := json.Unmarshal(raw, &qp); err != nil {
			return nil, errInvalidParams
		}
		task, rpcErr := s.TaskManager.GetTask(ctx, qp.ID, qp.HistoryLength)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return task, nil
	})

	// tasks/cancel
	s.rpc.Register("tasks/cancel", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, errInvalidParams
		}
		task, rpcErr := s.TaskManager.CancelTask(ctx, p.ID)
		if rpcErr != nil {
			return nil, rpcErr
		}
		return task, nil
	})

	// tasks/sendSubscribe (basic implementation)
	s.rpc.Register("tasks/sendSubscribe", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
		var params types.TaskSendParams
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, errInvalidParams
		}

		stream, rpcErr := s.TaskManager.StreamTask(ctx, params)
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Consume first event to return immediately per JSON‑RPC semantics.
		var first interface{}
		select {
		case first = <-stream:
		default:
			// no event yet – fabricate a working status so caller gets something
			first = types.TaskStatusUpdateEvent{
				ID:     params.ID,
				Status: types.TaskStatus{State: types.TaskStateWorking},
				Final:  false,
			}
		}

		// forward rest of events to SSE broker
		go func() {
			for evt := range stream {
				_ = s.broker.Broadcast(evt)
			}
		}()

		return first, nil
	})
}
