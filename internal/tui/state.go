package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	devconfig "github.com/Souheab/lazydevcontainer/internal/config"
	"github.com/Souheab/lazydevcontainer/internal/domain"
	containerfilter "github.com/Souheab/lazydevcontainer/internal/filter"
	devtemplates "github.com/Souheab/lazydevcontainer/internal/templates"
)

func (m *Model) setFilter(mode containerfilter.Mode) {
	m.filterMode = mode
	m.applyFilters()
	m.ensureCursorBounds()
	m.ensureCursorVisible()
}

func (m Model) switchTab(tab activeTab) Model {
	if m.activeTab == tab {
		return m
	}
	if m.activeTab == tabConfig && m.configDirty {
		m.pendingConfigTab = tab
		m.modal = modalConfirmConfigDiscard
		return m
	}
	m.activeTab = tab
	m.actionErr = nil
	switch tab {
	case tabContainers:
		m.ensureCursorBounds()
		m.ensureCursorVisible()
	case tabTemplates:
		m.ensureTemplateCursorBounds()
		m.ensureTemplateCursorVisible()
	case tabConfig:
		m.ensureConfigCursorBounds()
		m.ensureConfigCursorVisible()
		if len(m.configCandidates) > 1 && !m.configCandidatePicked {
			m.configCandidateCursor = 0
			m.modal = modalConfigCandidate
		}
	}
	return m
}

func (m *Model) openStartStopConfirmation() {
	container, ok := m.selectedContainer()
	if !ok {
		m.setActionError("No container selected")
		return
	}

	kind := actionStart
	if containerIsRunning(container) {
		kind = actionStop
	}
	m.pendingAction = pendingAction{
		kind:          kind,
		containerID:   container.ID,
		containerName: container.DisplayName(),
	}
	m.modal = modalConfirmAction
	m.actionErr = nil
}

func (m *Model) openRestartConfirmation() {
	container, ok := m.selectedContainer()
	if !ok {
		m.setActionError("No container selected")
		return
	}

	m.pendingAction = pendingAction{
		kind:          actionRestart,
		containerID:   container.ID,
		containerName: container.DisplayName(),
	}
	m.modal = modalConfirmAction
	m.actionErr = nil
}

func (m *Model) openTemplateWriteConfirmation() {
	template, ok := m.selectedTemplate()
	if !ok {
		m.setActionError("No template selected")
		return
	}

	path := m.templateTargetPath()
	_, err := os.Stat(path)
	overwrite := false
	if err == nil {
		overwrite = true
	} else if !errors.Is(err, os.ErrNotExist) {
		m.setActionError(fmt.Sprintf("Could not inspect %s", path))
		m.actionErr = err
		return
	}

	m.pendingTemplate = template
	m.pendingTemplatePath = path
	m.pendingTemplateOverwrite = overwrite
	m.modal = modalConfirmTemplateWrite
	m.actionErr = nil
}

func (m Model) openShell() (tea.Model, tea.Cmd) {
	container, ok := m.selectedContainer()
	if !ok {
		m.setActionError("No container selected")
		return m, nil
	}
	if container.ID == "" {
		m.setActionError("Selected container has no ID")
		return m, nil
	}

	m.actionStatus = fmt.Sprintf("Opening shell in %s", container.DisplayName())
	m.actionErr = nil
	cmd := exec.Command("docker", "exec", "-it", container.ID, "sh", "-lc", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi")
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return externalCommandFinishedMsg{kind: "shell", err: err}
	})
}

func (m Model) openEditor() (tea.Model, tea.Cmd) {
	container, ok := m.selectedContainer()
	if !ok {
		m.setActionError("No container selected")
		return m, nil
	}
	if strings.TrimSpace(container.DevcontainerPath) == "" {
		m.setActionError("Selected container has no detected workspace path")
		return m, nil
	}

	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		m.setActionError("EDITOR is not set")
		return m, nil
	}

	m.actionStatus = fmt.Sprintf("Opening %s in editor", container.DisplayName())
	m.actionErr = nil
	cmd := editorCommand(editor, container.DevcontainerPath)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return externalCommandFinishedMsg{kind: "editor", err: err}
	})
}

func editorCommand(editor string, path string) *exec.Cmd {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return exec.Command(editor, path)
	}
	args := append(parts[1:], path)
	return exec.Command(parts[0], args...)
}

func (m *Model) setActionError(message string) {
	m.actionStatus = message
	m.actionErr = errors.New(message)
}

func (m *Model) openConfigEditor() {
	row, ok := m.selectedConfigRow()
	if !ok {
		m.setActionError("No config field selected")
		return
	}

	switch row.kind {
	case configRowAddFeature:
		m.openFeaturePicker()
	case configRowAddExtension:
		m.configEditExtensionIndex = -1
		m.openConfigInput(configEditAddExtension, "", "publisher.extension")
	case configRowFeature:
		m.configEditFeatureID = row.id
		m.openConfigInput(configEditFeatureOptions, featureOptionsString(m.configDoc.Features[row.id]), "version=latest, option=true")
	case configRowExtension:
		m.configEditExtensionIndex = row.index
		m.openConfigInput(configEditAddExtension, row.value, "publisher.extension")
	case configRowField:
		m.openConfigInput(row.edit, row.value, "value")
	}
}

func (m *Model) openConfigInput(kind configEditKind, value string, placeholder string) {
	m.configEdit = kind
	m.configInput.SetValue(value)
	m.configInput.Placeholder = placeholder
	m.configInput.Focus()
	m.modal = modalConfigInput
	m.actionErr = nil
}

func (m *Model) openFeaturePicker() {
	m.featureSearchInput.SetValue("")
	m.featureSearchInput.Focus()
	m.featureQuery = ""
	m.applyFeatureFilters()
	m.ensureFeatureCursorBounds()
	m.ensureFeatureCursorVisible()
	m.modal = modalConfigFeature
	m.actionErr = nil
}

func (m *Model) openConfigSaveConfirmation() {
	if strings.TrimSpace(m.configDoc.Path) == "" {
		m.setActionError("No devcontainer path selected")
		return
	}
	m.modal = modalConfirmConfigSave
	m.actionErr = nil
}

func (m *Model) applyConfigInputValue(value string) {
	value = strings.TrimSpace(value)
	switch m.configEdit {
	case configEditName:
		m.configDoc.Name = value
	case configEditImage:
		m.configDoc.Image = value
	case configEditRemoteUser:
		m.configDoc.RemoteUser = value
	case configEditPostCreate:
		m.configDoc.PostCreateCommand = value
	case configEditAddExtension:
		if m.configEditExtensionIndex >= 0 && m.configEditExtensionIndex < len(m.configDoc.Extensions) {
			if value == "" {
				m.configDoc.Extensions = append(m.configDoc.Extensions[:m.configEditExtensionIndex], m.configDoc.Extensions[m.configEditExtensionIndex+1:]...)
			} else {
				m.configDoc.Extensions[m.configEditExtensionIndex] = value
			}
		} else if value != "" {
			m.configDoc.Extensions = append(m.configDoc.Extensions, value)
		}
	case configEditFeatureOptions:
		if m.configDoc.Features == nil {
			m.configDoc.Features = map[string]map[string]any{}
		}
		m.configDoc.Features[m.configEditFeatureID] = parseFeatureOptions(value)
	case configEditAddFeatureManual:
		m.addFeatureID(value)
	default:
		return
	}
	m.configDirty = true
	m.actionStatus = "Config changed"
	m.actionErr = nil
}

func (m *Model) addSelectedFeature() {
	query := strings.TrimSpace(m.featureSearchInput.Value())
	if len(m.visibleFeatures) > 0 && m.featureCursor >= 0 && m.featureCursor < len(m.visibleFeatures) {
		m.addFeatureID(m.visibleFeatures[m.featureCursor].ID)
		return
	}
	m.addFeatureID(query)
}

func (m *Model) addFeatureID(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		m.setActionError("Enter a feature ID")
		return
	}
	if m.configDoc.Features == nil {
		m.configDoc.Features = map[string]map[string]any{}
	}
	if _, exists := m.configDoc.Features[id]; !exists {
		m.configDoc.Features[id] = map[string]any{}
	}
	m.configDirty = true
	m.actionStatus = fmt.Sprintf("Added feature %s", id)
	m.actionErr = nil
}

func (m *Model) removeSelectedConfigItem() {
	row, ok := m.selectedConfigRow()
	if !ok {
		return
	}
	switch row.kind {
	case configRowFeature:
		delete(m.configDoc.Features, row.id)
	case configRowExtension:
		if row.index >= 0 && row.index < len(m.configDoc.Extensions) {
			m.configDoc.Extensions = append(m.configDoc.Extensions[:row.index], m.configDoc.Extensions[row.index+1:]...)
		}
	default:
		m.setActionError("Selected config item cannot be removed")
		return
	}
	m.configDirty = true
	m.actionStatus = "Config changed"
	m.actionErr = nil
	m.ensureConfigCursorBounds()
	m.ensureConfigCursorVisible()
}

func (m Model) selectedConfigRow() (configRow, bool) {
	rows := m.configRows()
	if m.configCursor < 0 || m.configCursor >= len(rows) {
		return configRow{}, false
	}
	return rows[m.configCursor], true
}

func (m Model) configRows() []configRow {
	rows := []configRow{
		{kind: configRowField, edit: configEditName, label: "Name", value: m.configDoc.Name},
		{kind: configRowField, edit: configEditImage, label: "Base image", value: m.configDoc.Image},
		{kind: configRowField, edit: configEditRemoteUser, label: "Remote user", value: m.configDoc.RemoteUser},
		{kind: configRowField, edit: configEditPostCreate, label: "Post create", value: m.configDoc.PostCreateCommand},
		{kind: configRowAddFeature, label: "+ Add feature", value: "Browse catalog or type a feature ID"},
	}

	featureIDs := make([]string, 0, len(m.configDoc.Features))
	for id := range m.configDoc.Features {
		featureIDs = append(featureIDs, id)
	}
	sort.Strings(featureIDs)
	for _, id := range featureIDs {
		rows = append(rows, configRow{kind: configRowFeature, label: "Feature", value: id, id: id})
	}

	rows = append(rows, configRow{kind: configRowAddExtension, label: "+ Add extension", value: "VS Code extension ID"})
	for index, extension := range m.configDoc.Extensions {
		rows = append(rows, configRow{kind: configRowExtension, label: "Extension", value: extension, index: index})
	}
	return rows
}

func (m Model) selectedContainer() (domain.Container, bool) {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return domain.Container{}, false
	}
	return m.visible[m.cursor], true
}

func containerIsRunning(container domain.Container) bool {
	return strings.EqualFold(container.State, "running") || strings.HasPrefix(strings.ToLower(container.Status), "up ")
}

func actionVerb(kind actionKind) string {
	switch kind {
	case actionStart:
		return "Starting"
	case actionStop:
		return "Stopping"
	case actionRestart:
		return "Restarting"
	default:
		return "Running action for"
	}
}

func actionPastTense(kind actionKind) string {
	switch kind {
	case actionStart:
		return "Started"
	case actionStop:
		return "Stopped"
	case actionRestart:
		return "Restarted"
	default:
		return "Updated"
	}
}

func actionPrompt(kind actionKind) string {
	switch kind {
	case actionStart:
		return "Start"
	case actionStop:
		return "Stop"
	case actionRestart:
		return "Restart"
	default:
		return "Run action for"
	}
}

func parseFeatureOptions(value string) map[string]any {
	options := map[string]any{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, rawValue, ok := strings.Cut(part, "=")
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)
		if !ok || key == "" {
			continue
		}
		switch strings.ToLower(rawValue) {
		case "true":
			options[key] = true
		case "false":
			options[key] = false
		default:
			options[key] = rawValue
		}
	}
	return options
}

func featureOptionsString(options map[string]any) string {
	if len(options) == 0 {
		return ""
	}
	keys := make([]string, 0, len(options))
	for key := range options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, options[key]))
	}
	return strings.Join(parts, ", ")
}

func (m *Model) applyFilters() {
	m.visible = containerfilter.Apply(m.containers, m.filterMode, m.query)
}

func (m *Model) applyTemplateFilters() {
	m.visibleTemplates = devtemplates.Filter(m.templates, m.templateQuery)
}

func (m *Model) applyFeatureFilters() {
	m.visibleFeatures = devconfig.SearchFeatures(m.featureCatalog, m.featureQuery)
}

func (m *Model) moveCursor(delta int) {
	if len(m.visible) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}

	m.cursor += delta
	m.ensureCursorBounds()
	m.ensureCursorVisible()
}

func (m *Model) moveTemplateCursor(delta int) {
	if len(m.visibleTemplates) == 0 {
		m.templateCursor = 0
		m.templateOffset = 0
		return
	}

	m.templateCursor += delta
	m.ensureTemplateCursorBounds()
	m.ensureTemplateCursorVisible()
}

func (m *Model) moveConfigCursor(delta int) {
	if len(m.configRows()) == 0 {
		m.configCursor = 0
		m.configOffset = 0
		return
	}

	m.configCursor += delta
	m.ensureConfigCursorBounds()
	m.ensureConfigCursorVisible()
}

func (m *Model) moveFeatureCursor(delta int) {
	if len(m.visibleFeatures) == 0 {
		m.featureCursor = 0
		m.featureOffset = 0
		return
	}

	m.featureCursor += delta
	m.ensureFeatureCursorBounds()
	m.ensureFeatureCursorVisible()
}

func (m *Model) ensureCursorBounds() {
	if len(m.visible) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
}

func (m *Model) ensureTemplateCursorBounds() {
	if len(m.visibleTemplates) == 0 {
		m.templateCursor = 0
		m.templateOffset = 0
		return
	}
	if m.templateCursor < 0 {
		m.templateCursor = 0
	}
	if m.templateCursor >= len(m.visibleTemplates) {
		m.templateCursor = len(m.visibleTemplates) - 1
	}
}

func (m *Model) ensureConfigCursorBounds() {
	rows := m.configRows()
	if len(rows) == 0 {
		m.configCursor = 0
		m.configOffset = 0
		return
	}
	if m.configCursor < 0 {
		m.configCursor = 0
	}
	if m.configCursor >= len(rows) {
		m.configCursor = len(rows) - 1
	}
}

func (m *Model) ensureFeatureCursorBounds() {
	if len(m.visibleFeatures) == 0 {
		m.featureCursor = 0
		m.featureOffset = 0
		return
	}
	if m.featureCursor < 0 {
		m.featureCursor = 0
	}
	if m.featureCursor >= len(m.visibleFeatures) {
		m.featureCursor = len(m.visibleFeatures) - 1
	}
}

func (m *Model) ensureCursorVisible() {
	rows := m.visibleRowCount()
	if rows <= 0 {
		rows = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if m.offset < 0 || len(m.visible) == 0 {
		m.offset = 0
	}
}

func (m *Model) ensureTemplateCursorVisible() {
	rows := m.visibleRowCount()
	if rows <= 0 {
		rows = 1
	}
	if m.templateCursor < m.templateOffset {
		m.templateOffset = m.templateCursor
	}
	if m.templateCursor >= m.templateOffset+rows {
		m.templateOffset = m.templateCursor - rows + 1
	}
	if m.templateOffset < 0 || len(m.visibleTemplates) == 0 {
		m.templateOffset = 0
	}
}

func (m *Model) ensureConfigCursorVisible() {
	rows := m.visibleRowCount()
	if rows <= 0 {
		rows = 1
	}
	if m.configCursor < m.configOffset {
		m.configOffset = m.configCursor
	}
	if m.configCursor >= m.configOffset+rows {
		m.configOffset = m.configCursor - rows + 1
	}
	if m.configOffset < 0 || len(m.configRows()) == 0 {
		m.configOffset = 0
	}
}

func (m *Model) ensureFeatureCursorVisible() {
	rows := max(1, m.visibleRowCount())
	if m.featureCursor < m.featureOffset {
		m.featureOffset = m.featureCursor
	}
	if m.featureCursor >= m.featureOffset+rows {
		m.featureOffset = m.featureCursor - rows + 1
	}
	if m.featureOffset < 0 || len(m.visibleFeatures) == 0 {
		m.featureOffset = 0
	}
}

func (m Model) visibleRowCount() int {
	rows := containerContentHeight(m.height, m.help.ShowAll) / rowHeight
	if rows < 1 {
		return 1
	}
	return rows
}

func footerContentHeight(showAll bool) int {
	if showAll {
		return fullFooterHeight
	}
	return shortFooterHeight
}

func headerPaneHeight() int {
	return headerContentHeight + 2
}

func footerPaneHeight(showAll bool) int {
	return footerContentHeight(showAll) + 2
}

func containerPaneHeight(height int, showAll bool) int {
	available := height - headerPaneHeight() - footerPaneHeight(showAll) - 2
	if available < 3 {
		return 3
	}
	return available
}

func containerContentHeight(height int, showAll bool) int {
	return max(1, containerPaneHeight(height, showAll)-2)
}

func (m Model) selectedID() string {
	if m.cursor < 0 || m.cursor >= len(m.visible) {
		return ""
	}
	return m.visible[m.cursor].ID
}

func (m Model) selectedTemplate() (devtemplates.Template, bool) {
	if m.templateCursor < 0 || m.templateCursor >= len(m.visibleTemplates) {
		return devtemplates.Template{}, false
	}
	return m.visibleTemplates[m.templateCursor], true
}

func (m Model) templateTargetPath() string {
	return filepath.Join(m.targetDir, ".devcontainer", "devcontainer.json")
}

func (m *Model) selectID(id string) {
	if id == "" {
		return
	}
	for index, container := range m.visible {
		if container.ID == id {
			m.cursor = index
			return
		}
	}
}
