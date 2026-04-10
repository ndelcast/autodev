package executor

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type ContainerOpts struct {
	Image         string
	ContainerName string
	Cmd           []string
	Env           []string
	Binds         []string
	NetworkMode   string
	Timeout       time.Duration
}

type ContainerResult struct {
	ExitCode int64
	Stdout   string
	Stderr   string
}

type DockerRunner struct {
	client *client.Client
}

func NewDockerRunner() (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &DockerRunner{client: cli}, nil
}

func (d *DockerRunner) Close() error {
	return d.client.Close()
}

// RunContainer creates, starts, waits for, and removes an ephemeral container.
func (d *DockerRunner) RunContainer(ctx context.Context, opts ContainerOpts) (*ContainerResult, error) {
	// Apply timeout to context
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Create
	resp, err := d.client.ContainerCreate(ctx, &container.Config{
		Image: opts.Image,
		Cmd:   opts.Cmd,
		Env:   opts.Env,
	}, &container.HostConfig{
		Binds:       opts.Binds,
		NetworkMode: container.NetworkMode(opts.NetworkMode),
	}, nil, nil, opts.ContainerName)
	if err != nil {
		return nil, fmt.Errorf("creating container %s: %w", opts.ContainerName, err)
	}

	containerID := resp.ID

	// Ensure cleanup
	defer func() {
		rmCtx := context.Background()
		d.client.ContainerRemove(rmCtx, containerID, container.RemoveOptions{Force: true})
	}()

	// Start
	if err := d.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("starting container %s: %w", opts.ContainerName, err)
	}

	// Wait
	statusCh, errCh := d.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	var exitCode int64
	select {
	case err := <-errCh:
		if err != nil {
			// Timeout or Docker error — try to stop the container
			stopCtx := context.Background()
			stopTimeout := 10
			d.client.ContainerStop(stopCtx, containerID, container.StopOptions{Timeout: &stopTimeout})
			return nil, fmt.Errorf("waiting for container %s: %w", opts.ContainerName, err)
		}
	case status := <-statusCh:
		exitCode = status.StatusCode
	}

	// Collect logs
	stdout, stderr := d.collectLogs(containerID)

	return &ContainerResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}, nil
}

func (d *DockerRunner) collectLogs(containerID string) (string, string) {
	ctx := context.Background()

	reader, err := d.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", ""
	}
	defer reader.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	// StdCopy demultiplexes the Docker log stream (8-byte header per frame)
	stdcopy.StdCopy(&stdoutBuf, &stderrBuf, reader)

	return stdoutBuf.String(), stderrBuf.String()
}
