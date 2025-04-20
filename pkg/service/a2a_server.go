package service

import (
	"context"
	"encoding/json"
	"net/http"

	"os"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/prompts"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/resources"
	"github.com/theapemachine/a2a-go/pkg/roots"
	"github.com/theapemachine/a2a-go/pkg/sampling"
	"github.com/theapemachine/a2a-go/pkg/service/sse"
	"github.com/theapemachine/a2a-go/pkg/tasks"
	"github.com/theapemachine/a2a-go/pkg/types"
)

/*
A2AServer is safe for concurrent use by default because
RPCServer & SSEBroker are.
*/
type A2AServer struct {
	Card        types.AgentCard
	TaskManager tasks.TaskManager

	PromptManager   prompts.PromptManager
	ResourceManager resources.ResourceManager
	SamplingManager sampling.Manager
	RootManager     *roots.Manager

	rpc    *RPCServer
	broker *sse.SSEBroker
}

/*
chooseSamplingManager returns OpenAI manager if API key available 
otherwise falls back to dummy echo manager.
*/
func chooseSamplingManager() sampling.Manager {
	if os.Getenv("OPENAI_API_KEY") != "" {
		return provider.NewOpenAISamplingManager(nil)
	}
	return sampling.NewDefaultManager()
}

/*
NewA2AServer constructs a server with the supplied TaskManager.  The caller

must later mount Handlers().  This decouples protocol concerns from HTTP
routing frameworks (std net/http, gin, chi, …).
*/
func NewA2AServer(card types.AgentCard, tm tasks.TaskManager) *A2AServer {
	srv := &A2AServer{
		Card:            card,
		TaskManager:     tm,
		PromptManager:   prompts.NewDefaultManager(),
		ResourceManager: resources.NewDefaultManager(),
		SamplingManager: chooseSamplingManager(),
		RootManager:     roots.NewManager(),
		rpc:             NewRPCServer(),
		broker:          sse.NewSSEBroker(),
	}
	srv.registerRPCHandlers()
	return srv
}

/*
NewA2AServerWithDefaults returns a fully working server that echoes 
user input. Great for smoke tests.
*/
func NewA2AServerWithDefaults(url string) *A2AServer {
	card := types.AgentCard{
		Name:        "Echo Agent (Go)",
		URL:         url,
		Version:     "0.1.0",
		Description: stringPtr("A simple echo agent that demonstrates A2A protocol features"),
		Capabilities: types.AgentCapabilities{
			Streaming:              true,
			PushNotifications:      true,
			StateTransitionHistory: true,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []types.AgentSkill{{
			ID:          "echo",
			Name:        "Echo",
			Description: stringPtr("Echoes back user input"),
			Examples:    []string{"Hello A2A!", "Echo this message"},
		}},
	}
	return NewA2AServer(card, tasks.NewEchoTaskManager(nil))
}

func stringPtr(s string) *string {
	return &s
}

/*
Handlers returns a map of path → http.Handler to be mounted by the host

application.  By default two endpoints are exposed:

/rpc     – JSON‑RPC 2.0
/events  – SSE stream
*/
func (s *A2AServer) Handlers() map[string]http.Handler {
	return map[string]http.Handler{
		"/rpc":    s.rpc,
		"/events": http.HandlerFunc(s.broker.Subscribe),
	}
}

func (s *A2AServer) registerRPCHandlers() {
	s.rpc.Register(
		"tasks/send",
		func(
			ctx context.Context, raw json.RawMessage,
		) (any, *errors.RpcError) {
			return tasks.Send(ctx, raw, s.TaskManager)
		},
	)

	s.rpc.Register(
		"tasks/pushNotification/set",
		func(
			ctx context.Context, raw json.RawMessage,
		) (any, *errors.RpcError) {
			return tasks.SetPushNotification(ctx, raw, s.TaskManager)
		},
	)

	s.rpc.Register(
		"tasks/pushNotification/get",
		func(
			ctx context.Context, raw json.RawMessage,
		) (any, *errors.RpcError) {
			return tasks.GetPushNotification(ctx, raw, s.TaskManager)
		},
	)

	s.rpc.Register(
		"tasks/resubscribe",
		func(
			ctx context.Context, raw json.RawMessage,
		) (any, *errors.RpcError) {
			return tasks.ResubscribeTask(ctx, raw, s.TaskManager, s.broker)
		},
	)

	promptHandler := prompts.NewMCPHandler(s.PromptManager)

	s.rpc.Register("prompts/list", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return prompts.List(ctx, raw, promptHandler)
	})

	s.rpc.Register("prompts/get", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return prompts.Get(ctx, raw, promptHandler)
	})

	resHandler := resources.NewMCPHandler(s.ResourceManager)

	s.rpc.Register("resources/list", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return resources.List(ctx, raw, resHandler)
	})

	s.rpc.Register("resources/read", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return resources.Read(ctx, raw, resHandler)
	})

	rootsHandler := roots.NewMCPHandler(s.RootManager)

	s.rpc.Register("roots/list", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return roots.List(ctx, raw, rootsHandler)
	})

	s.rpc.Register("roots/create", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return roots.Create(ctx, raw, rootsHandler)
	})

	sampHandler := sampling.NewMCPHandler(s.SamplingManager)

	s.rpc.Register("sampling/createMessage", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return sampling.CreateMessage(ctx, raw, sampHandler)
	})

	// sampling/createMessageStream – first delta returned, rest over SSE
	s.rpc.Register("sampling/createMessageStream", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return sampling.CreateMessageStream(ctx, raw, sampHandler, s.broker)
	})

	// tasks/get
	s.rpc.Register("tasks/get", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return tasks.Get(ctx, raw, s.TaskManager)
	})

	// tasks/cancel
	s.rpc.Register("tasks/cancel", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return tasks.Cancel(ctx, raw, s.TaskManager)
	})

	// tasks/sendSubscribe (basic implementation)
	s.rpc.Register("tasks/sendSubscribe", func(
		ctx context.Context,
		raw json.RawMessage,
	) (any, *errors.RpcError) {
		return tasks.SendSubscribe(ctx, raw, s.TaskManager, s.broker)
	})
}
