package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func Aquire(id string) (*mcp.Tool, error) {
	log.Info("aquiring tool", "id", id)

	switch id {
	case "development":
		tool := NewDockerTools()
		log.Info("tool", "name", tool.Name, "description", tool.Description, "inputSchema", tool.InputSchema)
		return tool, nil
	}

	return nil, errors.New("tool not found")
}

func NewOpenAIExecutor(
	ctx context.Context, name, args string,
) (string, error) {
	client, err := client.NewStreamableHttpClient(
		"http://" + name + ":3210",
	)

	if err != nil {
		return "", err
	}

	arguments := map[string]any{}

	err = json.Unmarshal([]byte(args), &arguments)

	if err != nil {
		return "", err
	}

	request := mcp.CallToolRequest{}
	request.Params.Name = name
	request.Params.Arguments = arguments

	result, err := client.CallTool(ctx, request)

	if err != nil {
		return "", err
	}

	resultStr, err := json.Marshal(result)

	if err != nil {
		return "", err
	}

	return string(resultStr), nil
}
