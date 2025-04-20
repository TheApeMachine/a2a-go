package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/types"
)

type A2AClient struct{}

func registerA2ATools(srv *server.MCPServer) {
	srv.AddTool(mcp.NewTool(
		"discover_agents",
		mcp.WithDescription("Discover other agents you can communicate with."),
	), handleA2ADiscover)

	srv.AddTool(mcp.NewTool(
		"get_agent_card",
		mcp.WithDescription("Inspect an agent's card to discover their capabilities."),
		mcp.WithString(
			"agent_name",
			mcp.Description("The name of the agent you want to inspect."),
			mcp.Required(),
		),
	), handleA2AAgentCard)

	srv.AddTool(mcp.NewTool(
		"send_task",
		mcp.WithDescription("Send a task to an agent"),
		mcp.WithString(
			"agent_name",
			mcp.Description("The name of the agent you want to send a task to."),
			mcp.Required(),
		),
		mcp.WithArray(
			"messages",
			mcp.Description("The messages to attach to the task."),
			mcp.Required(),
		),
	), handleA2AAgentCard)
}

func handleA2ADiscover(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	v := viper.GetViper()
	result, err := client.Get(v.GetString("server.defaultCatalogPath"))

	if err != nil {
		return mcp.NewToolResultError(err.Error()), err
	}

	if result.StatusCode() != 200 {
		err := errors.New("unable to retrieve catalog")
		return mcp.NewToolResultError(err.Error()), err
	}

	return mcp.NewToolResultText(
		string(result.Body()),
	), nil
}

func handleA2AAgentCard(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	card, err := getAgentCard(req)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), err
	}

	var (
		buf []byte
	)

	if buf, err = json.Marshal(card); err != nil {
		return mcp.NewToolResultError(err.Error()), err
	}

	return mcp.NewToolResultText(
		string(buf),
	), nil
}

func handleA2ASendTask(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	card, err := getAgentCard(req)

	if err != nil {
		return mcp.NewToolResultError(err.Error()), err
	}

	messages, ok := req.Params.Arguments["agent_name"].([]string)

	if !ok {
		err := errors.New("unable to retrieve agent")
		return mcp.NewToolResultError(err.Error()), err
	}

	parts := make([]types.Part, len(messages))

	for _, message := range messages {
		parts = append(parts, types.Part{
			Type: types.PartTypeText,
			Text: message,
		})
	}

	params := types.TaskSendParams{
		ID: fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Message: types.Message{
			Role:  "user",
			Parts: parts,
		},
	}

	client := jsonrpc.RPCClient{
		Endpoint: card.URL,
		HTTP:     &http.Client{},
	}

	result := &jsonrpc.RPCResponse{}

	client.Call(ctx, "tasks/send", params, result)

	return mcp.NewToolResultText(
		string(result.Result.([]byte)),
	), nil
}

func getAgentCard(req mcp.CallToolRequest) (types.AgentCard, error) {
	registry := catalog.NewRegistry()

	name, ok := req.Params.Arguments["agent_name"].(string)

	if !ok {
		err := errors.New("unable to retrieve agent")
		return types.AgentCard{}, err
	}

	agent := registry.GetAgent(name)

	if agent == nil {
		err := errors.New("agent not found")
		return types.AgentCard{}, err
	}

	return agent.Card(), nil
}
