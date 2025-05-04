package service

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
)

/*
A2AServer is safe for concurrent use by default because
RPCServer & SSEBroker are.
*/
type A2AServer struct {
	app   *fiber.App
	agent *ai.Agent
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
		agent: agent,
	}
}

func (srv *A2AServer) Start() error {
	srv.app.Use(logger.New(), healthcheck.NewHealthChecker())
	srv.app.Get("/", srv.handleRoot)
	srv.app.Get("/.well-known/agent.json", srv.handleAgentCard)
	srv.app.Post("/rpc", srv.handleRPC)
	return srv.app.Listen(":3210", fiber.ListenConfig{DisableStartupMessage: true})
}

func (srv *A2AServer) handleRoot(ctx fiber.Ctx) error {
	return ctx.SendString("OK")
}

func (srv *A2AServer) handleAgentCard(ctx fiber.Ctx) error {
	return ctx.JSON(srv.agent.Card())
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
		).SendString("Invalid request body")
	}

	switch request.Method {
	case "tasks/send":
		return srv.handleTaskOperation(ctx, func() (any, error) {
			var params a2a.TaskSendParams

			if err := json.Unmarshal(request.Params.([]byte), &params); err != nil {
				return nil, err
			}

			return srv.agent.SendTask(ctx.Context(), params)
		})
	case "tasks/get":
		return srv.handleTaskOperation(ctx, func() (any, error) {
			var params a2a.TaskQueryParams
			return srv.agent.GetTask(ctx.Context(), params.ID, *params.HistoryLength)
		})
	case "tasks/cancel":
		return srv.handleTaskOperation(ctx, func() (any, error) {
			var params a2a.TaskIDParams
			return nil, srv.agent.CancelTask(ctx.Context(), params.ID)
		})
	default:
		return ctx.Status(fiber.StatusBadRequest).SendString("Unsupported method")
	}
}

func (srv *A2AServer) handleTaskOperation(ctx fiber.Ctx, op func() (interface{}, error)) error {
	result, err := op()

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	if result == nil {
		return ctx.SendString("Task cancelled successfully")
	}

	return ctx.JSON(result)
}

func (srv *A2AServer) parseParams(request *jsonrpc.Request, params interface{}) error {
	paramsBytes, ok := request.Params.([]byte)

	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request parameters")
	}

	return json.Unmarshal(paramsBytes, params)
}
