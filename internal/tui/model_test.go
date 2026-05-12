package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	devconfig "github.com/Souheab/lazydevcontainer/internal/config"
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

func TestRenderSelectedRowUsesDarkBlueAcrossWholeRow(t *testing.T) {
	m := testModel(nil)
	container := domain.Container{
		Name:             "api",
		Status:           "Up 2 minutes",
		IsDevcontainer:   true,
		DevcontainerPath: "/workspaces/api",
	}

	row := m.renderRow(0, container, true, 60)

	if got := fmt.Sprint(m.styles.SelectedRow.GetBackground()); got != "18" {
		t.Fatalf("selected row background = %q, want dark blue color 18", got)
	}
	for _, want := range []string{"api", "Up 2 minutes", "/workspaces/api"} {
		if !strings.Contains(stripANSI(row), want) {
			t.Fatalf("selected row %q does not contain %q", stripANSI(row), want)
		}
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
			IsDevcontainer:   true,
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

	for _, want := range []string{"Containers 1 of 1", "Details", "Container Type", "Devcontainer", "Project", "/home/me/api", "Image", "golang:1.24", "Volumes", "volume: workspace", "bind: /home/me/api", "Ports", "3000 -> 3000"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("rendered split pane %q does not contain %q", plain, want)
		}
	}
}

func TestMountSummaryListsAttachedVolumes(t *testing.T) {
	got := mountSummary([]domain.Mount{
		{Type: "volume", Name: "workspace", Destination: "/workspaces/api"},
		{Type: "bind", Source: "/home/me/api", Destination: "/src", ReadOnly: true},
	})

	for _, want := range []string{"volume: workspace -> /workspaces/api", "bind: /home/me/api -> /src (read-only)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("mount summary %q does not contain %q", got, want)
		}
	}
	if strings.Contains(got, "attached") {
		t.Fatalf("mount summary should list mounts instead of a count: %q", got)
	}
}

func TestRenderHeaderAndFooterUseSimplifiedLabels(t *testing.T) {
	m := testModel([]domain.Container{
		{ID: "1", ShortID: "111", Name: "api", IsDevcontainer: true},
		{ID: "2", ShortID: "222", Name: "db"},
	})

	header := stripANSI(m.renderHeaderPane())
	footer := stripANSI(m.renderFooterPane())

	for _, unwanted := range []string{"lazydc", "read-only devcontainer viewer"} {
		if strings.Contains(header, unwanted) {
			t.Fatalf("header should not contain %q: %q", unwanted, header)
		}
	}
	for _, unwanted := range []string{"1/2", "global:"} {
		if strings.Contains(footer, unwanted) {
			t.Fatalf("footer should not contain %q: %q", unwanted, footer)
		}
	}
	if !strings.Contains(header, "Status") || !strings.Contains(footer, "Keybindings") {
		t.Fatalf("header/footer missing titles: header=%q footer=%q", header, footer)
	}
	if !strings.Contains(strings.Split(header, "\n")[0], "Status") {
		t.Fatalf("header title should render in top border: %q", header)
	}
	if !strings.Contains(strings.Split(footer, "\n")[0], "Keybindings") {
		t.Fatalf("footer title should render in top border: %q", footer)
	}
}

func TestTabSwitchingShowsTemplatesAndContainers(t *testing.T) {
	m := testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api"}})

	m = updateModel(t, m, runeKey('t'))
	if m.activeTab != tabTemplates {
		t.Fatalf("expected templates tab, got %v", m.activeTab)
	}
	if !strings.Contains(stripANSI(m.renderHeaderPane()), "[Templates]") {
		t.Fatalf("header should show active templates tab: %q", stripANSI(m.renderHeaderPane()))
	}

	m = updateModel(t, m, runeKey('c'))
	if m.activeTab != tabContainers {
		t.Fatalf("expected containers tab, got %v", m.activeTab)
	}
	if !strings.Contains(stripANSI(m.renderHeaderPane()), "[Containers]") {
		t.Fatalf("header should show active containers tab: %q", stripANSI(m.renderHeaderPane()))
	}
}

func TestTemplateSearchFiltersCatalog(t *testing.T) {
	m := testModel(nil)
	m = updateModel(t, m, runeKey('t'))
	m = updateModel(t, m, runeKey('/'))

	for _, r := range "typescript web" {
		m = updateModel(t, m, runeKey(r))
	}

	if m.templateQuery != "typescript web" {
		t.Fatalf("expected template query, got %q", m.templateQuery)
	}
	if len(m.visibleTemplates) != 1 || m.visibleTemplates[0].ID != "node-typescript" {
		t.Fatalf("unexpected visible templates after search: %+v", m.visibleTemplates)
	}
}

func TestTemplateCreateConfirmationForMissingTarget(t *testing.T) {
	m := testModel(nil)
	m.targetDir = t.TempDir()
	m = updateModel(t, m, runeKey('t'))
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.modal != modalConfirmTemplateWrite {
		t.Fatalf("expected template confirmation modal, got %v", m.modal)
	}
	if m.pendingTemplate.ID == "" || m.pendingTemplateOverwrite {
		t.Fatalf("unexpected pending template state: template=%+v overwrite=%v", m.pendingTemplate, m.pendingTemplateOverwrite)
	}
	if got := m.pendingTemplatePath; got != filepath.Join(m.targetDir, ".devcontainer", "devcontainer.json") {
		t.Fatalf("target path = %q, want project devcontainer path", got)
	}
}

func TestTemplateOverwriteConfirmationForExistingTarget(t *testing.T) {
	m := testModel(nil)
	m.targetDir = t.TempDir()
	path := filepath.Join(m.targetDir, ".devcontainer", "devcontainer.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m = updateModel(t, m, runeKey('t'))
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.modal != modalConfirmTemplateWrite || !m.pendingTemplateOverwrite {
		t.Fatalf("expected overwrite confirmation, got modal=%v overwrite=%v", m.modal, m.pendingTemplateOverwrite)
	}
	if !strings.Contains(stripANSI(m.renderConfirmTemplateWriteModal()), "Overwrite existing") {
		t.Fatalf("overwrite modal should warn about replacement: %q", stripANSI(m.renderConfirmTemplateWriteModal()))
	}
}

func TestTemplateWriteCreatesDevcontainerJSON(t *testing.T) {
	m := testModel(nil)
	m.targetDir = t.TempDir()
	m = updateModel(t, m, runeKey('t'))
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	if cmd == nil || !m.templateWriteInProgress || m.modal != modalNone {
		t.Fatalf("expected template write command, modal=%v writing=%v cmd=%v", m.modal, m.templateWriteInProgress, cmd)
	}

	msg := cmd()
	completed, ok := msg.(templateWriteCompletedMsg)
	if !ok {
		t.Fatalf("expected templateWriteCompletedMsg, got %T", msg)
	}
	if completed.err != nil {
		t.Fatalf("unexpected write error: %v", completed.err)
	}

	next, _ = m.Update(completed)
	m, ok = next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	data, err := os.ReadFile(filepath.Join(m.targetDir, ".devcontainer", "devcontainer.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"name": "Base Ubuntu"`) || !strings.Contains(m.actionStatus, "Created") {
		t.Fatalf("unexpected written template or status: status=%q data=%s", m.actionStatus, data)
	}
}

func TestTemplateWriteFailureReportsError(t *testing.T) {
	m := testModel(nil)
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("block"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = updateModel(t, m, runeKey('t'))
	template, ok := m.selectedTemplate()
	if !ok {
		t.Fatal("expected selected template")
	}
	m.modal = modalConfirmTemplateWrite
	m.pendingTemplate = template
	m.pendingTemplatePath = filepath.Join(blocker, ".devcontainer", "devcontainer.json")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok = next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	if cmd == nil {
		t.Fatal("expected template write command")
	}

	completed, ok := cmd().(templateWriteCompletedMsg)
	if !ok {
		t.Fatalf("expected templateWriteCompletedMsg")
	}
	if completed.err == nil {
		t.Fatal("expected write error")
	}

	next, _ = m.Update(completed)
	m, ok = next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	if m.templateWriteInProgress || m.actionErr == nil || m.actionStatus != "Template write failed" {
		t.Fatalf("unexpected failure state: writing=%v status=%q err=%v", m.templateWriteInProgress, m.actionStatus, m.actionErr)
	}
}

func TestConfigTabSwitchingShowsConfig(t *testing.T) {
	m := testModel(nil)
	m.configDoc = devconfig.ConfigDocument{Path: filepath.Join(t.TempDir(), ".devcontainer", "devcontainer.json")}
	m.configLoading = false

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	if m.activeTab != tabConfig {
		t.Fatalf("expected config tab, got %v", m.activeTab)
	}
	if !strings.Contains(stripANSI(m.renderHeaderPane()), "[Config]") {
		t.Fatalf("header should show active config tab: %q", stripANSI(m.renderHeaderPane()))
	}
}

func TestConfigFieldEditMarksDirty(t *testing.T) {
	m := testModel(nil)
	m.configLoading = false
	m.configDoc = devconfig.ConfigDocument{Path: filepath.Join(t.TempDir(), ".devcontainer", "devcontainer.json")}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.modal != modalConfigInput {
		t.Fatalf("expected config input modal, got %v", m.modal)
	}

	for _, r := range "Demo" {
		m = updateModel(t, m, runeKey(r))
	}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.configDirty || m.configDoc.Name != "Demo" {
		t.Fatalf("expected dirty renamed config, dirty=%v name=%q", m.configDirty, m.configDoc.Name)
	}
}

func TestConfigFeatureAddAndRemove(t *testing.T) {
	m := testModel(nil)
	m.configLoading = false
	m.featureCatalogLoading = false
	m.configDoc = devconfig.ConfigDocument{Path: filepath.Join(t.TempDir(), ".devcontainer", "devcontainer.json"), Features: map[string]map[string]any{}}
	m.featureCatalog = []devconfig.Feature{{ID: "ghcr.io/devcontainers/features/go:1", Name: "Go"}}
	m.applyFeatureFilters()
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	m = updateModel(t, m, runeKey('/'))
	if m.modal != modalConfigFeature {
		t.Fatalf("expected feature modal, got %v", m.modal)
	}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.configDirty {
		t.Fatal("expected adding feature to mark config dirty")
	}
	if _, ok := m.configDoc.Features["ghcr.io/devcontainers/features/go:1"]; !ok {
		t.Fatalf("feature was not added: %+v", m.configDoc.Features)
	}

	for index, row := range m.configRows() {
		if row.kind == configRowFeature {
			m.configCursor = index
			break
		}
	}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyDelete})
	if _, ok := m.configDoc.Features["ghcr.io/devcontainers/features/go:1"]; ok {
		t.Fatalf("feature was not removed: %+v", m.configDoc.Features)
	}
}

func TestConfigSaveConfirmationWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".devcontainer", "devcontainer.json")
	m := testModel(nil)
	m.configLoading = false
	m.configDoc = devconfig.ConfigDocument{Path: path, Name: "Demo"}
	m.configDirty = true
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if m.modal != modalConfirmConfigSave {
		t.Fatalf("expected save confirmation, got %v", m.modal)
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	if cmd == nil || !m.configSaving {
		t.Fatalf("expected save command, saving=%v cmd=%v", m.configSaving, cmd)
	}
	completed, ok := cmd().(configSavedMsg)
	if !ok {
		t.Fatalf("expected configSavedMsg")
	}
	if completed.err != nil {
		t.Fatalf("unexpected save error: %v", completed.err)
	}
	m = updateModel(t, m, completed)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.configDirty || !strings.Contains(string(data), `"name": "Demo"`) {
		t.Fatalf("unexpected saved state dirty=%v data=%s", m.configDirty, data)
	}
}

func TestConfigUnsavedChangesPromptBeforeLeaving(t *testing.T) {
	m := testModel(nil)
	m.configLoading = false
	m.configDirty = true
	m.activeTab = tabConfig

	m = updateModel(t, m, runeKey('t'))

	if m.modal != modalConfirmConfigDiscard || m.pendingConfigTab != tabTemplates || m.activeTab != tabConfig {
		t.Fatalf("expected discard confirmation, modal=%v pending=%v active=%v", m.modal, m.pendingConfigTab, m.activeTab)
	}
}

func TestConfigMultipleCandidatesOpenPicker(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, ".devcontainer", "devcontainer.json")
	second := filepath.Join(dir, ".devcontainer", "api", "devcontainer.json")
	m := testModel(nil)
	m.configLoading = false
	m.configCandidatePicked = false
	m.configCandidates = []devconfig.ConfigCandidate{{Path: first, Exists: true}, {Path: second, Exists: true}}
	m.configDoc = devconfig.ConfigDocument{Path: first}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	if m.modal != modalConfigCandidate {
		t.Fatalf("expected config candidate picker, got %v", m.modal)
	}
}

func TestPanesUseFullConfiguredWidth(t *testing.T) {
	m := testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api"}})

	for name, view := range map[string]string{
		"header": m.renderHeaderPane(),
		"main":   m.renderMainPane(),
		"footer": m.renderFooterPane(),
	} {
		if got := lipgloss.Width(view); got != m.width {
			t.Fatalf("%s width = %d, want %d", name, got, m.width)
		}
	}
}

func TestViewFitsConfiguredSize(t *testing.T) {
	m := testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api"}})
	view := m.View()

	if got := lipgloss.Width(view); got != m.width {
		t.Fatalf("view width = %d, want %d", got, m.width)
	}
	if got := lipgloss.Height(view); got != m.height {
		t.Fatalf("view height = %d, want %d", got, m.height)
	}
}

func TestStartStopOpensStopConfirmationForRunningContainer(t *testing.T) {
	m := testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api", State: "running"}})

	m = updateModel(t, m, runeKey('s'))

	if m.modal != modalConfirmAction {
		t.Fatalf("expected confirm modal, got %v", m.modal)
	}
	if m.pendingAction.kind != actionStop || m.pendingAction.containerID != "1" {
		t.Fatalf("unexpected pending action: %+v", m.pendingAction)
	}
	if !strings.Contains(stripANSI(m.renderConfirmActionModal()), "Stop api?") {
		t.Fatalf("confirm modal does not describe stop action: %q", stripANSI(m.renderConfirmActionModal()))
	}
}

func TestStartStopOpensStartConfirmationForStoppedContainer(t *testing.T) {
	m := testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api", State: "exited"}})

	m = updateModel(t, m, runeKey('s'))

	if m.modal != modalConfirmAction {
		t.Fatalf("expected confirm modal, got %v", m.modal)
	}
	if m.pendingAction.kind != actionStart || m.pendingAction.containerID != "1" {
		t.Fatalf("unexpected pending action: %+v", m.pendingAction)
	}
}

func TestRestartOpensConfirmation(t *testing.T) {
	m := testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api", State: "running"}})

	m = updateModel(t, m, runeKey('R'))

	if m.modal != modalConfirmAction {
		t.Fatalf("expected confirm modal, got %v", m.modal)
	}
	if m.pendingAction.kind != actionRestart || m.pendingAction.containerName != "api" {
		t.Fatalf("unexpected pending action: %+v", m.pendingAction)
	}
}

func TestConfirmActionDispatchesCommand(t *testing.T) {
	service := &fakeContainerService{
		containers: []domain.Container{{ID: "1", ShortID: "111", Name: "api", State: "running"}},
	}
	m := testModelWithProvider(service)
	m = updateModel(t, m, runeKey('s'))

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	if cmd == nil {
		t.Fatal("expected action command")
	}
	if m.modal != modalNone || !m.actionInProgress {
		t.Fatalf("expected action to be in progress after confirm: %+v", m)
	}

	msg := cmd()
	completed, ok := msg.(containerActionCompletedMsg)
	if !ok {
		t.Fatalf("expected containerActionCompletedMsg, got %T", msg)
	}
	if completed.err != nil {
		t.Fatalf("unexpected action error: %v", completed.err)
	}
	if service.stopped != "1" {
		t.Fatalf("expected stop to be called for 1, got %q", service.stopped)
	}
}

func TestCancelActionDoesNotDispatchCommand(t *testing.T) {
	m := testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api", State: "running"}})
	m = updateModel(t, m, runeKey('s'))

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	if cmd != nil {
		t.Fatal("expected no command when cancelling action")
	}
	if m.modal != modalNone || m.pendingAction.kind != actionNone {
		t.Fatalf("expected action to be cleared, got modal=%v action=%+v", m.modal, m.pendingAction)
	}
}

func TestSuccessfulActionSetsStatusAndRefreshes(t *testing.T) {
	service := &fakeContainerService{
		containers: []domain.Container{{ID: "1", ShortID: "111", Name: "api", State: "exited"}},
	}
	m := testModelWithProvider(service)
	action := pendingAction{kind: actionStart, containerID: "1", containerName: "api"}
	m.actionInProgress = true

	next, cmd := m.Update(containerActionCompletedMsg{action: action})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	if m.actionInProgress || m.actionErr != nil || m.actionStatus != "Started api" {
		t.Fatalf("unexpected action state after success: status=%q err=%v running=%v", m.actionStatus, m.actionErr, m.actionInProgress)
	}
	if cmd == nil {
		t.Fatal("expected refresh command after successful action")
	}
}

func TestFailedActionRendersErrorStatus(t *testing.T) {
	m := testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api", State: "running"}})
	action := pendingAction{kind: actionStop, containerID: "1", containerName: "api"}
	m.actionInProgress = true

	next, cmd := m.Update(containerActionCompletedMsg{action: action, err: errors.New("boom")})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("expected tui.Model, got %T", next)
	}
	if cmd != nil {
		t.Fatal("expected no refresh command after failed action")
	}
	header := stripANSI(m.renderHeaderPane())
	if !strings.Contains(header, "Stopping failed for api") || !strings.Contains(header, "boom") {
		t.Fatalf("header missing action error: %q", header)
	}
}

func TestShellAndEditorErrorsWithoutUsableSelection(t *testing.T) {
	m := testModel(nil)
	m = updateModel(t, m, runeKey('x'))
	if m.actionErr == nil || !strings.Contains(m.actionStatus, "No container selected") {
		t.Fatalf("expected shell selection error, got status=%q err=%v", m.actionStatus, m.actionErr)
	}

	m = testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api", State: "running"}})
	m = updateModel(t, m, runeKey('e'))
	if m.actionErr == nil || !strings.Contains(m.actionStatus, "no detected workspace path") {
		t.Fatalf("expected editor path error, got status=%q err=%v", m.actionStatus, m.actionErr)
	}

	t.Setenv("EDITOR", "")
	m = testModel([]domain.Container{{ID: "1", ShortID: "111", Name: "api", State: "running", DevcontainerPath: "/workspaces/api"}})
	m = updateModel(t, m, runeKey('e'))
	if m.actionErr == nil || !strings.Contains(m.actionStatus, "EDITOR is not set") {
		t.Fatalf("expected editor environment error, got status=%q err=%v", m.actionStatus, m.actionErr)
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

func testModelWithProvider(provider *fakeContainerService) Model {
	m := New(provider)
	m.width = 100
	m.height = 30
	m.loading = false
	m.containers = provider.containers
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

type fakeContainerService struct {
	containers []domain.Container
	started    string
	stopped    string
	restarted  string
	err        error
}

func (f *fakeContainerService) ListContainers(context.Context) ([]domain.Container, error) {
	return f.containers, f.err
}

func (f *fakeContainerService) StartContainer(_ context.Context, id string) error {
	f.started = id
	return f.err
}

func (f *fakeContainerService) StopContainer(_ context.Context, id string) error {
	f.stopped = id
	return f.err
}

func (f *fakeContainerService) RestartContainer(_ context.Context, id string) error {
	f.restarted = id
	return f.err
}
