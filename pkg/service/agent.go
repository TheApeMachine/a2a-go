package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v3"
	fiberadaptor "github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/service/sse"
)

/*
A2AServer is safe for concurrent use by default because
RPCServer & SSEBroker are.
*/
type A2AServer struct {
	app    *fiber.App
	agent  *ai.Agent
	broker *sse.SSEBroker
}

/*
NewA2AServer constructs a server with the supplied Agent.
*/
func NewAgentServer(agent *ai.Agent) *A2AServer {
	return &A2AServer{
		app: fiber.New(fiber.Config{
			AppName:           agent.Name(),
			ServerHeader:      "A2A-Agent-Server",
			StreamRequestBody: true,
		}),
		agent:  agent,
		broker: sse.NewSSEBroker(),
	}
}

func (srv *A2AServer) Start() error {
	srv.app.Use(logger.New(logger.Config{
		// Skip logging for the /events endpoint to reduce noise
		Next: func(c fiber.Ctx) bool {
			return c.Path() == "/events"
		},
	}), healthcheck.NewHealthChecker())
	srv.app.Get("/", srv.handleRoot)
	srv.app.Get("/.well-known/agent.json", srv.handleAgentCard)
	srv.app.Get("/events", srv.handleEvents)
	srv.app.Post("/rpc", srv.handleRPC)
	return srv.app.Listen(":3210", fiber.ListenConfig{DisableStartupMessage: true})
}

func (srv *A2AServer) handleRoot(ctx fiber.Ctx) error {
	return ctx.SendString("OK")
}

func (srv *A2AServer) handleAgentCard(ctx fiber.Ctx) error {
	return ctx.JSON(srv.agent.Card())
}

func (srv *A2AServer) handleEvents(ctx fiber.Ctx) error {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Ensure standard SSE headers are set for clients
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		srv.broker.Subscribe(w, r)
	}
	return fiberadaptor.HTTPHandler(http.HandlerFunc(handler))(ctx)
}

func (srv *A2AServer) parseParamsWithDecoding(params any) ([]byte, error) {
	var paramsBytes []byte
	var err error

	switch v := params.(type) {
	case string:
		decoded, decodeErr := base64.StdEncoding.DecodeString(v)
		if decodeErr == nil {
			paramsBytes = decoded
			break
		}

		paramsBytes = []byte(v)
	case []byte:
		paramsBytes = v
	default:
		paramsBytes, err = json.Marshal(params)

		if err != nil {
			log.Error("failed to marshal params", "error", err)
			return nil, err
		}
	}

	return paramsBytes, nil
}

// parseAndUnmarshalParams handles decoding and unmarshalling of RPC parameters.
func (srv *A2AServer) parseAndUnmarshalParams(rawParams any, out any) *errors.RpcError {
	paramsBytes, err := srv.parseParamsWithDecoding(rawParams)
	if err != nil {
		return errors.ErrInvalidParams.WithMessagef("failed to parse params: %v", err)
	}

	if err := json.Unmarshal(paramsBytes, out); err != nil {
		log.Error("failed to unmarshal params", "error", err, "params", string(paramsBytes))
		return errors.ErrInvalidParams.WithMessagef("failed to unmarshal params: %v", err)
	}
	return nil
}

// forwardEventsToBroker reads from a channel until closed or the context is done
// and broadcasts each event on the SSE broker.
func (srv *A2AServer) forwardEventsToBroker(ctx context.Context, stream <-chan any) {
	go func() {
		for {
			select {
			case evt, ok := <-stream:
				if !ok {
					return
				}
				if err := srv.broker.Broadcast(evt); err != nil {
					log.Error("failed to broadcast event in forwardEventsToBroker", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// forwardResponseChannelToAnyChannel creates a channel adapter from chan jsonrpc.Response to <-chan any
func forwardResponseChannelToAnyChannel(ctx context.Context, input chan jsonrpc.Response) <-chan any {
	output := make(chan any)
	go func() {
		defer close(output)
		for {
			select {
			case <-ctx.Done():
				return
			case item, ok := <-input:
				if !ok {
					return
				}
				output <- item
			}
		}
	}()
	return output
}

// forwardTaskStreamAdapter creates a channel adapter from <-chan a2a.Task to <-chan any
func forwardTaskStreamAdapter(ctx context.Context, input <-chan a2a.Task) <-chan any {
	output := make(chan any)

	go func() {
		defer close(output)
		for {
			select {
			case <-ctx.Done():
				return
			case task, ok := <-input:
				if !ok {
					return
				}
				output <- task
			}
		}
	}()

	return output
}

/*
handleRPC acts as the central routing for all a2a RPC methods.
*/
func (srv *A2AServer) handleRPC(ctx fiber.Ctx) error {
	ctx.Set("Content-Type", "application/json")

	var request jsonrpc.Request

	if err := ctx.Bind().Body(&request); err != nil {
		return ctx.Status(
			fiber.StatusBadRequest,
		).JSON(jsonrpc.Response{ // Send structured error
			Message: jsonrpc.Message{
				MessageIdentifier: jsonrpc.MessageIdentifier{ID: nil}, // ID might not be available if body is invalid
				JSONRPC:           "2.0",
			},
			Error: &jsonrpc.Error{
				Code:    errors.ErrInvalidRequest.Code,
				Message: "Invalid request body: " + err.Error(),
			},
		})
	}

	switch request.Method {
	case "tasks/send":
		return srv.handleTaskOperation(ctx, request.ID, func() (any, error) {
			var params a2a.TaskSendParams

			if rpcErr := srv.parseAndUnmarshalParams(request.Params, &params); rpcErr != nil {
				return nil, rpcErr
			}

			return srv.agent.SendTask(ctx.Context(), params)
		})
	case "tasks/sendSubscribe":
		return srv.handleTaskOperation(ctx, request.ID, func() (any, error) {
			var params a2a.TaskSendParams

			if rpcErr := srv.parseAndUnmarshalParams(request.Params, &params); rpcErr != nil {
				return nil, rpcErr
			}

			// Convert send parameters into a task for streaming
			task := a2a.NewTask(srv.agent.Name())
			task.ID = params.ID
			if params.SessionID != "" {
				task.SessionID = params.SessionID
			}
			task.History = append(task.History, params.Message)
			task.Metadata = params.Metadata

			stream, rpcErr := srv.agent.StreamTask(ctx.Context(), task)
			if rpcErr != nil {
				return nil, rpcErr
			}

			var firstResultPayload any
			// Wait for the first response from the stream.
			// This response is a jsonrpc.Response, and we are interested in its Result field.
			firstStreamResponse, ok := <-stream
			if ok {
				firstResultPayload = firstStreamResponse.Result // Extract the actual payload (the task)
			} else {
				// Stream closed before sending the first item.
				log.Warn("tasks/sendSubscribe: stream closed before the first item (initial task data) could be read", "taskID", task.ID)
				firstResultPayload = nil
			}

			adaptedResponseStream := forwardResponseChannelToAnyChannel(ctx.Context(), stream)
			srv.forwardEventsToBroker(ctx.Context(), adaptedResponseStream)

			return firstResultPayload, nil // Return the payload of the first stream message
		})
	case "tasks/get":
		return srv.handleTaskOperation(ctx, request.ID, func() (any, error) {
			var params a2a.TaskQueryParams

			if rpcErr := srv.parseAndUnmarshalParams(request.Params, &params); rpcErr != nil {
				return nil, rpcErr
			}

			return srv.agent.GetTask(ctx.Context(), params.ID, *params.HistoryLength)
		})
	case "tasks/cancel":
		return srv.handleTaskOperation(ctx, request.ID, func() (any, error) {
			var params a2a.TaskIDParams

			if rpcErr := srv.parseAndUnmarshalParams(request.Params, &params); rpcErr != nil {
				return nil, rpcErr
			}
			// CancelTask specifically returns nil result on success, and an error on failure.
			// The handleTaskOperation will correctly wrap this in a JSON-RPC response.
			return nil, srv.agent.CancelTask(ctx.Context(), params.ID)
		})
	case "tasks/resubscribe":
		return srv.handleTaskOperation(ctx, request.ID, func() (any, error) {
			var params a2a.TaskQueryParams

			if rpcErr := srv.parseAndUnmarshalParams(request.Params, &params); rpcErr != nil {
				return nil, rpcErr
			}

			hl := 0
			if params.HistoryLength != nil {
				hl = *params.HistoryLength
			}
			stream, rpcErr := srv.agent.ResubscribeTask(ctx.Context(), params.ID, hl)
			if rpcErr != nil {
				return nil, rpcErr
			}

			var first any
			select {
			case first = <-stream:
			default:
				first = nil
			}

			taskStreamAdapter := forwardTaskStreamAdapter(ctx.Context(), stream)
			srv.forwardEventsToBroker(ctx.Context(), taskStreamAdapter)

			return first, nil
		})
	default:
		return ctx.Status(fiber.StatusBadRequest).JSON(jsonrpc.Response{
			Message: jsonrpc.Message{
				MessageIdentifier: jsonrpc.MessageIdentifier{ID: request.ID},
				JSONRPC:           "2.0",
			},
			Error: &jsonrpc.Error{
				Code:    errors.ErrMethodNotFound.Code,
				Message: errors.ErrMethodNotFound.Message + ": " + request.Method,
			},
		})
	}
}

func (srv *A2AServer) handleTaskOperation(ctx fiber.Ctx, requestID any, op func() (any, error)) error {
	result, errOp := op()

	// First, explicitly check if errOp is an interface holding (*errors.RpcError)(nil).
	// If so, treat it as a non-error (effectively plain nil) for further processing.
	if rpcErr, ok := errOp.(*errors.RpcError); ok && rpcErr == nil {
		// Log an info message for visibility, but this is now handled as a success path.
		log.Info("Operation returned a (nil *errors.RpcError), treating as success.", "requestID", requestID)
		errOp = nil // Normalize it to plain nil, so it passes the next error check.
	}

	// Now, errOp is either plain nil (if originally nil, or normalized from typed nil),
	// or it's a non-nil concrete error.
	if errOp != nil {
		// This block is now only for actual, non-nil errors.
		log.Error("Error processing task operation", "error", errOp, "requestID", requestID)

		respErrorCode := errors.ErrInternal.Code
		// Use the error's message directly. .Error() on RpcError includes "RPC error %d: ".
		respErrorMessage := errOp.Error()

		// If the actual error is an RpcError (and not the typed nil we already handled), use its details.
		if e, ok := errOp.(*errors.RpcError); ok { // No '&& e != nil' needed here, typed nil handled above
			respErrorCode = e.Code
			respErrorMessage = e.Message // Prefer the direct message for clarity in JSON response
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(jsonrpc.Response{
			Message: jsonrpc.Message{
				MessageIdentifier: jsonrpc.MessageIdentifier{ID: requestID},
				JSONRPC:           "2.0",
			},
			Error: &jsonrpc.Error{
				Code:    respErrorCode,
				Message: respErrorMessage,
			},
		})
	}

	// Success cases (errOp is now guaranteed to be plain nil here)
	// If result is nil (and errOp was nil), return JSON-RPC null result
	// This handles cases like successful task cancellation that might return (nil, nil) from the op.
	if result == nil {
		return ctx.Status(fiber.StatusOK).JSON(jsonrpc.Response{
			Message: jsonrpc.Message{
				MessageIdentifier: jsonrpc.MessageIdentifier{ID: requestID},
				JSONRPC:           "2.0",
			},
			Result: nil, // Explicit null result
		})
	}

	// Success with a non-nil result
	return ctx.Status(fiber.StatusOK).JSON(jsonrpc.Response{
		Message: jsonrpc.Message{
			MessageIdentifier: jsonrpc.MessageIdentifier{ID: requestID},
			JSONRPC:           "2.0",
		},
		Result: result,
	})
}
