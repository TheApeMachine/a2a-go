package a2a

// Lightweight GitHub tools using stdlib HTTP.  These are best‑effort: if the
// token/repo is missing or network unavailable the tool returns an error
// message to the caller without crashing the host agent.

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strconv"
    "strings"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"

    gh "github.com/theapemachine/a2a-go/pkg/tools/github"
)

func registerGitHubTools(srv *server.MCPServer) {
    srv.AddTool(buildGHListPRsTool(), handleGHListPRs)
    srv.AddTool(buildGHCreateIssueTool(), handleGHCreateIssue)
    srv.AddTool(buildGHCommentPRTool(), handleGHCommentPR)
}

// ---------------------------------------------------------------------
// Tool specs
// ---------------------------------------------------------------------

func buildGHListPRsTool() mcp.Tool {
    return mcp.NewTool(
        "github_list_prs",
        mcp.WithDescription("Lists pull requests for a repository"),
        mcp.WithString("repository", mcp.Description("owner/repo"), mcp.Required()),
        mcp.WithString("state", mcp.Description("open, closed, all (default open)")),
        mcp.WithString("token", mcp.Description("GitHub PAT (optional, falls back to env GITHUB_TOKEN)")),
    )
}

func buildGHCreateIssueTool() mcp.Tool {
    return mcp.NewTool(
        "github_create_issue",
        mcp.WithDescription("Creates an issue in a repository"),
        mcp.WithString("repository", mcp.Description("owner/repo"), mcp.Required()),
        mcp.WithString("title", mcp.Description("Issue title"), mcp.Required()),
        mcp.WithString("body", mcp.Description("Markdown body")),
        mcp.WithString("token", mcp.Description("GitHub PAT (optional)")),
    )
}

func buildGHCommentPRTool() mcp.Tool {
    return mcp.NewTool(
        "github_comment_pr",
        mcp.WithDescription("Adds a comment to a pull request"),
        mcp.WithString("repository", mcp.Description("owner/repo"), mcp.Required()),
        mcp.WithString("pr_number", mcp.Description("Pull request number"), mcp.Required()),
        mcp.WithString("body", mcp.Description("Comment text"), mcp.Required()),
        mcp.WithString("token", mcp.Description("GitHub PAT (optional)")),
    )
}

// ---------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------

func tokenFromArgs(args map[string]interface{}) string {
    if t, ok := args["token"].(string); ok && t != "" {
        return t
    }
    return os.Getenv("GITHUB_TOKEN")
}

func handleGHListPRs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.Params.Arguments
    repo, _ := args["repository"].(string)
    if repo == "" {
        return nil, fmt.Errorf("repository parameter required")
    }
    state, _ := args["state"].(string)
    cli := gh.New(tokenFromArgs(args))
    prs, err := cli.ListPRs(ctx, repo, state)
    if err != nil {
        return nil, err
    }
    b, _ := json.Marshal(prs)
    return mcp.NewToolResultText(string(b)), nil
}

func handleGHCreateIssue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.Params.Arguments
    repo, _ := args["repository"].(string)
    title, _ := args["title"].(string)
    body, _ := args["body"].(string)
    if repo == "" || strings.TrimSpace(title) == "" {
        return nil, fmt.Errorf("repository and title required")
    }
    cli := gh.New(tokenFromArgs(args))
    issue, err := cli.CreateIssue(ctx, repo, title, body)
    if err != nil {
        return nil, err
    }
    b, _ := json.Marshal(issue)
    return mcp.NewToolResultText(string(b)), nil
}

func handleGHCommentPR(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.Params.Arguments
    repo, _ := args["repository"].(string)
    body, _ := args["body"].(string)
    prStr, _ := args["pr_number"].(string)
    n, _ := strconv.Atoi(prStr)
    if repo == "" || n == 0 || strings.TrimSpace(body) == "" {
        return nil, fmt.Errorf("repository, pr_number, body required")
    }
    cli := gh.New(tokenFromArgs(args))
    comment, err := cli.CommentPR(ctx, repo, n, body)
    if err != nil {
        return nil, err
    }
    b, _ := json.Marshal(comment)
    return mcp.NewToolResultText(string(b)), nil
}
