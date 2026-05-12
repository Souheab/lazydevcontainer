package docker

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"

	"github.com/Souheab/lazydevcontainer/internal/devcontainer"
	"github.com/Souheab/lazydevcontainer/internal/domain"
	containerfilter "github.com/Souheab/lazydevcontainer/internal/filter"
)

func TestFromSummaryIncludesOrdinaryContainer(t *testing.T) {
	summary := container.Summary{
		ID:      "abcdef0123456789",
		Names:   []string{"/postgres"},
		Image:   "postgres:16",
		Command: "postgres",
		State:   container.StateRunning,
		Status:  "Up 5 minutes",
		Created: 1710000000,
		Ports: []container.PortSummary{
			{
				IP:          netip.MustParseAddr("127.0.0.1"),
				PrivatePort: 5432,
				PublicPort:  15432,
				Type:        "tcp",
			},
		},
		Labels: map[string]string{
			"com.example.service": "database",
		},
		Mounts: []container.MountPoint{
			{
				Type:        mount.TypeVolume,
				Name:        "pgdata",
				Destination: "/var/lib/postgresql/data",
				RW:          true,
			},
		},
	}

	got := fromSummary(summary)

	if got.IsDevcontainer {
		t.Fatalf("ordinary container was marked as devcontainer: %+v", got)
	}
	if got.Name != "postgres" {
		t.Fatalf("name = %q, want postgres", got.Name)
	}
	if got.ShortID != "abcdef012345" {
		t.Fatalf("short ID = %q, want abcdef012345", got.ShortID)
	}
	if got.Image != "postgres:16" || got.Status != "Up 5 minutes" || got.State != string(container.StateRunning) {
		t.Fatalf("unexpected mapped container fields: %+v", got)
	}
	if len(got.Ports) != 1 || got.Ports[0].IP != "127.0.0.1" || got.Ports[0].PublicPort != 15432 || got.Ports[0].PrivatePort != 5432 {
		t.Fatalf("unexpected mapped ports: %+v", got.Ports)
	}
	if got.Created != time.Unix(1710000000, 0) {
		t.Fatalf("created = %v, want %v", got.Created, time.Unix(1710000000, 0))
	}
	if got.DevcontainerPath != "" || got.DevcontainerSource != "" {
		t.Fatalf("ordinary container should not have devcontainer metadata: %+v", got)
	}
}

func TestFromSummaryMarksDevcontainerWithoutFilteringIt(t *testing.T) {
	summary := container.Summary{
		ID:    "1234567890abcdef",
		Names: []string{"/app"},
		Image: "golang:1.24",
		Labels: map[string]string{
			devcontainer.LabelLocalFolder: "/home/me/app",
		},
	}

	got := fromSummary(summary)

	if !got.IsDevcontainer {
		t.Fatalf("expected devcontainer metadata, got %+v", got)
	}
	if got.DevcontainerPath != "/home/me/app" {
		t.Fatalf("devcontainer path = %q, want /home/me/app", got.DevcontainerPath)
	}
	if got.DevcontainerSource != "label:"+devcontainer.LabelLocalFolder {
		t.Fatalf("devcontainer source = %q, want label:%s", got.DevcontainerSource, devcontainer.LabelLocalFolder)
	}
	if got.Name != "app" {
		t.Fatalf("name = %q, want app", got.Name)
	}
}

func TestSortDefaultKeepsOrdinaryContainersAfterDevcontainers(t *testing.T) {
	summaries := []container.Summary{
		{
			ID:    "ordinary",
			Names: []string{"/aaa-db"},
			Image: "postgres:16",
		},
		{
			ID:    "dev",
			Names: []string{"/zzz-app"},
			Image: "golang:1.24",
			Labels: map[string]string{
				devcontainer.LabelLocalFolder: "/home/me/app",
			},
		},
	}

	containers := make([]domain.Container, 0, len(summaries))
	for _, summary := range summaries {
		containers = append(containers, fromSummary(summary))
	}

	containerfilter.SortDefault(containers)

	if len(containers) != 2 {
		t.Fatalf("got %d containers, want 2", len(containers))
	}
	if containers[0].Name != "zzz-app" || !containers[0].IsDevcontainer {
		t.Fatalf("first container = %+v, want devcontainer zzz-app", containers[0])
	}
	if containers[1].Name != "aaa-db" || containers[1].IsDevcontainer {
		t.Fatalf("second container = %+v, want ordinary aaa-db", containers[1])
	}
}

func TestLifecycleMethodsRequireInitializedDockerClient(t *testing.T) {
	ctx := context.Background()
	client := &Client{}

	for name, run := range map[string]func() error{
		"start":   func() error { return client.StartContainer(ctx, "abc") },
		"stop":    func() error { return client.StopContainer(ctx, "abc") },
		"restart": func() error { return client.RestartContainer(ctx, "abc") },
	} {
		err := run()
		if err == nil {
			t.Fatalf("%s returned nil error for uninitialized client", name)
		}
		if !strings.Contains(err.Error(), "Docker client is not initialized") {
			t.Fatalf("%s error = %q, want initialized message", name, err)
		}
	}
}
