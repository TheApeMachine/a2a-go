// agent-client demonstrates how to use the high‑level a2a.Agent abstraction to
// communicate with a remote agent.  For a self‑contained runnable demo we spin
// up a minimal in‑process JSON‑RPC server that implements the "tasks/send"
// method and then point the Agent at it.
//
//   go run ./examples/agent-client
//
// The output should look similar to:
//   Sent task, received status: completed
//   Artifacts: [hello from agent]

package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "time"

    a2a "github.com/theapemachine/a2a-go"
)

func main() {
    // ---------------------------------------------------------------------
    // 1. Start a tiny stub agent server so the example is self‑contained.
    // ---------------------------------------------------------------------

    rpcSrv := a2a.NewRPCServer()

    rpcSrv.Register("tasks/send", func(ctx context.Context, params json.RawMessage) (interface{}, *a2a.RPCError) {
        var p a2a.TaskSendParams
        _ = json.Unmarshal(params, &p) // ignore errors for brevity in example
        now := time.Now().UTC()
        task := a2a.Task{
            ID: p.ID,
            Status: a2a.TaskStatus{
                State:     a2a.TaskStateCompleted,
                Timestamp: &now,
            },
            Artifacts: []a2a.Artifact{{
                Parts: []a2a.Part{{Type: a2a.PartTypeText, Text: "hello from agent"}},
            }},
        }
        return task, nil
    })

    // Serve JSON‑RPC on :8080/rpc
    go func() {
        log.Println("Stub agent RPC listening on http://localhost:8080/rpc")
        http.Handle("/rpc", rpcSrv)
        _ = http.ListenAndServe(":8080", nil)
    }()

    // Give the server a moment to start (avoids connection refused when run
    // with `go run`).
    time.Sleep(100 * time.Millisecond)

    // ---------------------------------------------------------------------
    // 2. Create an Agent instance pointing at the stub and send a task.
    // ---------------------------------------------------------------------

    card := a2a.AgentCard{
        Name: "StubAgent",
        URL:  "http://localhost:8080",
        Capabilities: a2a.AgentCapabilities{
            Streaming: false,
        },
        Skills: []a2a.AgentSkill{},
    }

    agent := a2a.NewAgentFromCard(card)

    msg := a2a.Message{
        Role: "user",
        Parts: []a2a.Part{{
            Type: a2a.PartTypeText,
            Text: "ping",
        }},
    }

    params := a2a.TaskSendParams{
        ID:      "task‑1",
        Message: msg,
    }

    task, err := agent.Send(context.Background(), params)
    if err != nil {
        log.Fatalf("send failed: %v", err)
    }

    log.Printf("Sent task, received status: %s", task.Status.State)
    if len(task.Artifacts) > 0 && len(task.Artifacts[0].Parts) > 0 {
        log.Printf("Artifacts: [%s]", task.Artifacts[0].Parts[0].Text)
    }
}
