package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/theapemachine/a2a-go/pkg/tools/docker"
)

func NewDockerTools() mcp.Tool {
	tool := mcp.NewTool(
		"docker_exec",
		mcp.WithDescription("A fully featured Debian terminal, useful for when you require access to a computer."),
		mcp.WithString("cmd",
			mcp.Description("Shell command to execute inside the container"),
			mcp.Required(),
		),
	)

	return tool
}

func registerDockerTools(srv *server.MCPServer) {
	tool := mcp.NewTool(
		"docker_exec",
		mcp.WithDescription("A fully featured Debian terminal, useful for when you require access to a computer."),
		mcp.WithString("cmd",
			mcp.Description("Shell command to execute inside the container"),
			mcp.Required(),
		),
	)

	srv.AddTool(tool, handleDockerExec)
}

func handleDockerExec(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	var (
		args   = req.Params.Arguments
		cmdStr string
		err    error
		ok     bool
	)

	if cmdStr, ok = args["cmd"].(string); !ok {
		err = errors.New("unable to convert cmd to string")
		return mcp.NewToolResultError(err.Error()), err
	}

	if strings.TrimSpace(cmdStr) == "" {
		return nil, errors.New("cmd parameter is required")
	}

	env, err := docker.NewEnvironment()
	if err = env.BuildImage(ctx, "docker-environment"); err != nil {
		return mcp.NewToolResultError(err.Error()), err
	}

	res, err := env.Exec(ctx, cmdStr)

	if err != nil {
		return nil, err
	}

	b, _ := json.Marshal(res)
	return mcp.NewToolResultText(string(b)), nil
}
