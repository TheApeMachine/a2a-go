package browser

import (
    "context"
    "encoding/base64"
    "errors"
    "net/url"
    "strings"
    "time"

    "github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/launcher"
)

// Result captures the essential page data returned by Fetch.
type Result struct {
    Title         string `json:"title"`
    URL           string `json:"url"`
    Text          string `json:"text"`
    Duration      int64  `json:"duration_ms"`
    Screenshot    string `json:"screenshot,omitempty"` // Base64 encoded image
    HasScreenshot bool   `json:"has_screenshot"`
}

const maxTextLen = 4 * 1024 // 4 KB cap on extracted text

// Fetch opens pageURL in a headless browser, waits for the load event, and
// extracts visible text and optionally takes a screenshot.
// If selector is supplied we only extract that DOM subtree.
// If takeScreenshot is true, a base64-encoded screenshot will be included in the result.
// The function is cancellable via ctx.
func Fetch(ctx context.Context, pageURL, selector string, takeScreenshot bool, waitForSelector string) (*Result, error) {
    u, err := url.Parse(pageURL)
    if err != nil {
        return nil, err
    }
    if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "data" {
        return nil, errors.New("unsupported URL scheme (allowed: http, https, data)")
    }

    launch := launcher.New().Headless(true).Leakless(true)
    // respect ctx deadline when launching the browser binary
    if deadline, ok := ctx.Deadline(); ok {
        launch = launch.Context(ctx)
        launch.Set("--timeout", time.Until(deadline).String())
    }
    wsURL, err := launch.Launch()
    if err != nil {
        return nil, err
    }

    browser := rod.New().ControlURL(wsURL)
    if err := browser.Connect(); err != nil {
        return nil, err
    }
    defer browser.Close()

    start := time.Now()
    page := browser.MustPage()
    if err := page.Navigate(pageURL); err != nil {
        return nil, err
    }
    page.MustWaitLoad()
    
    // Wait for specific selector if provided
    if waitForSelector != "" {
        page.Timeout(5 * time.Second).MustElement(waitForSelector)
    }

    var txt string
    var el *rod.Element
    if selector != "" {
        el = page.Timeout(2 * time.Second).MustElement(selector)
    } else {
        el = page.Timeout(2 * time.Second).MustElement("body")
    }
    txt = el.MustText()
    if len(txt) > maxTextLen {
        txt = txt[:maxTextLen]
    }

    title := page.MustEval(`() => document.title`).String()
    
    // Take screenshot if requested
    var screenshot string
    if takeScreenshot {
        // Capture a screenshot of the page
        screenshotBytes, err := page.Screenshot(true, nil)
        if err != nil {
            // Don't fail the entire operation if screenshot fails
            screenshot = ""
        } else {
            // Convert to base64
            screenshot = "data:image/png;base64," + base64.StdEncoding.EncodeToString(screenshotBytes)
        }
    }

    return &Result{
        Title:         title,
        URL:           page.MustInfo().URL,
        Text:          strings.TrimSpace(txt),
        Duration:      time.Since(start).Milliseconds(),
        Screenshot:    screenshot,
        HasScreenshot: screenshot != "",
    }, nil
}
