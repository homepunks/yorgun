package docker

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const defaultSocketProxy = "tcp://127.0.0.1:2375"

type ContainerStatus struct {
	Name         string
	Service      string
	State        string
	Health       string
	Status       string
	ExitCode     int
	Error        string
	OOMKilled    bool
	RestartCount int
	StartedAt    time.Time
}

type Event struct {
	Service  string
	Action   string
	ExitCode string
	Time     time.Time
}

type Client struct {
	cli     *client.Client
	project string
}

func NewClient(project string) (*Client, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}

	if host := resolveHost(); host != "" {
		opts = append(opts, client.WithHost(host))
	} else {
		opts = append(opts, client.FromEnv)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	_, err = cli.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("connecting to docker at %s: %w", cli.DaemonHost(), err)
	}

	return &Client{cli: cli, project: project}, nil
}

func (c *Client) Host() string {
	return c.cli.DaemonHost()
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) FetchAll(ctx context.Context) ([]ContainerStatus, error) {
	f := filters.NewArgs(
		filters.Arg("label", "com.docker.compose.project="+c.project),
	)

	containers, err := c.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	statuses := make([]ContainerStatus, 0, len(containers))

	for _, ctr := range containers {
		inspect, err := c.cli.ContainerInspect(ctx, ctr.ID)
		if err != nil {
			return nil, fmt.Errorf("inspecting %s: %w", ctr.ID[:12], err)
		}

		s := ContainerStatus{
			Name:         strings.TrimPrefix(inspect.Name, "/"),
			Service:      ctr.Labels["com.docker.compose.service"],
			State:        inspect.State.Status,
			Status:       ctr.Status,
			ExitCode:     inspect.State.ExitCode,
			Error:        inspect.State.Error,
			OOMKilled:    inspect.State.OOMKilled,
			RestartCount: inspect.RestartCount,
			Health:       "none",
		}

		if inspect.State.Health != nil {
			s.Health = inspect.State.Health.Status
		}

		if t, err := time.Parse(time.RFC3339Nano, inspect.State.StartedAt); err == nil {
			s.StartedAt = t
		}

		statuses = append(statuses, s)
	}

	return statuses, nil
}

func (c *Client) Watch(ctx context.Context) (<-chan Event, <-chan error) {
	out := make(chan Event)
	errChan := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errChan)

		for {
			err := c.streamEvents(ctx, out)
			if ctx.Err() != nil {
				return
			}

			log.Printf("event stream disconnected: %v - reconnecting in 5s\n", err)
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, errChan
}

func (c *Client) streamEvents(ctx context.Context, out chan<- Event) error {
	f := filters.NewArgs(
		filters.Arg("type", "container"),
		filters.Arg("label", "com.docker.compose.project="+c.project),
		filters.Arg("event", "die"),
		filters.Arg("event", "stop"),
		filters.Arg("event", "start"),
		filters.Arg("event", "oom"),
		filters.Arg("event", "health_status"),
	)

	msgs, errs := c.cli.Events(ctx, events.ListOptions{Filters: f})

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("event channel closed")
			}

			ev := Event{
				Service:  msg.Actor.Attributes["com.docker.compose.service"],
				Action:   string(msg.Action),
				ExitCode: msg.Actor.Attributes["exitCode"],
				Time:     time.Unix(msg.Time, 0),
			}

			if ev.Service != "" {
				out <- ev
			}

		case err, ok := <-errs:
			if !ok {
				return fmt.Errorf("error channel closed")
			}
			return err

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func resolveHost() string {
	if os.Getenv("DOCKER_HOST") != "" {
		return ""
	}

	switch runtime.GOOS {
	case "darwin":
		// yes, im using podman locally
		if sock := podmanSocket(); sock != "" {
			return "unix://" + sock
		}
	case "linux":
		return defaultSocketProxy
	}

	return ""
}

func podmanSocket() string {
	podmanViaTmp := os.TempDir() + "/podman/podman-machine-default-api.sock"

	if _, err := os.Stat(podmanViaTmp); err == nil {
		return podmanViaTmp
	}

	return ""
}
