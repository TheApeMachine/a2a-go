package tools

import (
    "context"
    "encoding/json"

    "github.com/google/uuid"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/theapemachine/a2a-go/pkg/a2a"
)

// NewDelegateTool returns a tool definition for delegating a task to another agent.
func NewDelegateTool() *mcp.Tool {
    tool := mcp.NewTool(
        "delegate_task",
        mcp.WithDescription("Delegate a task to another agent"),
        mcp.WithString("agent", mcp.Description("Agent URL"), mcp.Required()),
        mcp.WithString("message", mcp.Description("Task message"), mcp.Required()),
    )
    return &tool
}

// delegateParams defines the input for the delegate tool.
type delegateParams struct {
    Agent   string `json:"agent"`
    Message string `json:"message"`
}

func executeDelegate(ctx context.Context, raw string) (string, error) {
    var p delegateParams
    if err := json.Unmarshal([]byte(raw), &p); err != nil {
        return "", err
    }
    client := a2a.NewClient(p.Agent)
    resp, err := client.SendTask(a2a.TaskSendParams{
        ID:      uuid.NewString(),
        Message: a2a.NewTextMessage("user", p.Message),
    })
    if err != nil {
        return "", err
    }
    data, err := json.Marshal(resp.Result)
    if err != nil {
        return "", err
    }
    return string(data), nil
}
