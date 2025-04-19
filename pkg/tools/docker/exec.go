package docker

// Lightweight wrapper around the local `docker` CLI.  It requires Docker to
// be installed and the current user to have permission to run containers.

import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "os/exec"
    "strings"
    "time"
)

// ExecOptions provides configuration options for Docker container execution.
type ExecOptions struct {
    // Volumes maps host paths to container paths for mounting
    Volumes map[string]string
    // Env specifies environment variables to set in the container
    Env map[string]string
    // Network specifies the network configuration (default: "none")
    Network string
    // Memory specifies the memory limit (default: "256m")
    Memory string
    // MaxRetries specifies how many times to retry on temporary failures
    MaxRetries int
    // RetryDelay specifies the delay between retries
    RetryDelay time.Duration
}

// Result captures the command output from inside the container.
type Result struct {
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    ExitCode int    `json:"exit_code"`
    Duration int64  `json:"duration_ms"`
    Retries  int    `json:"retries"`
    Error    string `json:"error,omitempty"`
}

// DefaultExecOptions returns the default options for Docker execution.
func DefaultExecOptions() *ExecOptions {
    return &ExecOptions{
        Volumes:    make(map[string]string),
        Env:        make(map[string]string),
        Network:    "none",
        Memory:     "256m",
        MaxRetries: 3,
        RetryDelay: 2 * time.Second,
    }
}

// Exec runs `docker run --rm <image> <cmd...>` with configurable options.
// If Docker is not available we return an error.
func Exec(ctx context.Context, image string, cmd []string, timeout time.Duration, opts *ExecOptions) (*Result, error) {
    if len(cmd) == 0 {
        return nil, errors.New("cmd must not be empty")
    }
    if image == "" {
        image = "busybox:latest"
    }
    if timeout <= 0 || timeout > 2*time.Minute {
        timeout = 60 * time.Second
    }
    if opts == nil {
        opts = DefaultExecOptions()
    }

    var retriesCount int
    var lastErr error
    var isTemporaryError bool

    for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
        if attempt > 0 {
            // This is a retry attempt
            retriesCount++
            time.Sleep(opts.RetryDelay)
        }

        // Build docker command with all options
        args := []string{"run", "--rm"}

        // Add network configuration
        if opts.Network != "" {
            args = append(args, "--network", opts.Network)
        }

        // Add memory limit
        if opts.Memory != "" {
            args = append(args, "--memory", opts.Memory)
        }

        // Add volume mounts
        for hostPath, containerPath := range opts.Volumes {
            volumeArg := fmt.Sprintf("%s:%s", hostPath, containerPath)
            args = append(args, "-v", volumeArg)
        }

        // Add environment variables
        for key, value := range opts.Env {
            envArg := fmt.Sprintf("%s=%s", key, value)
            args = append(args, "-e", envArg)
        }

        // Add image and command
        args = append(args, image)
        args = append(args, cmd...)

        // Create a new context with timeout for this attempt
        execCtx, cancel := context.WithTimeout(ctx, timeout)
        defer cancel()

        c := exec.CommandContext(execCtx, "docker", args...)
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
                // Consider certain exit codes as temporary errors that can be retried
                isTemporaryError = exitCode == 125 || // Docker daemon error
                    exitCode == 126 || // Command cannot be invoked
                    exitCode == 137    // Container killed (likely OOM)
            } else if errors.Is(err, context.DeadlineExceeded) {
                lastErr = errors.New("docker exec timed out")
                isTemporaryError = true
                continue
            } else {
                // Unknown error (e.g. Docker not installed)
                lastErr = err
                isTemporaryError = false
                break
            }
        } else {
            // Execution succeeded
            return &Result{
                Stdout:   strings.TrimSpace(stdout.String()),
                Stderr:   strings.TrimSpace(stderr.String()),
                ExitCode: exitCode,
                Duration: duration,
                Retries:  retriesCount,
            }, nil
        }

        // If this is not a temporary error or we've exhausted our retries, return the result
        if !isTemporaryError || attempt >= opts.MaxRetries {
            errMsg := ""
            if lastErr != nil {
                errMsg = lastErr.Error()
            }
            
            return &Result{
                Stdout:   strings.TrimSpace(stdout.String()),
                Stderr:   strings.TrimSpace(stderr.String()),
                ExitCode: exitCode,
                Duration: duration,
                Retries:  retriesCount,
                Error:    errMsg,
            }, nil
        }
    }

    // If we reached here, we've exhausted all retries
    return nil, fmt.Errorf("docker exec failed after %d retries: %w", opts.MaxRetries, lastErr)
}

// ExecWithDefaults runs docker exec with default options for backward compatibility.
func ExecWithDefaults(ctx context.Context, image string, cmd []string, timeout time.Duration) (*Result, error) {
    return Exec(ctx, image, cmd, timeout, DefaultExecOptions())
}
