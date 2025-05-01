package service

import (
	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v3"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/types"
)

type CatalogServer struct {
	app           *fiber.App
	agentRegistry *catalog.Registry
}

func NewCatalogServer() *CatalogServer {
	return &CatalogServer{
		app: fiber.New(fiber.Config{
			AppName:           "A2A Catalog",
			ServerHeader:      "A2A-Catalog-Server",
			StreamRequestBody: true,
		}),
		agentRegistry: catalog.NewRegistry(),
	}
}

func (srv *CatalogServer) Run() error {
	srv.app.Get("/", func(ctx fiber.Ctx) error {
		log.Info("Health check request received", "ip", ctx.IP())
		return ctx.Status(fiber.StatusOK).SendString("A2A Catalog Service is running")
	})

	srv.app.Get("/.well-known/catalog.json", func(ctx fiber.Ctx) error {
		return ctx.Status(fiber.StatusOK).JSON(srv.agentRegistry.GetAgents())
	})

	// Get a specific agent from the catalog
	srv.app.Get("/agent/:id", func(ctx fiber.Ctx) error {
		agent := srv.agentRegistry.GetAgent(ctx.Params("id"))

		if agent == nil {
			return ctx.Status(fiber.StatusNotFound).SendString("Agent not found")
		}

		return ctx.Status(fiber.StatusOK).JSON(agent)
	})

	srv.app.Post("/agent", func(ctx fiber.Ctx) error {
		var agentCard types.AgentCard

		if err := ctx.Bind().Body(&agentCard); err != nil {
			return ctx.Status(fiber.StatusBadRequest).SendString("Invalid agent card: " + err.Error())
		}

		if agentCard.Name == "" {
			return ctx.Status(fiber.StatusBadRequest).SendString("Agent name is required")
		}

		if agentCard.URL == "" {
			return ctx.Status(fiber.StatusBadRequest).SendString("Agent URL is required")
		}

		srv.agentRegistry.AddAgent(agentCard)
		return ctx.Status(fiber.StatusCreated).JSON(agentCard)
	})

	return srv.app.Listen(":3210")
}
