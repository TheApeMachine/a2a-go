// streaming-chat demonstrates token‑level streaming from the OpenAI API using
// the a2a‑go ChatClient.
//
//   OPENAI_API_KEY=sk‑... go run ./examples/streaming-chat
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    a2a "github.com/theapemachine/a2a-go"
    "github.com/mark3labs/mcp-go/mcp"
)

func main() {
    if os.Getenv("OPENAI_API_KEY") == "" {
        log.Println("OPENAI_API_KEY not set – skipping streaming example")
        return
    }

    executor := func(ctx context.Context, t mcp.Tool, args map[string]interface{}) (string, error) {
        return "tool execution placeholder", nil
    }

    client := a2a.NewChatClient(executor)

    history := []a2a.Message{{
        Role: "user",
        Parts: []a2a.Part{{
            Type: a2a.PartTypeText,
            Text: "Explain the HTTP 418 status code in one sentence.",
        }},
    }}

    fmt.Println("Assistant (streaming):")

    final, err := client.Stream(context.Background(), history, nil, func(delta string) {
        fmt.Print(delta)
    })
    if err != nil {
        log.Fatalf("stream failure: %v", err)
    }

    fmt.Printf("\n\n[final content size=%d bytes]\n", len(final))
}
