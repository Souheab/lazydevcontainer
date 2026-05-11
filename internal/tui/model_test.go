package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Souheab/lazydevcontainer/internal/domain"
	containerfilter "github.com/Souheab/lazydevcontainer/internal/filter"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestSearchModalAppliesQueryAndClosesOnEnter(t *testing.T) {
	m := testModel([]domain.Container{
		{ID: "1", ShortID: "111", Name: "api", Image: "golang", IsDevcontainer: true},
		{ID: "2", ShortID: "222", Name: "db", Image: "postgres"},
	})

	m = updateModel(t, m, runeKey('/'))
	if m.modal != modalSearch {
		t.Fatalf("expected search modal, got %v", m.modal)
	}

	for _, r := range "api" {
		m = updateModel(t, m, runeKey(r))
	}
	if m.query != "api" {
		t.Fatalf("expected query api, got %q", m.query)
	}
	if len(m.visible) != 1 || m.visible[0].Name != "api" {
		t.Fatalf("unexpected visible containers after search: %+v", m.visible)
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.modal != modalNone {
		t.Fatalf("expected modal to close, got %v", m.modal)
	}
}

func TestFilterModalAppliesSelectedMode(t *testing.T) {
	m := testModel([]domain.Container{
		{ID: "1", ShortID: "111", Name: "api", IsDevcontainer: true},
		{ID: "2", ShortID: "222", Name: "db"},
	})

	m = updateModel(t, m, runeKey('f'))
	if m.modal != modalFilter {
		t.Fatalf("expected filter modal, got %v", m.modal)
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.filterMode != containerfilter.ModeDevcontainers {
		t.Fatalf("expected devcontainers filter, got %s", m.filterMode)
	}
	if len(m.visible) != 1 || !m.visible[0].IsDevcontainer {
		t.Fatalf("unexpected visible containers after filter: %+v", m.visible)
	}
	if m.modal != modalNone {
		t.Fatalf("expected modal to close, got %v", m.modal)
	}
}

func TestFilteringKeepsCursorInBounds(t *testing.T) {
	m := testModel([]domain.Container{
		{ID: "1", ShortID: "111", Name: "api", IsDevcontainer: true},
		{ID: "2", ShortID: "222", Name: "db"},
		{ID: "3", ShortID: "333", Name: "worker"},
	})
	m.cursor = 2

	m.setFilter(containerfilter.ModeDevcontainers)

	if m.cursor != 0 {
		t.Fatalf("expected cursor to clamp to 0, got %d", m.cursor)
	}
	if len(m.visible) != 1 {
		t.Fatalf("expected one visible container, got %d", len(m.visible))
	}
}

func TestRenderRowShowsNamePathAndRightAlignedUptime(t *testing.T) {
	m := testModel(nil)
	container := domain.Container{
		ID:               "abc123456789",
		ShortID:          "abc123456789",
		Name:             "reverent_hertz",
		Image:            "vsc-lazydevcontainer-fc123",
		Status:           "Up 58 minutes",
		IsDevcontainer:   true,
		DevcontainerPath: "/home/user/project",
	}

	row := m.renderRow(0, container, false, 80)
	plain := stripANSI(row)
	lines := strings.Split(plain, "\n")

	for _, want := range []string{"reverent_hertz", "/home/user/project", "Up 58 minutes"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("rendered row %q does not contain %q", plain, want)
		}
	}
	for _, unwanted := range []string{"abc123456789", "vsc-lazydevcontainer-fc123"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("rendered row %q should not contain %q", plain, unwanted)
		}
	}
	if !strings.HasSuffix(lines[0], "Up 58 minutes") {
		t.Fatalf("first line uptime should be right aligned at row edge, got %q", plain)
	}
	if got := lipgloss.Width(row); got != 80 {
		t.Fatalf("row width = %d, want 80", got)
	}
}

func TestRenderRowOmitsPathWhenUnavailable(t *testing.T) {
	m := testModel(nil)
	container := domain.Container{
		Name:           "plain_container",
		Status:         "Up 2 minutes",
		IsDevcontainer: true,
	}

	plain := stripANSI(m.renderRow(0, container, false, 60))
	lines := strings.Split(plain, "\n")

	if strings.Contains(plain, "[]") || strings.Contains(plain, "[") || strings.Contains(plain, "]") {
		t.Fatalf("rendered row should not show path brackets without a path: %q", plain)
	}
	if !strings.Contains(plain, "plain_container") || !strings.HasSuffix(lines[0], "Up 2 minutes") {
		t.Fatalf("rendered row missing name or uptime: %q", plain)
	}
}

func TestRenderMainPaneIncludesDetailsForSelectedContainer(t *testing.T) {
	m := testModel([]domain.Container{
		{
			ID:               "1",
			ShortID:          "111",
			Name:             "api",
			Image:            "golang:1.24",
			Status:           "Up 1 hour",
			DevcontainerPath: "/home/me/api",
			Mounts: []domain.Mount{
				{Type: "volume", Name: "workspace"},
				{Type: "bind", Source: "/home/me/api"},
			},
			Ports: []domain.Port{
				{PrivatePort: 3000, PublicPort: 3000, Type: "tcp"},
			},
		},
	})

	plain := stripANSI(m.renderMainPane())

	for _, want := range []string{"Containers 1 of 1", "Details", "Project", "/home/me/api", "Image", "golang:1.24", "Volumes", "2 attached", "Ports", "3000 -> 3000"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("rendered split pane %q does not contain %q", plain, want)
		}
	}
}

func testModel(containers []domain.Container) Model {
	m := New(nil)
	m.width = 100
	m.height = 30
	m.loading = false
	m.containers = containers
	m.applyFilters()
	m.ensureCursorBounds()
	m.ensureCursorVisible()
	return m
}

func updateModel(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	updated, ok := next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	return updated
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}
