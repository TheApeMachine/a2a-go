package a2a

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
)

// A2AServer is safe for concurrent use by default because RPCServer &
// SSEBroker are.
type A2AServer struct {
    Card        AgentCard
    TaskManager TaskManager

    rpc    *RPCServer
    broker *SSEBroker
}

// NewA2AServer constructs a server with the supplied TaskManager.  The caller
// must later mount Handlers().  This decouples protocol concerns from HTTP
// routing frameworks (std net/http, gin, chi, …).
func NewA2AServer(card AgentCard, tm TaskManager) *A2AServer {
    srv := &A2AServer{
        Card:        card,
        TaskManager: tm,
        rpc:         NewRPCServer(),
        broker:      NewSSEBroker(),
    }
    srv.registerRPCHandlers()
    return srv
}

// NewA2AServerWithDefaults returns a fully working server that echoes user
// input.  Great for smoke tests.
func NewA2AServerWithDefaults(url string) *A2AServer {
    card := AgentCard{
        Name:    "Echo Agent (Go)",
        URL:     url,
        Version: "0.1.0",
        Capabilities: AgentCapabilities{
            Streaming: true,
        },
        Skills: []AgentSkill{{ID: "echo", Name: "Echo"}},
    }
    return NewA2AServer(card, NewEchoTaskManager(nil))
}

// Handlers returns a map of path → http.Handler to be mounted by the host
// application.  By default two endpoints are exposed:
//   /rpc     – JSON‑RPC 2.0
//   /events  – SSE stream
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
        var params TaskSendParams
        if err := json.Unmarshal(raw, &params); err != nil {
            return nil, errInvalidParams
        }
        task, rpcErr := s.TaskManager.SendTask(ctx, params)
        if rpcErr != nil {
            return nil, rpcErr
        }
        return task, nil
    })

    // tasks/get
    s.rpc.Register("tasks/get", func(ctx context.Context, raw json.RawMessage) (interface{}, *rpcError) {
        var qp struct {
            ID           string `json:"id"`
            HistoryLength int   `json:"historyLength,omitempty"`
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
        var p struct{ ID string `json:"id"` }
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
        var params TaskSendParams
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
            first = TaskStatusUpdateEvent{
                ID: params.ID,
                Status: TaskStatus{State: TaskStateWorking},
                Final: false,
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
