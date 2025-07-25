package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v60/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"

	// "github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi" // No longer directly needed here

	"github.com/slack-go/slack"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
	"github.com/theapemachine/mcp-server-devops-bridge/pkg/config"
	"golang.org/x/oauth2"
)

// AzureEnrichWorkItemTool provides functionality to search across platforms for information related to a work item.
type AzureEnrichWorkItemTool struct {
	handle mcp.Tool
	// client workitemtracking.Client // No longer directly used by this tool for updates
	config AzureDevOpsConfig // Still useful for Azure context if needed, or could be removed if no Azure client interaction here
}

// SearchResult structures
type GitHubIssueResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Repo    string  `json:"repo"`
	State   string  `json:"state"`
	Body    *string `json:"body,omitempty"`
	DiffURL *string `json:"diff_url,omitempty"`
}

type GitHubCodeResult struct {
	File           string `json:"file"`
	URL            string `json:"url"`
	Repo           string `json:"repo"`
	SnippetPreview string `json:"snippet_preview,omitempty"` // Might not always be available or easy to get
}

type SlackMessageResult struct {
	Permalink   string `json:"permalink"`
	User        string `json:"user"`
	Channel     string `json:"channel"`
	TextPreview string `json:"text_preview"`
}

type SentryIssueResult struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Permalink string `json:"permalink"`
	Project   string `json:"project"`
}

type EnrichmentResults struct {
	GitHubIssuesPRs   []GitHubIssueResult  `json:"github_issues_prs,omitempty"`
	GitHubCodeResults []GitHubCodeResult   `json:"github_code_results,omitempty"`
	SlackMessages     []SlackMessageResult `json:"slack_messages,omitempty"`
	SentryIssues      []SentryIssueResult  `json:"sentry_issues,omitempty"`
}

// NewAzureEnrichWorkItemTool creates a new tool instance for enriching work items.
func NewAzureEnrichWorkItemTool(conn *azuredevops.Connection, globalConfig AzureDevOpsConfig) core.Tool {
	tool := &AzureEnrichWorkItemTool{
		// client: client, // Not storing client if not used for updates
		config: globalConfig, // Keep for Azure context if needed in future, or for logging project
	}

	tool.handle = mcp.NewTool(
		"azure_enrich_work_item",
		mcp.WithDescription("Searches GitHub (issues, PRs, code), Slack, and Sentry for keywords related to a work item. Returns structured results."),
		mcp.WithString("search_keywords", mcp.Required(), mcp.Description("Keywords to search for across platforms.")),
		mcp.WithString("github_repo_slug", mcp.Description("Optional. GitHub repo slug. Defaults to configured GITHUB_REPO_SLUG.")),
		mcp.WithString("sentry_project_slug", mcp.Description("Optional. Sentry project slug. Defaults to configured SENTRY_PROJECT_SLUG.")),
	)
	return tool
}

// Handle returns the MCP tool handle.
func (tool *AzureEnrichWorkItemTool) Handle() mcp.Tool {
	return tool.handle
}

// SentryIssue is defined above with other result structs

// Handler is the main execution function for the enrich work item tool.
func (tool *AzureEnrichWorkItemTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	appCfg := config.Load()
	if appCfg == nil {
		return mcp.NewToolResultError("failed to load application configuration"), nil
	}

	// workItemIDStr, _ := GetStringArg(request, "work_item_id") // For logging
	keywords, err := GetStringArg(request, "search_keywords")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error getting required parameter 'search_keywords': %v", err)), nil
	}
	if keywords == "" {
		return mcp.NewToolResultError("Missing required parameter: search_keywords"), nil
	}

	// Optional parameters with fallback to config
	ghOrg := GetOptionalStringParamWithFallback(request, "github_organization", appCfg.GitHub.Organization)
	searchGHCodeStr, _ := GetStringArg(request, "github_search_code")
	searchGHCode := true
	if strings.ToLower(searchGHCodeStr) == "false" {
		searchGHCode = false
	}

	results := EnrichmentResults{
		GitHubIssuesPRs:   make([]GitHubIssueResult, 0),
		GitHubCodeResults: make([]GitHubCodeResult, 0),
		SlackMessages:     make([]SlackMessageResult, 0),
		SentryIssues:      make([]SentryIssueResult, 0),
	}

	// Step 1.1: GitHub Issues/PRs Search
	if appCfg.GitHub.PersonalAccessToken == "" || ghOrg == "" {
		return mcp.NewToolResultError("GitHub PAT or Organization not configured/provided, skipping GitHub Issues/PRs search."), nil
	} else {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: appCfg.GitHub.PersonalAccessToken})
		tc := oauth2.NewClient(ctx, ts)
		ghClient := github.NewClient(tc)

		// -- TEMP: Hardcoded repos for testing --
		targetGhRepos := []string{"FanApp-Legacy", "phoneapp"}
		repoQueryParts := []string{}
		for _, repoName := range targetGhRepos {
			repoQueryParts = append(repoQueryParts, fmt.Sprintf("repo:%s/%s", ghOrg, repoName))
		}
		repoQueryString := strings.Join(repoQueryParts, " ")
		// -- END TEMP --

		// Construct a query: keywords AND specific repos type:issue type:pr
		ghIssuesQuery := fmt.Sprintf("%s %s type:issue type:pr", keywords, repoQueryString) // Modified
		searchOpts := &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 10}}
		issueSearchResults, _, err := ghClient.Search.Issues(ctx, ghIssuesQuery, searchOpts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error searching GitHub Issues/PRs: %v", err)), nil
		} else {
			for _, issue := range issueSearchResults.Issues {
				repoName := ""
				if issue.RepositoryURL != nil {
					parts := strings.Split(*issue.RepositoryURL, "/")
					if len(parts) > 1 {
						repoName = parts[len(parts)-1]
					}
				}

				var diffURL *string
				if issue.IsPullRequest() && issue.GetPullRequestLinks() != nil {
					val := issue.GetPullRequestLinks().GetDiffURL()
					if val != "" {
						diffURL = &val
					}
				}

				var body *string
				if b := issue.GetBody(); b != "" {
					body = &b
				}

				results.GitHubIssuesPRs = append(results.GitHubIssuesPRs, GitHubIssueResult{
					Title:   issue.GetTitle(),
					URL:     issue.GetHTMLURL(),
					Repo:    repoName,
					State:   issue.GetState(),
					Body:    body,
					DiffURL: diffURL,
				})
			}
		}
	}

	// Step 1.2: GitHub Code Search
	if searchGHCode {
		if appCfg.GitHub.PersonalAccessToken == "" || ghOrg == "" {
			return mcp.NewToolResultError("GitHub PAT or Organization not configured/provided, skipping GitHub Code search."), nil
		} else {
			ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: appCfg.GitHub.PersonalAccessToken})
			tc := oauth2.NewClient(ctx, ts)
			ghClient := github.NewClient(tc)

			// -- TEMP: Hardcoded repos for testing (reusing from above) --
			targetGhRepos := []string{"FanApp-Legacy", "phoneapp"} // Assuming ghOrg is defined
			repoQueryParts := []string{}
			if ghOrg != "" { // Ensure ghOrg is available before using it
				for _, repoName := range targetGhRepos {
					repoQueryParts = append(repoQueryParts, fmt.Sprintf("repo:%s/%s", ghOrg, repoName))
				}
			}
			repoQueryString := strings.Join(repoQueryParts, " ")
			// -- END TEMP --
			ghCodeQuery := fmt.Sprintf("%s %s", keywords, repoQueryString) // Modified
			codeSearchOpts := &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 10}}
			codeSearchResults, _, err := ghClient.Search.Code(ctx, ghCodeQuery, codeSearchOpts)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error searching GitHub Code: %v", err)), nil
			} else {
				for _, result := range codeSearchResults.CodeResults {
					// Snippet preview might need TextMatches or more complex handling
					results.GitHubCodeResults = append(results.GitHubCodeResults, GitHubCodeResult{
						File: result.GetName(),
						URL:  result.GetHTMLURL(),
						Repo: result.GetRepository().GetFullName(),
					})
				}
			}
		}
	}

	// Step 2: Slack Search
	if appCfg.Slack.UserToken == "" {
		return mcp.NewToolResultError("Slack User Token not configured, skipping Slack search."), nil
	} else {
		slackClient := slack.New(appCfg.Slack.UserToken)
		searchParams := slack.NewSearchParameters()
		searchParams.Count = 10
		messages, err := slackClient.SearchMessages(keywords, searchParams)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error searching Slack: %v", err)), nil
		} else {
			for _, msg := range messages.Matches {
				results.SlackMessages = append(results.SlackMessages, SlackMessageResult{
					Permalink:   msg.Permalink,
					User:        msg.Username,
					Channel:     msg.Channel.Name,
					TextPreview: msg.Text, // Full text for now, can be truncated
				})
			}
		}
	}

	// Step 3: Sentry Search
	for _, project := range []string{"4507328219709520", "4507171795959888"} {
		sentryAPIURL := fmt.Sprintf("https://sentry.io/api/0/organizations/%s/issues/?project=%s", appCfg.Sentry.Organization, project)

		httpReq, err := http.NewRequestWithContext(ctx, "GET", sentryAPIURL, nil)
		if err != nil {
		} else {
			httpReq.Header.Set("Authorization", "Bearer "+appCfg.Sentry.AuthToken)
			httpClient := &http.Client{}
			resp, err := httpClient.Do(httpReq)
			if err != nil {
			} else {
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
				} else {
					var sentryIssuesAPIResp []SentryIssueResult // Assuming API returns a list directly for this simplified example
					if err := json.NewDecoder(resp.Body).Decode(&sentryIssuesAPIResp); err != nil {
					} else {
						results.SentryIssues = append(results.SentryIssues, sentryIssuesAPIResp...)
					}
				}
			}
		}
	}

	// Marshal results to JSON
	jsonOutput, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal results to JSON: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonOutput)), nil
}

// GetOptionalStringParamWithFallback is a helper function.
// It tries to get a string parameter from the request.
// If not found or empty, it returns the fallbackValue.
func GetOptionalStringParamWithFallback(req mcp.CallToolRequest, key string, fallbackValue string) string {
	val, err := GetStringArg(req, key) // GetStringArg is from common.go
	if err != nil || val == "" {
		return fallbackValue
	}
	return val
}
