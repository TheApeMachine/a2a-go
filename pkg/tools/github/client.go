package github

// Very small wrapper around GitHub’s REST API v3 using the standard library
// only.  The goal is to avoid adding external dependencies (such as the
// official go‑github client) so the a2a‑go SDK can compile in completely
// offline environments.

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

type Client struct {
    httpClient *http.Client
    token      string
}

// New creates a token‑authenticated GitHub client.  Pass an empty token for
// unauthenticated (limited‑rate) access.
func New(token string) *Client {
    return &Client{
        token: token,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

func (c *Client) do(ctx context.Context, method, url string, body any, out any) error {
    var r io.Reader
    if body != nil {
        b, _ := json.Marshal(body)
        r = bytes.NewReader(b)
    }
    req, _ := http.NewRequestWithContext(ctx, method, url, r)
    req.Header.Set("Accept", "application/vnd.github+json")
    if body != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    if c.token != "" {
        req.Header.Set("Authorization", "Bearer "+c.token)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("github: %s %s returned %s", method, url, resp.Status)
    }
    if out != nil {
        return json.NewDecoder(resp.Body).Decode(out)
    }
    return nil
}

// ListPRs returns all pull requests for the given repo (owner/repo).  state can
// be "open", "closed", or "all".
func (c *Client) ListPRs(ctx context.Context, repo, state string) ([]map[string]any, error) {
    if state == "" {
        state = "open"
    }
    url := fmt.Sprintf("https://api.github.com/repos/%s/pulls?state=%s", repo, state)
    var out []map[string]any
    err := c.do(ctx, http.MethodGet, url, nil, &out)
    return out, err
}

// CreateIssue opens a new GitHub issue.
func (c *Client) CreateIssue(ctx context.Context, repo, title, body string) (map[string]any, error) {
    if strings.TrimSpace(title) == "" {
        return nil, errors.New("title is required")
    }
    url := fmt.Sprintf("https://api.github.com/repos/%s/issues", repo)
    payload := map[string]string{"title": title, "body": body}
    var out map[string]any
    err := c.do(ctx, http.MethodPost, url, payload, &out)
    return out, err
}

// CommentPR adds a comment to an existing pull request.
func (c *Client) CommentPR(ctx context.Context, repo string, prNumber int, body string) (map[string]any, error) {
    url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", repo, prNumber)
    payload := map[string]string{"body": body}
    var out map[string]any
    err := c.do(ctx, http.MethodPost, url, payload, &out)
    return out, err
}
