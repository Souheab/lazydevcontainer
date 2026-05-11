package devcontainer

import (
	"path"
	"sort"
	"strings"

	"github.com/Souheab/lazydevcontainer/internal/domain"
)

const (
	LabelLocalFolder       = "devcontainer.local_folder"
	LabelConfigFile        = "devcontainer.config_file"
	LabelMetadata          = "devcontainer.metadata"
	LabelVSCodeLocalFolder = "vsch.local.folder"
	LabelVSCodeConfigFile  = "vsch.devcontainer.config_file"
)

var pathLabelPrecedence = []string{
	LabelLocalFolder,
	LabelVSCodeLocalFolder,
}

var configLabelPrecedence = []string{
	LabelConfigFile,
	LabelVSCodeConfigFile,
}

// Result describes the devcontainer detection outcome for a container.
type Result struct {
	IsDevcontainer bool
	Path           string
	Source         string
}

// Detect identifies whether a Docker container looks like a Dev Container.
// Labels are treated as authoritative, while mount-based detection is a
// conservative fallback for v0.
func Detect(labels map[string]string, mounts []domain.Mount) Result {
	if labels == nil {
		labels = map[string]string{}
	}

	for _, label := range pathLabelPrecedence {
		if value := labelValue(labels, label); value != "" {
			return Result{
				IsDevcontainer: true,
				Path:           cleanPath(value),
				Source:         "label:" + label,
			}
		}
	}

	for _, label := range configLabelPrecedence {
		if value := labelValue(labels, label); value != "" {
			return Result{
				IsDevcontainer: true,
				Path:           workspaceFromConfigPath(value),
				Source:         "label:" + label,
			}
		}
	}

	if value := labelValue(labels, LabelMetadata); value != "" {
		bestMount, ok := bestWorkspaceMount(mounts)
		if ok {
			return Result{
				IsDevcontainer: true,
				Path:           bestMount,
				Source:         "label:" + LabelMetadata + "+mount",
			}
		}
		return Result{
			IsDevcontainer: true,
			Source:         "label:" + LabelMetadata,
		}
	}

	if devLabel := devcontainerLabel(labels); devLabel != "" {
		bestMount, _ := bestWorkspaceMount(mounts)
		return Result{
			IsDevcontainer: true,
			Path:           bestMount,
			Source:         "label:" + devLabel,
		}
	}

	if bestMount, ok := bestWorkspaceMount(mounts); ok {
		return Result{
			IsDevcontainer: true,
			Path:           bestMount,
			Source:         "mount:workspace",
		}
	}

	return Result{}
}

func labelValue(labels map[string]string, key string) string {
	if value := strings.TrimSpace(labels[key]); value != "" {
		return value
	}

	lowerKey := strings.ToLower(key)
	for candidate, value := range labels {
		if strings.ToLower(candidate) == lowerKey {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func devcontainerLabel(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "devcontainer") || strings.Contains(lowerKey, "dev_container") {
			return key
		}
	}

	return ""
}

func bestWorkspaceMount(mounts []domain.Mount) (string, bool) {
	for _, mount := range mounts {
		if pathContainsDevcontainer(mount.Source) {
			return workspaceFromDevcontainerPath(mount.Source), true
		}
	}

	for _, mount := range mounts {
		if pathContainsDevcontainer(mount.Destination) {
			if mount.Source != "" {
				return cleanPath(mount.Source), true
			}
			return workspaceFromDevcontainerPath(mount.Destination), true
		}
	}

	for _, mount := range mounts {
		if isWorkspaceTarget(mount.Destination) && mount.Source != "" {
			return cleanPath(mount.Source), true
		}
	}

	return "", false
}

func isWorkspaceTarget(value string) bool {
	normalized := normalizePath(value)
	return normalized == "/workspace" || strings.HasPrefix(normalized, "/workspaces/")
}

func pathContainsDevcontainer(value string) bool {
	return strings.Contains(normalizePath(value), "/.devcontainer")
}

func workspaceFromConfigPath(value string) string {
	cleaned := cleanPath(value)
	if cleaned == "" {
		return ""
	}

	if pathContainsDevcontainer(cleaned) {
		return workspaceFromDevcontainerPath(cleaned)
	}

	normalized := normalizePath(cleaned)
	dir := path.Dir(normalized)
	if dir == "." {
		return cleaned
	}
	return dir
}

func workspaceFromDevcontainerPath(value string) string {
	cleaned := cleanPath(value)
	normalized := normalizePath(cleaned)
	idx := strings.Index(normalized, "/.devcontainer")
	if idx < 0 {
		return cleaned
	}
	if idx == 0 {
		return "/"
	}
	return normalized[:idx]
}

func cleanPath(value string) string {
	return strings.TrimSpace(strings.Trim(value, "\"'"))
}

func normalizePath(value string) string {
	value = strings.ReplaceAll(cleanPath(value), "\\", "/")
	if len(value) > 1 {
		value = strings.TrimRight(value, "/")
	}
	return value
}
