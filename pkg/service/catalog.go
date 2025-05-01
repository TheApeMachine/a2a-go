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
			ServerHeader:      "A2A-Catalog-TheApeMachine",
			StreamRequestBody: true,
		}),
		agentRegistry: catalog.NewRegistry(),
	}
}

func (srv *CatalogServer) Run() error {
	// Setup a logger middleware
	srv.app.Use(func(c fiber.Ctx) error {
		log.Info("Catalog API request", "method", c.Method(), "path", c.Path(), "ip", c.IP())
		return c.Next()
	})

	// Basic health check endpoint
	srv.app.Get("/", func(ctx fiber.Ctx) error {
		log.Info("Health check request received", "ip", ctx.IP())
		return ctx.Status(fiber.StatusOK).SendString("A2A Catalog Service is running")
	})

	// Get all agents from the catalog
	srv.app.Get("/.well-known/catalog.json", func(ctx fiber.Ctx) error {
		log.Info("Getting agents from catalog", "ip", ctx.IP())
		agents := srv.agentRegistry.GetAgents()
		log.Info("Retrieved agents from catalog", "count", len(agents), "ip", ctx.IP())
		return ctx.Status(fiber.StatusOK).JSON(agents)
	})

	// Get a specific agent from the catalog
	srv.app.Get("/agent/:id", func(ctx fiber.Ctx) error {
		id := ctx.Params("id")
		log.Info("Getting agent from catalog", "id", id, "ip", ctx.IP())
		agent := srv.agentRegistry.GetAgent(id)

		if agent == nil {
			log.Warn("Agent not found", "id", id, "ip", ctx.IP())
			return ctx.Status(fiber.StatusNotFound).SendString("Agent not found")
		}

		return ctx.Status(fiber.StatusOK).JSON(agent)
	})

	// Register an agent with the catalog
	srv.app.Post("/agent", func(ctx fiber.Ctx) error {
		log.Info("Received agent registration request", "ip", ctx.IP())
		var agentCard types.AgentCard

		if err := ctx.Bind().Body(&agentCard); err != nil {
			log.Error("Invalid agent card", "error", err, "ip", ctx.IP())
			return ctx.Status(fiber.StatusBadRequest).SendString("Invalid agent card: " + err.Error())
		}

		// Validate the agent card
		if agentCard.Name == "" {
			log.Error("Agent name is required", "ip", ctx.IP())
			return ctx.Status(fiber.StatusBadRequest).SendString("Agent name is required")
		}

		if agentCard.URL == "" {
			log.Error("Agent URL is required", "ip", ctx.IP())
			return ctx.Status(fiber.StatusBadRequest).SendString("Agent URL is required")
		}

		log.Info("Registering agent with catalog", "name", agentCard.Name, "url", agentCard.URL, "ip", ctx.IP())
		srv.agentRegistry.AddAgent(agentCard)
		log.Info("Agent registered successfully", "name", agentCard.Name, "ip", ctx.IP())

		return ctx.Status(fiber.StatusCreated).JSON(agentCard)
	})

	log.Info("Starting catalog server on port 3210")
	return srv.app.Listen(":3210")
}
