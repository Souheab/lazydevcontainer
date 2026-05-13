package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	containerfilter "github.com/Souheab/lazydevcontainer/internal/filter"
	devtemplates "github.com/Souheab/lazydevcontainer/internal/templates"
)

// Update handles keyboard, mouse, resize, and load messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.searchInput.Width = max(10, min(56, msg.Width-12))
		m.templateSearchInput.Width = max(10, min(56, msg.Width-12))
		m.configInput.Width = max(10, min(64, msg.Width-12))
		m.featureSearchInput.Width = max(10, min(72, msg.Width-12))
		m.ensureCursorVisible()
		m.ensureTemplateCursorVisible()
		m.ensureConfigCursorVisible()
		m.ensureFeatureCursorVisible()
		return m, nil

	case containersLoadedMsg:
		selectedID := m.selectedID()
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.containers = msg.containers
		}
		m.applyFilters()
		m.selectID(selectedID)
		m.ensureCursorBounds()
		m.ensureCursorVisible()
		return m, nil

	case containerActionCompletedMsg:
		m.actionInProgress = false
		m.actionErr = friendlyDockerError(msg.err)
		if m.actionErr != nil {
			m.actionStatus = fmt.Sprintf("%s failed for %s", actionVerb(msg.action.kind), msg.action.containerName)
			return m, nil
		}
		m.actionStatus = fmt.Sprintf("%s %s", actionPastTense(msg.action.kind), msg.action.containerName)
		m.loading = true
		m.err = nil
		return m, loadContainers(m.provider)

	case externalCommandFinishedMsg:
		m.actionErr = msg.err
		if msg.err != nil {
			m.actionStatus = fmt.Sprintf("%s failed", msg.kind)
			return m, nil
		}
		m.actionStatus = fmt.Sprintf("%s finished", msg.kind)
		if msg.kind == "shell" {
			m.loading = true
			m.err = nil
			return m, loadContainers(m.provider)
		}
		return m, nil

	case templateWriteCompletedMsg:
		m.templateWriteInProgress = false
		m.actionErr = msg.err
		if msg.err != nil {
			m.actionStatus = "Template write failed"
			return m, nil
		}
		m.actionStatus = fmt.Sprintf("Created %s", msg.path)
		return m, nil

	case configLoadedMsg:
		m.configLoading = false
		m.configErr = msg.err
		if msg.err == nil {
			m.configCandidates = msg.candidates
			m.configDoc = msg.document
			m.configDirty = false
		}
		if len(m.configCandidates) <= 1 {
			m.configCandidatePicked = true
		}
		m.ensureConfigCursorBounds()
		m.ensureConfigCursorVisible()
		return m, nil

	case configSavedMsg:
		m.configSaving = false
		m.actionErr = msg.err
		if msg.err != nil {
			m.actionStatus = "Config save failed"
			return m, nil
		}
		m.configDirty = false
		m.actionStatus = fmt.Sprintf("Saved %s", msg.path)
		return m, nil

	case featureCatalogLoadedMsg:
		m.featureCatalogLoading = false
		if msg.err != nil {
			m.actionStatus = "Feature catalog unavailable; manual feature IDs still work"
		}
		m.featureCatalog = msg.features
		m.applyFeatureFilters()
		m.ensureFeatureCursorBounds()
		m.ensureFeatureCursorVisible()
		return m, nil

	case tea.KeyMsg:
		if m.modal == modalSearch {
			return m.updateSearch(msg)
		}
		if m.modal == modalFilter {
			return m.updateFilterModal(msg)
		}
		if m.modal == modalConfirmAction {
			return m.updateConfirmAction(msg)
		}
		if m.modal == modalConfirmTemplateWrite {
			return m.updateConfirmTemplateWrite(msg)
		}
		if m.modal == modalConfigInput {
			return m.updateConfigInput(msg)
		}
		if m.modal == modalConfigFeature {
			return m.updateConfigFeatureModal(msg)
		}
		if m.modal == modalConfigCandidate {
			return m.updateConfigCandidateModal(msg)
		}
		if m.modal == modalConfirmConfigSave {
			return m.updateConfirmConfigSave(msg)
		}
		if m.modal == modalConfirmConfigDiscard {
			return m.updateConfirmConfigDiscard(msg)
		}
		return m.updateKey(msg)

	case tea.MouseMsg:
		return m.updateMouse(msg)
	}

	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		m.searchInput.Blur()
		m.templateSearchInput.Blur()
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		m.modal = modalNone
		m.searchInput.Blur()
		m.templateSearchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	if m.activeTab == tabTemplates {
		m.templateSearchInput, cmd = m.templateSearchInput.Update(msg)
		m.templateQuery = m.templateSearchInput.Value()
		m.applyTemplateFilters()
		m.ensureTemplateCursorBounds()
		m.ensureTemplateCursorVisible()
		return m, cmd
	}

	m.searchInput, cmd = m.searchInput.Update(msg)
	m.query = m.searchInput.Value()
	m.applyFilters()
	m.ensureCursorBounds()
	m.ensureCursorVisible()
	return m, cmd
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.TemplatesTab) {
		return m.switchTab(tabTemplates), nil
	}
	if key.Matches(msg, m.keys.ConfigTab) {
		return m.switchTab(tabConfig), nil
	}
	if key.Matches(msg, m.keys.ContainersTab) {
		return m.switchTab(tabContainers), nil
	}
	if m.activeTab == tabConfig {
		return m.updateConfigKey(msg)
	}
	if m.activeTab == tabTemplates {
		return m.updateTemplateKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case key.Matches(msg, m.keys.Refresh):
		m.loading = true
		m.err = nil
		m.actionErr = nil
		m.actionStatus = "Refreshing containers"
		return m, loadContainers(m.provider)
	case key.Matches(msg, m.keys.StartStop):
		m.openStartStopConfirmation()
		return m, nil
	case key.Matches(msg, m.keys.Restart):
		m.openRestartConfirmation()
		return m, nil
	case key.Matches(msg, m.keys.Shell):
		return m.openShell()
	case key.Matches(msg, m.keys.Editor):
		return m.openEditor()
	case key.Matches(msg, m.keys.Search):
		m.modal = modalSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case key.Matches(msg, m.keys.FilterMenu):
		m.modal = modalFilter
		m.filterCursor = filterIndex(m.filterMode)
		return m, nil
	case key.Matches(msg, m.keys.Cancel):
		if m.query != "" {
			m.query = ""
			m.searchInput.SetValue("")
			m.applyFilters()
			m.ensureCursorBounds()
			m.ensureCursorVisible()
		}
		return m, nil
	case key.Matches(msg, m.keys.FilterAll):
		m.setFilter(containerfilter.ModeAll)
		return m, nil
	case key.Matches(msg, m.keys.FilterDev):
		m.setFilter(containerfilter.ModeDevcontainers)
		return m, nil
	case key.Matches(msg, m.keys.FilterDocker):
		m.setFilter(containerfilter.ModeContainers)
		return m, nil
	case key.Matches(msg, m.keys.CycleFilter), key.Matches(msg, m.keys.Right):
		m.setFilter(m.filterMode.Next())
		return m, nil
	case key.Matches(msg, m.keys.ReverseFilter), key.Matches(msg, m.keys.Left):
		m.setFilter(m.filterMode.Previous())
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
		return m, nil
	case key.Matches(msg, m.keys.PageUp):
		m.moveCursor(-m.visibleRowCount())
		return m, nil
	case key.Matches(msg, m.keys.PageDown):
		m.moveCursor(m.visibleRowCount())
		return m, nil
	case key.Matches(msg, m.keys.Home), key.Matches(msg, m.keys.Top):
		m.cursor = 0
		m.ensureCursorVisible()
		return m, nil
	case key.Matches(msg, m.keys.End), key.Matches(msg, m.keys.Bottom):
		m.cursor = len(m.visible) - 1
		m.ensureCursorBounds()
		m.ensureCursorVisible()
		return m, nil
	}

	return m, nil
}

func (m Model) updateTemplateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case key.Matches(msg, m.keys.Search):
		m.modal = modalSearch
		m.templateSearchInput.Focus()
		return m, textinput.Blink
	case key.Matches(msg, m.keys.Confirm):
		m.openTemplateWriteConfirmation()
		return m, nil
	case key.Matches(msg, m.keys.Cancel):
		if m.templateQuery != "" {
			m.templateQuery = ""
			m.templateSearchInput.SetValue("")
			m.applyTemplateFilters()
			m.ensureTemplateCursorBounds()
			m.ensureTemplateCursorVisible()
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.moveTemplateCursor(-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.moveTemplateCursor(1)
		return m, nil
	case key.Matches(msg, m.keys.PageUp):
		m.moveTemplateCursor(-m.visibleRowCount())
		return m, nil
	case key.Matches(msg, m.keys.PageDown):
		m.moveTemplateCursor(m.visibleRowCount())
		return m, nil
	case key.Matches(msg, m.keys.Home), key.Matches(msg, m.keys.Top):
		m.templateCursor = 0
		m.ensureTemplateCursorVisible()
		return m, nil
	case key.Matches(msg, m.keys.End), key.Matches(msg, m.keys.Bottom):
		m.templateCursor = len(m.visibleTemplates) - 1
		m.ensureTemplateCursorBounds()
		m.ensureTemplateCursorVisible()
		return m, nil
	case key.Matches(msg, m.keys.Refresh), key.Matches(msg, m.keys.StartStop), key.Matches(msg, m.keys.Restart), key.Matches(msg, m.keys.Shell), key.Matches(msg, m.keys.Editor), key.Matches(msg, m.keys.FilterMenu):
		m.setActionError("Switch to Containers for that action")
		return m, nil
	}

	return m, nil
}

func (m Model) updateConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		if m.configDirty {
			m.pendingConfigTab = tabConfig
			m.modal = modalConfirmConfigDiscard
			return m, nil
		}
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case key.Matches(msg, m.keys.Search):
		m.openFeaturePicker()
		return m, textinput.Blink
	case key.Matches(msg, m.keys.Save):
		m.openConfigSaveConfirmation()
		return m, nil
	case key.Matches(msg, m.keys.Delete):
		m.removeSelectedConfigItem()
		return m, nil
	case key.Matches(msg, m.keys.Refresh):
		m.configLoading = true
		m.configErr = nil
		m.actionStatus = "Reloading config"
		return m, loadConfig(m.targetDir)
	case key.Matches(msg, m.keys.Confirm):
		m.openConfigEditor()
		return m, textinput.Blink
	case key.Matches(msg, m.keys.Cancel):
		m.actionErr = nil
		m.actionStatus = ""
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.moveConfigCursor(-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.moveConfigCursor(1)
		return m, nil
	case key.Matches(msg, m.keys.PageUp):
		m.moveConfigCursor(-m.visibleRowCount())
		return m, nil
	case key.Matches(msg, m.keys.PageDown):
		m.moveConfigCursor(m.visibleRowCount())
		return m, nil
	case key.Matches(msg, m.keys.Home), key.Matches(msg, m.keys.Top):
		m.configCursor = 0
		m.ensureConfigCursorVisible()
		return m, nil
	case key.Matches(msg, m.keys.End), key.Matches(msg, m.keys.Bottom):
		m.configCursor = len(m.configRows()) - 1
		m.ensureConfigCursorBounds()
		m.ensureConfigCursorVisible()
		return m, nil
	case key.Matches(msg, m.keys.StartStop), key.Matches(msg, m.keys.Restart), key.Matches(msg, m.keys.Shell), key.Matches(msg, m.keys.Editor), key.Matches(msg, m.keys.FilterMenu):
		m.setActionError("Switch to Containers for that action")
		return m, nil
	}

	return m, nil
}

func (m Model) updateConfirmAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel), key.Matches(msg, m.keys.Confirm), isNoKey(msg):
		m.modal = modalNone
		m.pendingAction = pendingAction{}
		m.actionStatus = "Action cancelled"
		m.actionErr = nil
		return m, nil
	case isYesKey(msg):
		action := m.pendingAction
		m.modal = modalNone
		m.pendingAction = pendingAction{}
		m.actionInProgress = true
		m.actionStatus = fmt.Sprintf("%s %s", actionVerb(action.kind), action.containerName)
		m.actionErr = nil
		return m, runContainerAction(m.provider, action)
	}

	return m, nil
}

func (m Model) updateConfirmTemplateWrite(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel), key.Matches(msg, m.keys.Confirm), isNoKey(msg):
		m.modal = modalNone
		m.pendingTemplate = devtemplates.Template{}
		m.pendingTemplatePath = ""
		m.pendingTemplateOverwrite = false
		m.actionStatus = "Template write cancelled"
		m.actionErr = nil
		return m, nil
	case isYesKey(msg):
		template := m.pendingTemplate
		path := m.pendingTemplatePath
		m.modal = modalNone
		m.pendingTemplate = devtemplates.Template{}
		m.pendingTemplatePath = ""
		m.pendingTemplateOverwrite = false
		m.templateWriteInProgress = true
		m.actionStatus = fmt.Sprintf("Writing %s", path)
		m.actionErr = nil
		return m, writeTemplate(template, path)
	}

	return m, nil
}

func (m Model) updateConfigInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		if m.configInputReturnFeature {
			m.modal = modalConfigFeature
			m.featureSearchInput.Focus()
		}
		m.configInput.Blur()
		m.configEdit = configEditNone
		m.configEditFeatureID = ""
		m.configEditExtensionIndex = -1
		m.configInputReturnFeature = false
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		m.applyConfigInputValue(m.configInput.Value())
		m.modal = modalNone
		if m.configInputReturnFeature {
			m.modal = modalConfigFeature
			m.featureSearchInput.Focus()
		}
		m.configInput.Blur()
		m.configEdit = configEditNone
		m.configEditFeatureID = ""
		m.configEditExtensionIndex = -1
		m.configInputReturnFeature = false
		m.ensureConfigCursorBounds()
		m.ensureConfigCursorVisible()
		m.ensureFeatureCursorBounds()
		m.ensureFeatureCursorVisible()
		return m, nil
	}

	var cmd tea.Cmd
	m.configInput, cmd = m.configInput.Update(msg)
	return m, cmd
}

func (m Model) updateConfigFeatureModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		m.featureSearchInput.Blur()
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.moveFeatureCursor(-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.moveFeatureCursor(1)
		return m, nil
	case key.Matches(msg, m.keys.PageUp):
		m.moveFeatureCursor(-5)
		return m, nil
	case key.Matches(msg, m.keys.PageDown):
		m.moveFeatureCursor(5)
		return m, nil
	case key.Matches(msg, m.keys.Delete):
		items := m.featureModalItems()
		if m.featureCursor < 0 || m.featureCursor >= len(items) || items[m.featureCursor].kind != configFeatureModalConfigured {
			m.setActionError("Select a configured feature to remove")
			return m, nil
		}
		delete(m.configDoc.Features, items[m.featureCursor].id)
		m.configDirty = true
		m.actionStatus = "Config changed"
		m.actionErr = nil
		m.ensureFeatureCursorBounds()
		m.ensureFeatureCursorVisible()
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		items := m.featureModalItems()
		if m.featureCursor >= 0 && m.featureCursor < len(items) {
			item := items[m.featureCursor]
			switch item.kind {
			case configFeatureModalConfigured:
				m.openFeatureOptionsInput(item.id, true)
				return m, textinput.Blink
			case configFeatureModalManual, configFeatureModalCatalog:
				m.addFeatureID(item.id)
				m.ensureFeatureCursorBounds()
				m.ensureFeatureCursorVisible()
				return m, nil
			}
		}
		m.addFeatureID(m.featureSearchInput.Value())
		m.ensureFeatureCursorBounds()
		m.ensureFeatureCursorVisible()
		return m, nil
	}

	var cmd tea.Cmd
	m.featureSearchInput, cmd = m.featureSearchInput.Update(msg)
	m.featureQuery = m.featureSearchInput.Value()
	m.applyFeatureFilters()
	m.ensureFeatureCursorBounds()
	m.ensureFeatureCursorVisible()
	return m, cmd
}

func (m Model) updateConfigCandidateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		m.configCandidatePicked = true
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if len(m.configCandidates) == 0 {
			return m, nil
		}
		m.configCandidateCursor = (m.configCandidateCursor + len(m.configCandidates) - 1) % len(m.configCandidates)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if len(m.configCandidates) == 0 {
			return m, nil
		}
		m.configCandidateCursor = (m.configCandidateCursor + 1) % len(m.configCandidates)
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		if len(m.configCandidates) == 0 {
			m.modal = modalNone
			return m, nil
		}
		if m.configCandidateCursor < 0 || m.configCandidateCursor >= len(m.configCandidates) {
			m.configCandidateCursor = 0
		}
		m.modal = modalNone
		m.configCandidatePicked = true
		m.configLoading = true
		m.actionStatus = "Loading config"
		return m, loadConfigPath(m.configCandidates[m.configCandidateCursor].Path, m.configCandidates)
	}

	return m, nil
}

func (m Model) updateConfirmConfigSave(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel), key.Matches(msg, m.keys.Confirm), isNoKey(msg):
		m.modal = modalNone
		m.actionStatus = "Config save cancelled"
		m.actionErr = nil
		return m, nil
	case isYesKey(msg):
		m.modal = modalNone
		m.configSaving = true
		m.actionStatus = fmt.Sprintf("Saving %s", m.configDoc.Path)
		m.actionErr = nil
		return m, saveConfig(m.configDoc)
	}

	return m, nil
}

func (m Model) updateConfirmConfigDiscard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel), key.Matches(msg, m.keys.Confirm), isNoKey(msg):
		m.modal = modalNone
		m.pendingConfigTab = tabConfig
		return m, nil
	case isYesKey(msg):
		m.configDirty = false
		m.modal = modalNone
		if m.pendingConfigTab != tabConfig {
			target := m.pendingConfigTab
			m.pendingConfigTab = tabConfig
			return m.switchTab(target), nil
		}
		return m, tea.Quit
	}

	return m, nil
}

func isYesKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'y' || msg.Runes[0] == 'Y')
}

func isNoKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'n' || msg.Runes[0] == 'N')
}

func (m Model) updateFilterModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.filterCursor = (m.filterCursor + len(filterModes()) - 1) % len(filterModes())
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.filterCursor = (m.filterCursor + 1) % len(filterModes())
		return m, nil
	case key.Matches(msg, m.keys.FilterAll):
		m.filterCursor = filterIndex(containerfilter.ModeAll)
		m.setFilter(containerfilter.ModeAll)
		m.modal = modalNone
		return m, nil
	case key.Matches(msg, m.keys.FilterDev):
		m.filterCursor = filterIndex(containerfilter.ModeDevcontainers)
		m.setFilter(containerfilter.ModeDevcontainers)
		m.modal = modalNone
		return m, nil
	case key.Matches(msg, m.keys.FilterDocker):
		m.filterCursor = filterIndex(containerfilter.ModeContainers)
		m.setFilter(containerfilter.ModeContainers)
		m.modal = modalNone
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		m.setFilter(filterModes()[m.filterCursor])
		m.modal = modalNone
		return m, nil
	}

	return m, nil
}

func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.activeTab == tabConfig {
		switch msg.Type {
		case tea.MouseWheelUp:
			m.moveConfigCursor(-3)
		case tea.MouseWheelDown:
			m.moveConfigCursor(3)
		case tea.MouseLeft:
			row := (msg.Y - headerPaneHeight() - 2) / rowHeight
			if row >= 0 {
				index := m.configOffset + row
				if index >= 0 && index < len(m.configRows()) {
					m.configCursor = index
					m.ensureConfigCursorVisible()
				}
			}
		}
		return m, nil
	}
	if m.activeTab == tabTemplates {
		switch msg.Type {
		case tea.MouseWheelUp:
			m.moveTemplateCursor(-3)
		case tea.MouseWheelDown:
			m.moveTemplateCursor(3)
		case tea.MouseLeft:
			row := (msg.Y - headerPaneHeight() - 2) / rowHeight
			if row >= 0 {
				index := m.templateOffset + row
				if index >= 0 && index < len(m.visibleTemplates) {
					m.templateCursor = index
					m.ensureTemplateCursorVisible()
				}
			}
		}
		return m, nil
	}

	switch msg.Type {
	case tea.MouseWheelUp:
		m.moveCursor(-3)
	case tea.MouseWheelDown:
		m.moveCursor(3)
	case tea.MouseLeft:
		row := (msg.Y - headerPaneHeight() - 2) / rowHeight
		if row >= 0 {
			index := m.offset + row
			if index >= 0 && index < len(m.visible) {
				m.cursor = index
				m.ensureCursorVisible()
			}
		}
	}

	return m, nil
}
