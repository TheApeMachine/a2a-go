package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/go-github/v60/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/config"
	"golang.org/x/oauth2"
)

// AzureGetGitHubFileContentTool provides functionality to retrieve the content of a file from GitHub.
type AzureGetGitHubFileContentTool struct {
	handle mcp.Tool
	// No Azure specific client needed here, but config might be useful for general context or future integrations
	// azureConfig AzureDevOpsConfig // Retaining for consistency if other Azure tools need it
}

// NewAzureGetGitHubFileContentTool creates a new tool instance for retrieving GitHub file content.
func NewAzureGetGitHubFileContentTool() core.Tool { // Removed unused conn and azureConfig params
	tool := &AzureGetGitHubFileContentTool{
		// azureConfig: azureConfig,
	}

	tool.handle = mcp.NewTool(
		"azure_get_github_file_content",
		mcp.WithDescription("Retrieves the content of a file from a specified GitHub repository."),
		mcp.WithString("github_owner", mcp.Required(), mcp.Description("The owner of the GitHub repository (organization or user).")),
		mcp.WithString("github_repo", mcp.Required(), mcp.Description("The name of the GitHub repository.")),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("The path to the file within the repository.")),
		mcp.WithString("github_ref", mcp.Description("Optional. The name of the commit/branch/tag. Default: the repository's default branch.")),
		mcp.WithString("format", mcp.Description("Optional. Output format: 'raw' (default, decoded content) or 'base64' (original encoded content from GitHub API).")),
	)
	return tool
}

// Handle returns the MCP tool handle.
func (tool *AzureGetGitHubFileContentTool) Handle() mcp.Tool {
	return tool.handle
}

// Handler is the main execution function for the get GitHub file content tool.
func (tool *AzureGetGitHubFileContentTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appCfg := config.Load()
	if appCfg == nil {
		return mcp.NewToolResultError("failed to load application configuration"), nil
	}

	if appCfg.GitHub.PersonalAccessToken == "" {
		return mcp.NewToolResultError("GitHub Personal Access Token (GITHUB_PAT) not configured."), nil
	}

	owner, err := GetStringArg(request, "github_owner")
	if err != nil || owner == "" {
		return mcp.NewToolResultError("Missing required parameter: github_owner"), nil
	}

	repo, err := GetStringArg(request, "github_repo")
	if err != nil || repo == "" {
		return mcp.NewToolResultError("Missing required parameter: github_repo"), nil
	}

	filePath, err := GetStringArg(request, "file_path")
	if err != nil || filePath == "" {
		return mcp.NewToolResultError("Missing required parameter: file_path"), nil
	}

	ref, _ := GetStringArg(request, "github_ref") // Optional
	outputFormat, _ := GetStringArg(request, "format")
	if outputFormat == "" {
		outputFormat = "raw" // Default to raw, decoded content
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: appCfg.GitHub.PersonalAccessToken})
	tc := oauth2.NewClient(ctx, ts)
	ghClient := github.NewClient(tc)

	opts := &github.RepositoryContentGetOptions{}
	if ref != "" {
		opts.Ref = ref
	}

	fileContent, _, _, err := ghClient.Repositories.GetContents(ctx, owner, repo, filePath, opts)
	if err != nil {
		if ghErr, ok := err.(*github.ErrorResponse); ok && ghErr.Response.StatusCode == 404 {
			return mcp.NewToolResultError(fmt.Sprintf("File not found in repository %s/%s at path '%s' (ref: %s). Error: %v", owner, repo, filePath, ref, err)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Error getting file content from GitHub for %s/%s path '%s' (ref: %s). Error: %v", owner, repo, filePath, ref, err)), nil
	}

	if fileContent == nil {
		return mcp.NewToolResultError(fmt.Sprintf("No content received for file %s/%s path '%s' (ref: %s). The path might be a directory or an empty file.", owner, repo, filePath, ref)), nil
	}

	encodedContent, err := fileContent.GetContent()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error retrieving encoded content for %s/%s path '%s' (is it a symlink/submodule?): %v", owner, repo, filePath, err)), nil
	}

	if encodedContent == "" {
		return mcp.NewToolResultText(""), nil // File is empty
	}

	if strings.ToLower(outputFormat) == "base64" {
		return mcp.NewToolResultText(encodedContent), nil
	}

	// Default is "raw", so decode from base64
	decodedContent, err := base64.StdEncoding.DecodeString(encodedContent)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error decoding file content from base64 for %s/%s path '%s': %v", owner, repo, filePath, err)), nil
	}

	return mcp.NewToolResultText(string(decodedContent)), nil
}
