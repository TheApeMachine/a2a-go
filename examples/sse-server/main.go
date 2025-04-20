// sse-server showcases the minimal code required to expose task events over
// Server‑Sent Events using the SSEBroker.
//
//	go run ./examples/sse-server
//	curl -N http://localhost:8080/events
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/theapemachine/a2a-go/pkg/service/sse"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func main() {
	broker := sse.NewSSEBroker()

	http.HandleFunc("/events", broker.Subscribe)

	go func() {
		i := 0
		for {
			evt := types.TaskStatusUpdateEvent{
				ID: fmt.Sprintf("task-%d", i),
				Status: types.TaskStatus{
					State: types.TaskStateWorking,
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
