package main

// docker-exec uses the low‑level docker.Exec helper directly.  It is kept
// separate from the MCP layer so developers without a Docker daemon can still
// build the binary – the call will simply fail at runtime and we print the
// error.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/theapemachine/a2a-go/pkg/tools/docker"
)

func main() {
	res, err := docker.Exec(context.Background(), "busybox:latest", []string{"uname", "-a"}, 30*time.Second, &docker.ExecOptions{})
	if err != nil {
		log.Fatalf("docker exec failed: %v", err)
	}

	b, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(b))
}
