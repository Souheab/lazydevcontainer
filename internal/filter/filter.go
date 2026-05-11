package filter

import (
	"sort"
	"strings"

	"github.com/Souheab/lazydevcontainer/internal/domain"
)

// Mode controls which container classes are visible.
type Mode int

const (
	ModeAll Mode = iota
	ModeDevcontainers
	ModeContainers
)

// String returns the human-readable mode name.
func (m Mode) String() string {
	switch m {
	case ModeDevcontainers:
		return "devcontainers"
	case ModeContainers:
		return "containers"
	default:
		return "all"
	}
}

// Next returns the next filter mode in UI order.
func (m Mode) Next() Mode {
	switch m {
	case ModeAll:
		return ModeDevcontainers
	case ModeDevcontainers:
		return ModeContainers
	default:
		return ModeAll
	}
}

// Previous returns the previous filter mode in UI order.
func (m Mode) Previous() Mode {
	switch m {
	case ModeAll:
		return ModeContainers
	case ModeDevcontainers:
		return ModeAll
	default:
		return ModeDevcontainers
	}
}

// Apply filters by mode and query, then applies the default sort.
func Apply(containers []domain.Container, mode Mode, query string) []domain.Container {
	visible := make([]domain.Container, 0, len(containers))
	for _, container := range containers {
		if !matchesMode(container, mode) {
			continue
		}
		if !MatchesSearch(container, query) {
			continue
		}
		visible = append(visible, container)
	}

	SortDefault(visible)
	return visible
}

// SortDefault places devcontainers first and then sorts by display name.
func SortDefault(containers []domain.Container) {
	sort.SliceStable(containers, func(i, j int) bool {
		left := containers[i]
		right := containers[j]

		if left.IsDevcontainer != right.IsDevcontainer {
			return left.IsDevcontainer
		}

		leftName := strings.ToLower(left.DisplayName())
		rightName := strings.ToLower(right.DisplayName())
		if leftName != rightName {
			return leftName < rightName
		}

		if !left.Created.Equal(right.Created) {
			if left.Created.IsZero() {
				return false
			}
			if right.Created.IsZero() {
				return true
			}
			return left.Created.After(right.Created)
		}

		return left.ID < right.ID
	})
}

// MatchesSearch reports whether a container matches every search term.
func MatchesSearch(container domain.Container, query string) bool {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return true
	}

	haystack := strings.ToLower(searchText(container))
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}

	return true
}

func matchesMode(container domain.Container, mode Mode) bool {
	switch mode {
	case ModeDevcontainers:
		return container.IsDevcontainer
	case ModeContainers:
		return !container.IsDevcontainer
	default:
		return true
	}
}

func searchText(container domain.Container) string {
	parts := []string{
		container.ID,
		container.ShortID,
		container.DisplayName(),
		container.Image,
		container.Command,
		container.State,
		container.Status,
		container.DevcontainerPath,
		container.DevcontainerSource,
	}

	parts = append(parts, container.Names...)
	for key, value := range container.Labels {
		parts = append(parts, key, value)
	}
	for _, mount := range container.Mounts {
		parts = append(parts, mount.Type, mount.Name, mount.Source, mount.Destination)
	}

	return strings.Join(parts, " ")
}
