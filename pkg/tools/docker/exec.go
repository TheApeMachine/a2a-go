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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type Result struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
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
	ctx context.Context, cmd string,
) (Result, error) {
	exec, err := env.client.ContainerExecCreate(
		ctx,
		env.containerID,
		container.ExecOptions{
			Cmd:          []string{"/bin/sh", "-c", cmd},
			AttachStdout: true,
			AttachStderr: true,
		},
	)

	if err != nil {
		return Result{}, err
	}

	env.client.ContainerExecAttach(
		ctx, exec.ID, container.ExecStartOptions{},
	)

	return Result{}, nil
}

/*
BuildImage builds a Docker image from a Dockerfile.

It creates a tar archive containing the Dockerfile, builds the image,
and processes the build output. Returns an error if the build fails.
*/
func (env *Environment) BuildImage(
	ctx context.Context, imageName string,
) error {
	home, err := os.UserHomeDir()

	if err != nil {
		return err
	}

	dockerfile, err := os.ReadFile(path.Join(home, "Dockerfile"))

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

	opts := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       []string{imageName},
		Remove:     true,
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
