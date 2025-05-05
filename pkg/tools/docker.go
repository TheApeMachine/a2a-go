package tools

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	dkr "github.com/theapemachine/a2a-go/pkg/tools/docker"
)

type DockerTool struct {
	tool *mcp.Tool
}

func NewDockerTool() *mcp.Tool {
	tool := mcp.NewTool(
		"terminal",
		mcp.WithDescription("A fully featured Debian terminal, useful for when you require access to a computer."),
		mcp.WithString("cmd",
			mcp.Description("Shell command to execute inside the container"),
			mcp.Required(),
		),
	)

	return &tool
}

func (dt *DockerTool) RegisterDockerTools(srv *server.MCPServer) {
	srv.AddTool(*dt.tool, dt.handle)
}

func (dt *DockerTool) handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("terminal executing")

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

	env, err := dkr.NewEnvironment()

	if err != nil {
		return nil, err
	}

	res, err := env.Exec(ctx, cmdStr, "a2a-go")

	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(
		strings.TrimSpace(strings.Join([]string{
			res.Stdout.String(),
			res.Stderr.String(),
		}, "\n")),
	), nil
}
