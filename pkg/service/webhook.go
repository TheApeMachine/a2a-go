package service

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/catalog"
)

/*
A2AServer is safe for concurrent use by default because
RPCServer & SSEBroker are.
*/
type WebhookServer struct {
	app        *fiber.App
	catalogURL string
	agentURL   string
}

/*
NewWebhookServer constructs a server with the supplied Agent.
*/
func NewWebhookServer() *WebhookServer {
	v := viper.GetViper()

	return &WebhookServer{
		app: fiber.New(fiber.Config{
			AppName:           "A2A-Webhook-Server",
			ServerHeader:      "A2A-Webhook-Server",
			StreamRequestBody: true,
		}),
		catalogURL: v.GetString("endpoints.catalog"),
	}
}

func (srv *WebhookServer) Start() error {
	srv.app.Use(logger.New(logger.Config{
		// Skip logging for the /events endpoint to reduce noise
		Next: func(c fiber.Ctx) bool {
			return c.Path() == "/events"
		},
	}), healthcheck.New())

	srv.app.Get("/", srv.handleRoot)
	srv.app.Post("/webhook", srv.handleWebhook)

	return srv.app.Listen(":3210", fiber.ListenConfig{DisableStartupMessage: true})
}

func (srv *WebhookServer) handleRoot(ctx fiber.Ctx) error {
	return ctx.SendString("OK")
}

func (srv *WebhookServer) handleWebhook(ctx fiber.Ctx) error {
	catalogClient := catalog.NewCatalogClient(
		srv.catalogURL,
	)

	agents, err := catalogClient.GetAgents()

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	var agent *a2a.AgentCard

	for _, a := range agents {
		if a.Name == "User Interface Agent" {
			agent = &a
			break
		}
	}

	if agent == nil {
		return ctx.Status(fiber.StatusNotFound).SendString("User Interface Agent not found")
	}

	agentClient := a2a.NewClient(agent.URL)

	message := a2a.NewTextMessage("user", string(ctx.Body()))

	origin := ctx.Get(fiber.HeaderOrigin)
	if origin == "" {
		origin = ctx.IP()
	}

	if origin == "" {
		origin = "webhook"
	}

	message.Metadata = map[string]any{"origin": origin}

	task, err := agentClient.SendTask(a2a.TaskSendParams{
		ID:        uuid.New().String(),
		SessionID: uuid.New().String(),
		Message:   *message,
	})

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	taskJSON, err := json.Marshal(task)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return ctx.Status(fiber.StatusCreated).Send(taskJSON)
}
