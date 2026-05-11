package docker

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"github.com/Souheab/lazydevcontainer/internal/devcontainer"
	"github.com/Souheab/lazydevcontainer/internal/domain"
	containerfilter "github.com/Souheab/lazydevcontainer/internal/filter"
)

// Client lists Docker containers using the Docker Engine API.
type Client struct {
	api *client.Client
}

// New creates a Docker client from the environment and enables API version negotiation.
func New() (*Client, error) {
	api, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}

	return &Client{api: api}, nil
}

// ListContainers returns every Docker container visible to the configured Docker daemon.
func (c *Client) ListContainers(ctx context.Context) ([]domain.Container, error) {
	if c == nil || c.api == nil {
		return nil, fmt.Errorf("Docker client is not initialized")
	}

	result, err := c.api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list Docker containers: %w", err)
	}

	containers := make([]domain.Container, 0, len(result.Items))
	for _, summary := range result.Items {
		containers = append(containers, fromSummary(summary))
	}

	containerfilter.SortDefault(containers)
	return containers, nil
}

func fromSummary(summary container.Summary) domain.Container {
	labels := cloneLabels(summary.Labels)
	mounts := convertMounts(summary.Mounts)
	detected := devcontainer.Detect(labels, mounts)
	names := normalizeNames(summary.Names)

	return domain.Container{
		ID:                 summary.ID,
		ShortID:            shortID(summary.ID),
		Names:              names,
		Name:               primaryName(names, summary.ID),
		Image:              summary.Image,
		Command:            summary.Command,
		State:              string(summary.State),
		Status:             summary.Status,
		Created:            unixTime(summary.Created),
		Labels:             labels,
		Mounts:             mounts,
		IsDevcontainer:     detected.IsDevcontainer,
		DevcontainerPath:   detected.Path,
		DevcontainerSource: detected.Source,
	}
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return map[string]string{}
	}

	cloned := make(map[string]string, len(labels))
	for key, value := range labels {
		cloned[key] = value
	}
	return cloned
}

func convertMounts(points []container.MountPoint) []domain.Mount {
	if len(points) == 0 {
		return nil
	}

	mounts := make([]domain.Mount, 0, len(points))
	for _, point := range points {
		mounts = append(mounts, domain.Mount{
			Type:        string(point.Type),
			Name:        point.Name,
			Source:      point.Source,
			Destination: point.Destination,
			ReadOnly:    !point.RW,
		})
	}
	return mounts
}

func normalizeNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(strings.TrimPrefix(name, "/"))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)
	return normalized
}

func primaryName(names []string, id string) string {
	if len(names) > 0 {
		return names[0]
	}
	return shortID(id)
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func unixTime(seconds int64) time.Time {
	if seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}
