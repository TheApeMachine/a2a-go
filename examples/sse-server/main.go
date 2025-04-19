// sse-server showcases the minimal code required to expose task events over
// Server‑Sent Events using the SSEBroker.
//
//   go run ./examples/sse-server
//   curl -N http://localhost:8080/events
package main

import (
    "fmt"
    "log"
    "net/http"
    "time"

    a2a "github.com/theapemachine/a2a-go"
)

func main() {
    broker := a2a.NewSSEBroker()

    http.HandleFunc("/events", broker.Subscribe)

    go func() {
        i := 0
        for {
            evt := a2a.TaskStatusUpdateEvent{
                ID: fmt.Sprintf("task-%d", i),
                Status: a2a.TaskStatus{
                    State: a2a.TaskStateWorking,
                },
            }
            if err := broker.Broadcast(evt); err != nil {
                log.Printf("broadcast error: %v", err)
            }
            i++
            time.Sleep(2 * time.Second)
        }
    }()

    log.Println("Serving SSE stream at http://localhost:8080/events")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
