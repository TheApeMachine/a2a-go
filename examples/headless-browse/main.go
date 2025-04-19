// headless-browse demonstrates the browser.Fetch helper directly.  Running
// the example with a live URL requires Chrome/Chromium installed locally.  If
// no argument is provided we fall back to a tiny `data:` URL so the demo is
// fully offline‑capable.
//
//   go run ./examples/headless-browse https://example.com "p"
//
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/theapemachine/a2a-go/pkg/tools/browser"
)

func main() {
    pageURL := "data:text/html,<html><head><title>Demo</title></head><body><h1>Hello</h1><p>Rod example</p></body></html>"
    selector := ""
    if len(os.Args) > 1 {
        pageURL = os.Args[1]
    }
    if len(os.Args) > 2 {
        selector = os.Args[2]
    }

    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    res, err := browser.Fetch(ctx, pageURL, selector)
    if err != nil {
        log.Fatalf("fetch failed: %v", err)
    }

    b, _ := json.MarshalIndent(res, "", "  ")
    fmt.Println(string(b))
}
