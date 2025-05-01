package service

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/prompts"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/roots"
	"github.com/theapemachine/a2a-go/pkg/sampling"
	"github.com/theapemachine/a2a-go/pkg/service/sse"
	"github.com/theapemachine/a2a-go/pkg/types"
)

/*
A2AServer is safe for concurrent use by default because
RPCServer & SSEBroker are.
*/
type A2AServer struct {
	app             *fiber.App
	agentRegistry   *catalog.Registry
	Agent           types.IdentifiableTaskManager
	PromptManager   prompts.PromptManager
	SamplingManager sampling.Manager
	RootManager     *roots.Manager
	rpc             *jsonrpc.RPCServer
	broker          *sse.SSEBroker
	mcp             *sse.MCPBroker
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
func NewA2AServer(agent types.IdentifiableTaskManager) *A2AServer {
	srv := &A2AServer{
		app: fiber.New(fiber.Config{
			AppName:           "A2A Server",
			ServerHeader:      "A2A-Server",
			StreamRequestBody: true,
		}),
		agentRegistry:   catalog.NewRegistry(),
		Agent:           agent,
		PromptManager:   prompts.NewDefaultManager(),
		SamplingManager: chooseSamplingManager(),
		RootManager:     roots.NewManager(),
		rpc:             jsonrpc.NewRPCServer(agent),
		broker:          sse.NewSSEBroker(),
		mcp:             sse.NewMCPBroker(),
	}

	if agentWithSSE, ok := agent.(interface{ SetSSEPublisher(types.SSEPublisher) }); ok {
		agentWithSSE.SetSSEPublisher(srv.broker)
	}

	srv.agentRegistry.AddAgent(*agent.Card())
	return srv
}

func (srv *A2AServer) Start() error {
	srv.app.Use(logger.New())

	srv.app.Get("/", func(ctx fiber.Ctx) error {
		return ctx.Status(fiber.StatusOK).SendString("OK")
	})

	srv.app.Get("/.well-known/agent.json", func(ctx fiber.Ctx) error {
		return ctx.Status(fiber.StatusOK).JSON(srv.Agent.Card())
	})

	srv.app.Post("/rpc", func(ctx fiber.Ctx) error {
		w := &responseWriter{ctx: ctx}

		r := &http.Request{
			Method: ctx.Method(),
			URL:    &url.URL{Path: ctx.Path()},
			Header: make(http.Header),
			Body:   io.NopCloser(bytes.NewReader(ctx.Body())),
		}

		ctx.Request().Header.VisitAll(func(key, value []byte) {
			r.Header.Add(string(key), string(value))
		})

		srv.rpc.ServeHTTP(w, r)
		return nil
	})

	// Add SSE event stream endpoint for tasks (according to A2A spec)
	srv.app.Get("/events/:taskID", func(ctx fiber.Ctx) error {
		taskID := ctx.Params("taskID")
		if taskID == "" {
			return ctx.Status(fiber.StatusBadRequest).SendString("Task ID is required")
		}

		w := &responseWriter{ctx: ctx}

		r := &http.Request{
			Method: ctx.Method(),
			URL:    &url.URL{Path: ctx.Path()},
			Header: make(http.Header),
		}

		ctx.Request().Header.VisitAll(func(key, value []byte) {
			r.Header.Add(string(key), string(value))
		})

		taskBroker := srv.broker.GetOrCreateTaskBroker(taskID)

		if sseTaskBroker, ok := taskBroker.(*sse.SSEBroker); ok {
			sseTaskBroker.Subscribe(w, r)
		} else {
			return ctx.Status(fiber.StatusInternalServerError).SendString("Failed to create or access task broker")
		}

		return nil
	})

	return srv.app.Listen(":3210", fiber.ListenConfig{
		DisableStartupMessage: true,
	})
}

type responseWriter struct {
	ctx fiber.Ctx
}

func (w *responseWriter) Header() http.Header {
	return http.Header{}
}

func (w *responseWriter) Write(data []byte) (int, error) {
	return w.ctx.Write(data)
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.ctx.Status(statusCode)
}
