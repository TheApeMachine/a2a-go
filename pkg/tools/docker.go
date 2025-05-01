package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/theapemachine/a2a-go/pkg/registry"
	dkr "github.com/theapemachine/a2a-go/pkg/tools/docker"
)

// init registers the Docker/terminal tool with the central registry.
func init() {
	dockerTool := NewDockerTools()

	toolDef := registry.ToolDefinition{
		SkillID:     "development",
		ToolName:    dockerTool.Name,
		Description: dockerTool.Description,
		Schema:      dockerTool.InputSchema,
		Executor:    executeDockerTool,
	}

	registry.RegisterTool(toolDef)
	log.Info("Registered built-in tool", "skillID", toolDef.SkillID, "toolName", toolDef.ToolName)
}

func NewDockerTools() mcp.Tool {
	tool := mcp.NewTool(
		"terminal",
		mcp.WithDescription("A fully featured Debian terminal, useful for when you require access to a computer."),
		mcp.WithString("cmd",
			mcp.Description("Shell command to execute inside the container"),
			mcp.Required(),
		),
	)

	return tool
}

func RegisterDockerTools(srv *server.MCPServer) {
	tool := mcp.NewTool(
		"terminal",
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

// executeDockerTool implements the registry.ToolExecutorFunc signature
// for the Docker terminal tool.
func executeDockerTool(ctx context.Context, args map[string]any) (string, error) {
	log.Info("Executing docker tool (terminal)", "args", args)
	cmdStr, ok := args["cmd"].(string)
	if !ok || strings.TrimSpace(cmdStr) == "" {
		return "", errors.New("invalid or missing 'cmd' argument")
	}

	// TODO: Make container name configurable
	containerName := "a2a-go-dev-env"

	env, err := dkr.NewEnvironment()
	if err != nil {
		log.Error("Failed to create docker environment", "error", err)
		return fmt.Sprintf("Failed to create tool environment: %v", err), nil
	}

	res, err := env.Exec(ctx, cmdStr, containerName)
	if err != nil {
		log.Error("Docker exec failed", "command", cmdStr, "error", err)
		output := res.Stdout.String() + "\n" + res.Stderr.String()
		return fmt.Sprintf("Execution failed: %v\nOutput:\n%s", err, output), nil
	}

	output := strings.TrimSpace(res.Stdout.String())
	stderr := strings.TrimSpace(res.Stderr.String())
	if stderr != "" {
		output += "\nstderr:\n" + stderr
	}

	log.Info("Docker tool execution successful")
	return output, nil
}
