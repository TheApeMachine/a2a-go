package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func Aquire(id string) (*mcp.Tool, error) {
	switch id {
	case "development":
		return NewDockerTools(), nil
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
