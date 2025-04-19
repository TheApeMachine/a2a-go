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
    // Standard docker_exec tool with enhanced options
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
        mcp.WithString("network",
            mcp.Description("Network configuration (optional, default 'none')"),
        ),
        mcp.WithString("memory",
            mcp.Description("Memory limit (optional, default '256m')"),
        ),
        mcp.WithObject("env",
            mcp.Description("Environment variables as key-value pairs (optional)"),
        ),
        mcp.WithNumber("max_retries",
            mcp.Description("Maximum number of retries on temporary failures (optional, default 3)"),
        ),
    )
    srv.AddTool(tool, handleDockerExec)

    // Specialized docker_exec_with_volumes tool for file access scenarios
    volumeTool := mcp.NewTool(
        "docker_exec_with_volumes",
        mcp.WithDescription("Runs a shell command inside a Docker container with volume mounts. Useful for file access scenarios."),
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
        mcp.WithString("network",
            mcp.Description("Network configuration (optional, default 'none')"),
        ),
        mcp.WithString("memory",
            mcp.Description("Memory limit (optional, default '256m')"),
        ),
        mcp.WithObject("env",
            mcp.Description("Environment variables as key-value pairs (optional)"),
        ),
        mcp.WithObject("volumes",
            mcp.Description("Volume mounts as key-value pairs where keys are host paths and values are container paths"),
            mcp.Required(),
        ),
        mcp.WithNumber("max_retries",
            mcp.Description("Maximum number of retries on temporary failures (optional, default 3)"),
        ),
    )
    srv.AddTool(volumeTool, handleDockerExecWithVolumes)
}

func handleDockerExec(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.Params.Arguments

    // Extract basic parameters
    image, _ := args["image"].(string)
    cmdStr, _ := args["cmd"].(string)
    if strings.TrimSpace(cmdStr) == "" {
        return nil, fmt.Errorf("cmd parameter is required")
    }
    cmd := []string{"sh", "-c", cmdStr}

    // Set timeout
    var timeout time.Duration
    if t, ok := args["timeout"].(float64); ok && t > 0 {
        timeout = time.Duration(t * float64(time.Second))
    }

    // Create ExecOptions with enhanced parameters
    opts := dock.DefaultExecOptions()

    // Set network if provided
    if network, ok := args["network"].(string); ok && network != "" {
        opts.Network = network
    }

    // Set memory limit if provided
    if memory, ok := args["memory"].(string); ok && memory != "" {
        opts.Memory = memory
    }

    // Set environment variables if provided
    if envObj, ok := args["env"].(map[string]interface{}); ok {
        for k, v := range envObj {
            if strVal, ok := v.(string); ok {
                opts.Env[k] = strVal
            }
        }
    }

    // Set max retries if provided
    if maxRetries, ok := args["max_retries"].(float64); ok && maxRetries >= 0 {
        opts.MaxRetries = int(maxRetries)
    }

    // Execute with the enhanced options
    res, err := dock.Exec(ctx, image, cmd, timeout, opts)
    if err != nil {
        return nil, err
    }

    b, _ := json.Marshal(res)
    return mcp.NewToolResultText(string(b)), nil
}

func handleDockerExecWithVolumes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.Params.Arguments

    // Extract basic parameters
    image, _ := args["image"].(string)
    cmdStr, _ := args["cmd"].(string)
    if strings.TrimSpace(cmdStr) == "" {
        return nil, fmt.Errorf("cmd parameter is required")
    }
    cmd := []string{"sh", "-c", cmdStr}

    // Set timeout
    var timeout time.Duration
    if t, ok := args["timeout"].(float64); ok && t > 0 {
        timeout = time.Duration(t * float64(time.Second))
    }

    // Create ExecOptions with enhanced parameters
    opts := dock.DefaultExecOptions()

    // Set network if provided
    if network, ok := args["network"].(string); ok && network != "" {
        opts.Network = network
    }

    // Set memory limit if provided
    if memory, ok := args["memory"].(string); ok && memory != "" {
        opts.Memory = memory
    }

    // Set environment variables if provided
    if envObj, ok := args["env"].(map[string]interface{}); ok {
        for k, v := range envObj {
            if strVal, ok := v.(string); ok {
                opts.Env[k] = strVal
            }
        }
    }

    // Set volume mounts (required for this specialized tool)
    volumesObj, ok := args["volumes"].(map[string]interface{})
    if !ok || len(volumesObj) == 0 {
        return nil, fmt.Errorf("volumes parameter is required and must contain at least one volume mapping")
    }

    for hostPath, containerPathObj := range volumesObj {
        if containerPath, ok := containerPathObj.(string); ok {
            opts.Volumes[hostPath] = containerPath
        }
    }

    // Set max retries if provided
    if maxRetries, ok := args["max_retries"].(float64); ok && maxRetries >= 0 {
        opts.MaxRetries = int(maxRetries)
    }

    // Execute with the enhanced options
    res, err := dock.Exec(ctx, image, cmd, timeout, opts)
    if err != nil {
        return nil, err
    }

    b, _ := json.Marshal(res)
    return mcp.NewToolResultText(string(b)), nil
}
