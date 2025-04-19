package tools

// Docker sandbox tool: run a shell command inside a temporary container and
// return stdout, stderr, exit code, and duration.  Uses the lightweight helper
// in pkg/tools/docker so that external code can be swapped easily.

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "time"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"

    dock "github.com/theapemachine/a2a-go/pkg/tools/docker"
)

func registerDockerTools(srv *server.MCPServer) {
    tool := mcp.NewTool(
        "docker_exec",
        mcp.WithDescription("Runs a shell command inside a temporary Docker container (default image busybox:latest). Returns JSON with stdout, stderr, exit_code, duration_ms."),
        mcp.WithString("image",
            mcp.Description("Container image to run (optional, default busybox:latest)"),
        ),
        mcp.WithString("cmd",
            mcp.Description("Shell command to execute inside the container"),
            mcp.Required(),
        ),
        mcp.WithNumber("timeout",
            mcp.Description("Max execution time in seconds (optional, default 60)"),
        ),
    )

    srv.AddTool(tool, handleDockerExec)
}

func handleDockerExec(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.Params.Arguments

    image, _ := args["image"].(string)

    cmdStr, _ := args["cmd"].(string)
    if strings.TrimSpace(cmdStr) == "" {
        return nil, fmt.Errorf("cmd parameter is required")
    }
    cmd := []string{"sh", "-c", cmdStr}

    var timeout time.Duration
    if t, ok := args["timeout"].(float64); ok && t > 0 {
        timeout = time.Duration(t*float64(time.Second))
    }

    res, err := dock.Exec(ctx, image, cmd, timeout)
    if err != nil {
        return nil, err
    }

    b, _ := json.Marshal(res)
    return mcp.NewToolResultText(string(b)), nil
}
