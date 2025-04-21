package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"time"

	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Result struct {
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

type Environment struct {
	client      *client.Client
	containerID string
}

func NewEnvironment() (*Environment, error) {
	client, err := client.NewClientWithOpts(
		client.FromEnv, client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}
	return &Environment{
		client: client,
	}, nil
}

func (env *Environment) Exec(
	ctx context.Context, cmd string, containerName string,
) (Result, error) {
	containers, err := env.client.ContainerList(ctx, container.ListOptions{All: true})

	if err != nil {
		return Result{}, err
	}

	for _, container := range containers {
		if slices.Contains(container.Names, "/"+containerName) {
			env.containerID = container.ID
			break
		}
	}

	if env.containerID == "" {
		if err = env.BuildImage(ctx, "a2a-go"); err != nil {
			return Result{}, err
		}

		resp, err := env.client.ContainerCreate(ctx,
			&container.Config{
				Image: containerName,
				Cmd:   []string{"/bin/bash"},
				Tty:   true,
			},
			nil, nil, nil, containerName,
		)

		if err != nil {
			return Result{}, err
		}

		env.containerID = resp.ID
	}

	log.Info("Creating exec", "containerID", env.containerID)
	exec, err := env.client.ContainerExecCreate(
		ctx,
		env.containerID,
		container.ExecOptions{
			User:         "agent",
			Cmd:          []string{"/bin/sh", "-c", cmd},
			AttachStdout: true,
			AttachStderr: true,
		},
	)

	if err != nil {
		return Result{}, err
	}

	// Attach to the exec instance to get the output
	log.Info("Attaching to exec", "execID", exec.ID)
	resp, err := env.client.ContainerExecAttach(
		ctx, exec.ID, container.ExecStartOptions{},
	)
	if err != nil {
		return Result{}, err
	}
	defer resp.Close()

	// Start the command
	log.Info("Starting exec", "execID", exec.ID)
	if err := env.client.ContainerExecStart(
		ctx, exec.ID, container.ExecStartOptions{},
	); err != nil {
		return Result{}, err
	}

	// Create buffers to capture output
	var result Result
	result.Stdout = &bytes.Buffer{}
	result.Stderr = &bytes.Buffer{}

	mw := io.MultiWriter(result.Stdout)
	mwErr := io.MultiWriter(result.Stderr)

	// Copy output using the demultiplexer since we're not in TTY mode
	errCh := make(chan error, 1)
	go func() {
		errCh <- demultiplexDockerStream(resp.Reader, mw, mwErr)
	}()

	// Wait for the command to complete
	for {
		inspectResp, err := env.client.ContainerExecInspect(ctx, exec.ID)
		if err != nil {
			return Result{}, err
		}
		if !inspectResp.Running {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for output copying to complete
	copyErr := <-errCh
	if copyErr != nil && copyErr != io.EOF {
		return Result{}, copyErr
	}

	// Check if this was an EOF during input read
	if stderrStr := result.Stderr.String(); stderrStr != "" &&
		(stderrStr == "EOFError: EOF when reading a line\n" ||
			stderrStr == "EOFError: EOF when reading a line") {
		// Just return without error - this is expected for interactive programs
		return result, nil
	}

	return result, nil
}

/*
BuildImage builds a Docker image from a Dockerfile.

It creates a tar archive containing the Dockerfile, builds the image,
and processes the build output. Returns an error if the build fails.
*/
func (env *Environment) BuildImage(
	ctx context.Context, imageName string,
) error {
	log.Info("Building image", "imageName", imageName)
	home, err := os.UserHomeDir()

	if err != nil {
		return err
	}

	log.Info("Reading Dockerfile", "path", path.Join(home, ".a2a-go", "Dockerfile"))

	dockerfile, err := os.ReadFile(path.Join(home, ".a2a-go", "Dockerfile"))

	if err != nil {
		return err
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	header := &tar.Header{
		Name: "Dockerfile",
		Mode: 0600,
		Size: int64(len(dockerfile)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tw.Write(dockerfile); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}

	log.Info("tar created")

	opts := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{imageName},
		Remove:     false,
		BuildArgs: map[string]*string{
			"TARGETARCH": nil,
		},
	}

	resp, err := env.client.ImageBuild(ctx, &buf, opts)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return env.print(resp.Body)
}

/*
print processes Docker build output.

It decodes the JSON stream from the build process and prints progress
information. Returns an error if output processing fails or if the
build reports an error.
*/
func (env *Environment) print(reader io.Reader) error {
	decoder := json.NewDecoder(reader)
	for {
		var message struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}

		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		if message.Error != "" {
			return errors.New(message.Error)
		}

		if message.Stream != "" {
			fmt.Print(message.Stream)
		}
	}
}

/*
demultiplexDockerStream processes a Docker multiplexed stream.

It reads the Docker stream header format and routes the data to the appropriate
stdout or stderr writer. Returns an error if stream processing fails.
*/
func demultiplexDockerStream(reader io.Reader, stdout, stderr io.Writer) error {
	var (
		header = make([]byte, 8)
		err    error
	)

	for {
		// Read header
		_, err = io.ReadFull(reader, header)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// Get size of the coming message
		size := int64(header[4])<<24 | int64(header[5])<<16 | int64(header[6])<<8 | int64(header[7])

		// Choose writer based on stream type (header[0])
		var w io.Writer
		switch header[0] {
		case 1:
			w = stdout
		case 2:
			w = stderr
		default:
			continue
		}

		// Copy the message to the appropriate writer
		_, err = io.CopyN(w, reader, size)
		if err != nil {
			return err
		}
	}
}
