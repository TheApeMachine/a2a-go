# Prompt Kitchen 🍲

*Become a master prompt chef – mix, taste, iterate.*

## What You’ll Learn

1. Create static & multi‑step prompts programmatically.  
2. Fetch prompts over MCP.  
3. Use them when sending tasks.

---

## 1  Define a Prompt

```go
pm := prompts.NewDefaultManager()

greet := prompts.Prompt{
    Name:        "Super‑Greeting",
    Description: "Greets the user in pirate style ☠️",
    Type:        prompts.SingleStepPrompt,
    Content:     "Ahoy matey! How be I of service today?",
}

pm.Create(ctx, greet)
```

### Multi‑step recipe

```go
flow := prompts.Prompt{ Name:"Support‑Flow", Type:prompts.MultiStepPrompt }
flowPtr, _ := pm.Create(ctx, flow)

steps := []string{"Greeting", "Gather Info", "Suggest Fix", "Closing"}
for i, txt := range steps {
    pm.CreateStep(ctx, prompts.PromptStep{ PromptID: flowPtr.ID, Name: txt, Content: txt, Order: i+1 })
}
```

---

## 2  Fetch via MCP

```bash
curl -s -X POST :8080/rpc -d '{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"Super‑Greeting"}}'
```

The server returns `messages[]` ready to be injected into a chat completion.

---

## 3  Use in a task

```go
promptRes, _ := promptHandler.HandleGetPrompt(ctx, &mcp.GetPromptRequest{Params: struct{ Name string }{"Super‑Greeting"}})

// pick first message as system prompt
sys := promptRes.Messages[0].Content.(*mcp.TextContent).Text

srv.TaskManager.SendTask(ctx, a2a.TaskSendParams{ ID:"t1", Message:a2a.Message{
    Role:"user", Parts:[]a2a.Part{{Type:a2a.PartTypeText, Text:sys}},
}})
```

Voilà – practical prompt reuse!  Try adding template arguments next.

---

## What’s Next?

* Explore [Resource Pantry](resources.md) to embed images/files in your prompts.  
* Dive into [Sampling Lab](sampling.md) to generate responses from GPT‑4o.
