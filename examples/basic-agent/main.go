// Basic Agent example – demonstrates building an AgentCard, mapping it to MCP
// objects, and optionally chatting with the agent via OpenAI.
//
//	go run ./examples/basic-agent
//
// Set OPENAI_API_KEY to exercise the OpenAI part of the demo.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/tools"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func main() {
	card := createCard()

	prettyPrint("Agent Card", card)

	// Convert to MCP representations.
	mcpRes := tools.ToMCPResource(card)
	mcpTool := tools.ToMCPTool(card.Skills[0])
	prettyPrint("MCP Resource", mcpRes)
	prettyPrint("Skill as MCP Tool", mcpTool)

	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Println("OPENAI_API_KEY not set – skipping OpenAI chat")
		return
	}

	executor := func(ctx context.Context, t mcp.Tool, args map[string]interface{}) (string, error) {
		return fmt.Sprintf("tool %s invoked with %v", t.Name, args), nil
	}

	chat := provider.NewChatClient(executor)

	userMsg := types.Message{
		Role: "user",
		Parts: []types.Part{{
			Type: types.PartTypeText,
			Text: "Please echo 'all good'.",
		}},
	}

	reply, err := chat.Complete(context.Background(), []types.Message{userMsg}, []mcp.Tool{mcpTool})
	if err != nil {
		log.Fatalf("chat completion failed: %v", err)
	}

	fmt.Printf("\nAssistant: %s\n", reply)
}

func createCard() types.AgentCard {
	desc := ptr("Echoes any text back to the caller")
	return types.AgentCard{
		Name:         "Echo Agent",
		URL:          "http://localhost:8080",
		Version:      "0.1.0",
		Description:  ptr("A tiny demo agent to showcase the a2a‑go SDK."),
		Capabilities: types.AgentCapabilities{Streaming: true},
		Skills: []types.AgentSkill{{
			ID:          "echo",
			Name:        "Echo",
			Description: desc,
			InputModes:  []string{"text"},
			OutputModes: []string{"text"},
		}},
	}
}

func ptr(s string) *string { return &s }

func prettyPrint(label string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("%s:\n%s\n\n", label, b)
}
