package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	devconfig "github.com/Souheab/lazydevcontainer/internal/config"
	devtemplates "github.com/Souheab/lazydevcontainer/internal/templates"
)

func loadContainers(provider ContainerService) tea.Cmd {
	return func() tea.Msg {
		if provider == nil {
			return containersLoadedMsg{err: errors.New("no container provider configured")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
		defer cancel()

		containers, err := provider.ListContainers(ctx)
		return containersLoadedMsg{containers: containers, err: friendlyDockerError(err)}
	}
}

func runContainerAction(provider ContainerService, action pendingAction) tea.Cmd {
	return func() tea.Msg {
		if provider == nil {
			return containerActionCompletedMsg{action: action, err: errors.New("no container provider configured")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
		defer cancel()

		var err error
		switch action.kind {
		case actionStart:
			err = provider.StartContainer(ctx, action.containerID)
		case actionStop:
			err = provider.StopContainer(ctx, action.containerID)
		case actionRestart:
			err = provider.RestartContainer(ctx, action.containerID)
		default:
			err = errors.New("no container action selected")
		}
		return containerActionCompletedMsg{action: action, err: err}
	}
}

func writeTemplate(template devtemplates.Template, path string) tea.Cmd {
	return func() tea.Msg {
		rendered, err := template.Render()
		if err != nil {
			return templateWriteCompletedMsg{path: path, err: fmt.Errorf("render devcontainer template: %w", err)}
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return templateWriteCompletedMsg{path: path, err: fmt.Errorf("create .devcontainer directory: %w", err)}
		}
		if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
			return templateWriteCompletedMsg{path: path, err: fmt.Errorf("write devcontainer template: %w", err)}
		}
		return templateWriteCompletedMsg{path: path}
	}
}

func loadConfig(targetDir string) tea.Cmd {
	return func() tea.Msg {
		candidates, err := devconfig.Discover(targetDir)
		if err != nil {
			return configLoadedMsg{err: err}
		}
		if len(candidates) == 0 {
			return configLoadedMsg{err: errors.New("no devcontainer config target found")}
		}
		document, err := devconfig.Load(candidates[0].Path)
		return configLoadedMsg{candidates: candidates, document: document, err: err}
	}
}

func loadConfigPath(path string, candidates []devconfig.ConfigCandidate) tea.Cmd {
	return func() tea.Msg {
		document, err := devconfig.Load(path)
		return configLoadedMsg{candidates: candidates, document: document, err: err}
	}
}

func saveConfig(document devconfig.ConfigDocument) tea.Cmd {
	return func() tea.Msg {
		err := devconfig.Save(document)
		return configSavedMsg{path: document.Path, err: err}
	}
}

func loadFeatureCatalog() tea.Cmd {
	return func() tea.Msg {
		features, err := devconfig.LoadFeatureCatalog(context.Background())
		return featureCatalogLoadedMsg{features: features, err: err}
	}
}

func friendlyDockerError(err error) error {
	if err == nil {
		return nil
	}

	message := err.Error()
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "permission denied"):
		return fmt.Errorf("Docker is reachable, but permission was denied. Check access to the Docker socket: %w", err)
	case strings.Contains(lower, "cannot connect") || strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such file"):
		return fmt.Errorf("Docker is not reachable. Start Docker or set DOCKER_HOST, then refresh: %w", err)
	default:
		return err
	}
}
