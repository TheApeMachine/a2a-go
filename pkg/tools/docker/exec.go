package docker

// Lightweight wrapper around the local `docker` CLI.  It requires Docker to
// be installed and the current user to have permission to run containers.

import (
    "bytes"
    "context"
    "errors"
    "os/exec"
    "strings"
    "time"
)

// Result captures the command output from inside the container.
type Result struct {
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    ExitCode int    `json:"exit_code"`
    Duration int64  `json:"duration_ms"`
}

// Exec runs `docker run --rm <image> <cmd...>` with basic resource limits.  If
// Docker is not available we return an error.
func Exec(ctx context.Context, image string, cmd []string, timeout time.Duration) (*Result, error) {
    if len(cmd) == 0 {
        return nil, errors.New("cmd must not be empty")
    }
    if image == "" {
        image = "busybox:latest"
    }
    if timeout <= 0 || timeout > 2*time.Minute {
        timeout = 60 * time.Second
    }

    // Build docker command.
    args := []string{"run", "--rm", "--network", "none", "--memory", "256m", image}
    args = append(args, cmd...)

    c := exec.CommandContext(ctx, "docker", args...)
    var stdout, stderr bytes.Buffer
    c.Stdout = &stdout
    c.Stderr = &stderr

    start := time.Now()
    err := c.Run()
    duration := time.Since(start).Milliseconds()

    exitCode := 0
    if err != nil {
        if ee, ok := err.(*exec.ExitError); ok {
            exitCode = ee.ExitCode()
        } else if errors.Is(err, context.DeadlineExceeded) {
            return nil, errors.New("docker exec timed out")
        } else {
            // Unknown error (e.g. Docker not installed)
            return nil, err
        }
    }

    return &Result{
        Stdout:   strings.TrimSpace(stdout.String()),
        Stderr:   strings.TrimSpace(stderr.String()),
        ExitCode: exitCode,
        Duration: duration,
    }, nil
}
