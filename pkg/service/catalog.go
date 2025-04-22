package service

import (
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
	}
}

func (srv *CatalogServer) Run() error {
	srv.app.Get("/.well-known/catalog.json", func(ctx fiber.Ctx) error {
		registry := catalog.NewRegistry()
		agents := registry.GetAgents()

		return ctx.Status(fiber.StatusOK).JSON(agents)
	})

	srv.app.Get("/agent/:id", func(ctx fiber.Ctx) error {
		registry := catalog.NewRegistry()
		agent := registry.GetAgent(ctx.Params("id"))

		if agent == nil {
			return ctx.Status(fiber.StatusNotFound).SendString("Agent not found")
		}

		return ctx.Status(fiber.StatusOK).JSON(agent)
	})

	srv.app.Post("/agent", func(ctx fiber.Ctx) error {
		registry := catalog.NewRegistry()
		var agentCard types.AgentCard

		if err := ctx.Bind().Body(&agentCard); err != nil {
			return ctx.Status(fiber.StatusBadRequest).SendString("Invalid agent card")
		}

		registry.AddAgent(agentCard)

		return ctx.Status(fiber.StatusCreated).JSON(agentCard)
	})

	return srv.app.Listen(":3210")
}
