package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Souheab/lazydevcontainer/internal/domain"
	containerfilter "github.com/Souheab/lazydevcontainer/internal/filter"
	devtemplates "github.com/Souheab/lazydevcontainer/internal/templates"
)

// View renders the current screen.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading lazydc..."
	}

	sections := []string{
		m.renderHeaderPane(),
		m.renderMainPane(),
		m.renderFooterPane(),
	}

	view := m.styles.App.Width(max(0, m.width)).Height(max(0, m.height)).Render(strings.Join(sections, "\n"))
	if m.modal != modalNone {
		return m.renderModal()
	}
	return view
}

func (m Model) renderHeaderPane() string {
	devCount := 0
	for _, container := range m.containers {
		if container.IsDevcontainer {
			devCount++
		}
	}

	status := fmt.Sprintf("%s  %d total  %d dev  %d shown  %s", m.renderTabs(), len(m.containers), devCount, len(m.visible), m.filterMode)
	if m.activeTab == tabTemplates {
		status = fmt.Sprintf("%s  %d templates  %d shown  target: %s", m.renderTabs(), len(m.templates), len(m.visibleTemplates), m.targetDir)
	}
	if m.activeTab == tabConfig {
		state := "clean"
		if m.configDirty {
			state = "dirty"
		}
		status = fmt.Sprintf("%s  config: %s  %s", m.renderTabs(), m.configDoc.Path, state)
	}
	if m.actionStatus != "" {
		status += "  " + m.actionStatus
	}
	if m.actionErr != nil {
		status += ": " + m.actionErr.Error()
	}
	if m.activeTab == tabContainers && m.query != "" {
		status += fmt.Sprintf("  search: %q", m.query)
	}
	if m.activeTab == tabTemplates && m.templateQuery != "" {
		status += fmt.Sprintf("  search: %q", m.templateQuery)
	}
	if m.err != nil {
		status += "  refresh failed"
	}
	if m.actionInProgress {
		status += "  action running..."
	}
	if m.templateWriteInProgress {
		status += "  writing template..."
	}
	if m.loading {
		status += "  refreshing..."
	}
	if m.activeTab == tabConfig && m.configLoading {
		status += "  loading config..."
	}
	if m.activeTab == tabConfig && m.configSaving {
		status += "  saving config..."
	}
	if m.activeTab == tabConfig && m.featureCatalogLoading {
		status += "  loading features..."
	}
	if m.configErr != nil {
		status += "  config failed: " + m.configErr.Error()
	}

	body := m.styles.Header.Render(truncate(status, paneContentWidth(m.width)))

	return renderTitledPane(m.styles.Pane, m.styles.PaneBorder, m.styles.PaneTitle, "Status", body, paneInnerWidth(m.width), headerContentHeight)
}

func (m Model) renderTabs() string {
	containers := "Containers"
	templates := "Templates"
	config := "Config"
	if m.activeTab == tabContainers {
		containers = "[" + containers + "]"
	} else if m.activeTab == tabTemplates {
		templates = "[" + templates + "]"
	} else {
		config = "[" + config + "]"
	}
	return containers + " " + templates + " " + config
}

func (m Model) renderMainPane() string {
	if m.activeTab == tabTemplates {
		return m.renderTemplatesMainPane()
	}
	if m.activeTab == tabConfig {
		return m.renderConfigMainPane()
	}

	contentHeight := containerContentHeight(m.height, m.help.ShowAll)
	bodyHeight := max(1, contentHeight)
	leftOuter, rightOuter := splitPaneOuterWidths(m.width)
	leftContentWidth := splitPaneContentWidth(leftOuter)
	rightContentWidth := splitPaneContentWidth(rightOuter)
	listRows := max(1, bodyHeight/rowHeight)
	body := m.renderRows(listRows, leftContentWidth)
	title := fmt.Sprintf("Containers %d of %d", selectedPosition(m.cursor, len(m.visible)), len(m.visible))

	if lipgloss.Height(body) < bodyHeight {
		body += strings.Repeat("\n", bodyHeight-lipgloss.Height(body))
	}

	listPane := renderTitledPane(m.styles.ActivePane, m.styles.ActiveBorder, m.styles.PaneTitle, title, body, splitPaneInnerWidth(leftOuter), contentHeight)
	detailsPane := renderTitledPane(m.styles.Pane, m.styles.PaneBorder, m.styles.PaneTitle, "Details", m.renderDetails(bodyHeight, rightContentWidth), splitPaneInnerWidth(rightOuter), contentHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailsPane)
}

func (m Model) renderConfigMainPane() string {
	contentHeight := containerContentHeight(m.height, m.help.ShowAll)
	bodyHeight := max(1, contentHeight)
	leftOuter, rightOuter := splitPaneOuterWidths(m.width)
	leftContentWidth := splitPaneContentWidth(leftOuter)
	rightContentWidth := splitPaneContentWidth(rightOuter)
	listRows := max(1, bodyHeight/rowHeight)
	body := m.renderConfigRows(listRows, leftContentWidth)
	title := fmt.Sprintf("Config %d of %d", selectedPosition(m.configCursor, len(m.configRows())), len(m.configRows()))

	if lipgloss.Height(body) < bodyHeight {
		body += strings.Repeat("\n", bodyHeight-lipgloss.Height(body))
	}

	listPane := renderTitledPane(m.styles.ActivePane, m.styles.ActiveBorder, m.styles.PaneTitle, title, body, splitPaneInnerWidth(leftOuter), contentHeight)
	previewPane := renderTitledPane(m.styles.Pane, m.styles.PaneBorder, m.styles.PaneTitle, "Preview", m.renderConfigPreview(bodyHeight, rightContentWidth), splitPaneInnerWidth(rightOuter), contentHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane)
}

func (m Model) renderConfigRows(rows int, rowWidth int) string {
	if m.configLoading {
		return m.styles.Empty.Render("Loading devcontainer config...")
	}
	if m.configErr != nil {
		return m.styles.Empty.Render(m.styles.Error.Render("Could not load config") + "\n" + wrap(m.configErr.Error(), max(24, rowWidth)))
	}

	configRows := m.configRows()
	end := min(len(configRows), m.configOffset+rows)
	rendered := make([]string, 0, end-m.configOffset)
	for index := m.configOffset; index < end; index++ {
		rendered = append(rendered, m.renderConfigRow(configRows[index], index == m.configCursor, rowWidth))
	}
	return strings.Join(rendered, "\n")
}

func (m Model) renderConfigRow(row configRow, selected bool, rowWidth int) string {
	selector := " "
	if selected {
		selector = ">"
	}
	value := row.value
	if strings.TrimSpace(value) == "" {
		value = "Not set"
	}
	label := truncate(row.label, max(1, rowWidth-4))
	value = truncate(value, max(1, rowWidth-2))
	renderedLabel := m.styles.Name.Render(label)
	renderedValue := m.styles.Path.Render(value)
	if selected {
		renderedLabel = label
		renderedValue = value
	}
	rowText := lipgloss.JoinVertical(lipgloss.Left, fmt.Sprintf("%s %s", selector, renderedLabel), "  "+renderedValue, "")
	style := m.styles.Row.Width(rowWidth)
	if selected {
		style = m.styles.SelectedRow.Width(rowWidth)
	}
	return style.Render(rowText)
}

func (m Model) renderConfigPreview(bodyHeight int, width int) string {
	if m.configLoading {
		return fillHeight(m.styles.Empty.Render("Waiting for devcontainer config..."), bodyHeight)
	}
	if m.configErr != nil {
		return fillHeight(m.styles.Error.Render(m.configErr.Error()), bodyHeight)
	}

	rendered, err := m.configDoc.JSON()
	if err != nil {
		return fillHeight(m.styles.Error.Render(err.Error()), bodyHeight)
	}

	lines := []string{}
	lines = appendDetail(lines, "Path", m.configDoc.Path, width, m.styles)
	lines = appendDetail(lines, "Features", fmt.Sprintf("%d configured", len(m.configDoc.Features)), width, m.styles)
	if len(m.configCandidates) > 1 {
		lines = appendDetail(lines, "Detected configs", fmt.Sprintf("%d; using first by spec precedence", len(m.configCandidates)), width, m.styles)
	}
	lines = append(lines, m.styles.Subtle.Render("devcontainer.json"))
	for _, line := range strings.Split(strings.TrimRight(string(rendered), "\n"), "\n") {
		lines = append(lines, truncate(line, width))
	}

	body := strings.Join(lines, "\n")
	if lipgloss.Height(body) > bodyHeight {
		bodyLines := strings.Split(body, "\n")
		body = strings.Join(bodyLines[:bodyHeight], "\n")
	}
	return fillHeight(body, bodyHeight)
}

func (m Model) renderTemplatesMainPane() string {
	contentHeight := containerContentHeight(m.height, m.help.ShowAll)
	bodyHeight := max(1, contentHeight)
	leftOuter, rightOuter := splitPaneOuterWidths(m.width)
	leftContentWidth := splitPaneContentWidth(leftOuter)
	rightContentWidth := splitPaneContentWidth(rightOuter)
	listRows := max(1, bodyHeight/rowHeight)
	body := m.renderTemplateRows(listRows, leftContentWidth)
	title := fmt.Sprintf("Templates %d of %d", selectedPosition(m.templateCursor, len(m.visibleTemplates)), len(m.visibleTemplates))

	if lipgloss.Height(body) < bodyHeight {
		body += strings.Repeat("\n", bodyHeight-lipgloss.Height(body))
	}

	listPane := renderTitledPane(m.styles.ActivePane, m.styles.ActiveBorder, m.styles.PaneTitle, title, body, splitPaneInnerWidth(leftOuter), contentHeight)
	detailsPane := renderTitledPane(m.styles.Pane, m.styles.PaneBorder, m.styles.PaneTitle, "Preview", m.renderTemplatePreview(bodyHeight, rightContentWidth), splitPaneInnerWidth(rightOuter), contentHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailsPane)
}

func (m Model) renderTemplateRows(rows int, rowWidth int) string {
	if len(m.visibleTemplates) == 0 {
		return m.styles.Empty.Render("No templates match the current search. Press esc to clear it.")
	}

	end := min(len(m.visibleTemplates), m.templateOffset+rows)
	rendered := make([]string, 0, end-m.templateOffset)
	for index := m.templateOffset; index < end; index++ {
		rendered = append(rendered, m.renderTemplateRow(index, m.visibleTemplates[index], index == m.templateCursor, rowWidth))
	}
	return strings.Join(rendered, "\n")
}

func (m Model) renderTemplateRow(index int, template devtemplates.Template, selected bool, rowWidth int) string {
	selector := " "
	if selected {
		selector = ">"
	}

	tagText := strings.Join(template.Tags, ", ")
	prefixWidth := lipgloss.Width(selector) + 1
	tagText = truncate(tagText, max(1, rowWidth-prefixWidth-2))
	tagWidth := lipgloss.Width(tagText)
	availableMainWidth := max(1, rowWidth-prefixWidth-tagWidth-2)
	name := truncate(template.Name, availableMainWidth)
	gapWidth := max(1, rowWidth-prefixWidth-lipgloss.Width(name)-tagWidth)

	renderedName := m.styles.Name.Render(name)
	renderedTags := m.styles.Status.Render(tagText)
	description := truncate(template.Description, max(0, rowWidth-2))
	if selected {
		renderedName = name
		renderedTags = tagText
	}

	firstLine := fmt.Sprintf("%s %s%s%s", selector, renderedName, strings.Repeat(" ", gapWidth), renderedTags)
	secondLine := "  " + description
	row := lipgloss.JoinVertical(lipgloss.Left, firstLine, secondLine, "")
	style := m.styles.Row.Width(rowWidth)
	if selected {
		style = m.styles.SelectedRow.Width(rowWidth)
	}

	_ = index
	return style.Render(row)
}

func (m Model) renderTemplatePreview(bodyHeight int, width int) string {
	template, ok := m.selectedTemplate()
	if !ok {
		return fillHeight(m.styles.Empty.Render("Select a template to preview devcontainer.json."), bodyHeight)
	}

	rendered, err := template.Render()
	if err != nil {
		return fillHeight(m.styles.Error.Render(err.Error()), bodyHeight)
	}

	lines := []string{}
	lines = appendDetail(lines, "Template", template.Name, width, m.styles)
	lines = appendDetail(lines, "Description", template.Description, width, m.styles)
	lines = appendDetail(lines, "Target", m.templateTargetPath(), width, m.styles)
	lines = append(lines, m.styles.Subtle.Render("devcontainer.json"))
	for _, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		lines = append(lines, truncate(line, width))
	}

	body := strings.Join(lines, "\n")
	if lipgloss.Height(body) > bodyHeight {
		bodyLines := strings.Split(body, "\n")
		body = strings.Join(bodyLines[:bodyHeight], "\n")
	}
	return fillHeight(body, bodyHeight)
}

func (m Model) renderRows(rows int, rowWidth int) string {
	if m.err != nil && len(m.containers) == 0 {
		return m.styles.Empty.Render(m.styles.Error.Render("Could not load Docker containers") + "\n" + wrap(m.err.Error(), max(24, rowWidth)))
	}

	if m.loading && len(m.containers) == 0 {
		return m.styles.Empty.Render("Loading Docker containers...")
	}

	if len(m.visible) == 0 {
		return m.styles.Empty.Render(m.emptyMessage())
	}

	end := min(len(m.visible), m.offset+rows)
	rendered := make([]string, 0, end-m.offset+1)

	if m.err != nil {
		rendered = append(rendered, m.styles.Error.Render("Refresh failed: ")+wrap(m.err.Error(), max(24, rowWidth-16)))
	}

	for index := m.offset; index < end; index++ {
		rendered = append(rendered, m.renderRow(index, m.visible[index], index == m.cursor, rowWidth))
	}

	return strings.Join(rendered, "\n")
}

func (m Model) renderRow(index int, container domain.Container, selected bool, rowWidth int) string {
	status := container.Status
	if status == "" {
		status = container.State
	}

	selector := " "
	if selected {
		selector = ">"
	}

	prefixWidth := lipgloss.Width(selector) + 1
	if status != "" {
		status = truncate(status, max(1, rowWidth-prefixWidth-2))
	}
	statusWidth := lipgloss.Width(status)
	availableMainWidth := max(1, rowWidth-prefixWidth-statusWidth-2)
	nameText := container.DisplayName()
	name := truncate(nameText, availableMainWidth)

	gapWidth := max(1, rowWidth-prefixWidth-lipgloss.Width(name)-statusWidth)
	renderedName := m.styles.Name.Render(name)
	renderedStatus := m.styles.Status.Render(status)
	path := container.DevcontainerPath
	if path == "" {
		path = container.Image
	}
	renderedPath := m.styles.Path.Render(truncate(path, max(0, rowWidth-2)))
	if selected {
		renderedName = name
		renderedStatus = status
		renderedPath = truncate(path, max(0, rowWidth-2))
	}
	firstLine := fmt.Sprintf(
		"%s %s%s%s",
		selector,
		renderedName,
		strings.Repeat(" ", gapWidth),
		renderedStatus,
	)
	secondLine := "  " + renderedPath
	row := lipgloss.JoinVertical(lipgloss.Left, firstLine, secondLine, "")
	style := m.styles.Row.Width(rowWidth)
	if selected {
		style = m.styles.SelectedRow.Width(rowWidth)
	}

	_ = index
	return style.Render(row)
}

func (m Model) renderDetails(bodyHeight int, width int) string {
	if m.err != nil && len(m.containers) == 0 {
		return fillHeight(m.styles.Empty.Render("Details unavailable"), bodyHeight)
	}
	if m.loading && len(m.containers) == 0 {
		return fillHeight(m.styles.Empty.Render("Waiting for Docker..."), bodyHeight)
	}
	if len(m.visible) == 0 {
		return fillHeight(m.styles.Empty.Render("Select a container to see details."), bodyHeight)
	}

	container := m.visible[m.cursor]
	project := container.DevcontainerPath
	if project == "" {
		project = "Not detected"
	}
	status := container.Status
	if status == "" {
		status = container.State
	}

	lines := []string{}
	lines = appendDetail(lines, "Container Type", containerType(container), width, m.styles)
	lines = appendDetail(lines, "Project", project, width, m.styles)
	lines = appendDetail(lines, "Image", container.Image, width, m.styles)
	lines = appendDetail(lines, "Status", status, width, m.styles)
	lines = appendDetail(lines, "Volumes", mountSummary(container.Mounts), width, m.styles)
	lines = appendDetail(lines, "Ports", portSummary(container.Ports), width, m.styles)
	body := strings.Join(lines, "\n")
	if lipgloss.Height(body) > bodyHeight {
		bodyLines := strings.Split(body, "\n")
		body = strings.Join(bodyLines[:bodyHeight], "\n")
	}
	return fillHeight(body, bodyHeight)
}

func (m Model) renderFooterPane() string {
	footer := m.styles.Help.Render(m.help.View(m.keys))
	return renderTitledPane(m.styles.Pane, m.styles.PaneBorder, m.styles.PaneTitle, "Keybindings", footer, paneInnerWidth(m.width), footerContentHeight(m.help.ShowAll))
}

func (m Model) renderModal() string {
	switch m.modal {
	case modalSearch:
		return m.renderSearchModal()
	case modalFilter:
		return m.renderFilterModal()
	case modalConfirmAction:
		return m.renderConfirmActionModal()
	case modalConfirmTemplateWrite:
		return m.renderConfirmTemplateWriteModal()
	case modalConfigInput:
		return m.renderConfigInputModal()
	case modalConfigFeature:
		return m.renderConfigFeatureModal()
	case modalConfigCandidate:
		return m.renderConfigCandidateModal()
	case modalConfirmConfigSave:
		return m.renderConfirmConfigSaveModal()
	case modalConfirmConfigDiscard:
		return m.renderConfirmConfigDiscardModal()
	default:
		return ""
	}
}

func (m Model) renderSearchModal() string {
	width := max(32, min(64, m.width-8))
	input := m.searchInput
	title := "[/] Search containers"
	if m.activeTab == tabTemplates {
		input = m.templateSearchInput
		title = "[/] Search templates"
	}
	input.Width = max(10, width-8)
	body := strings.Join([]string{
		m.styles.PaneTitle.Render(title),
		input.View(),
		m.styles.Subtle.Render("enter applies  esc closes"),
	}, "\n")
	modal := m.styles.Modal.Width(width).Render(body)
	return lipgloss.Place(m.width, max(m.height, lipgloss.Height(modal)), lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderFilterModal() string {
	width := max(28, min(44, m.width-8))
	lines := []string{m.styles.PaneTitle.Render("[f] Filter containers")}
	for index, mode := range filterModes() {
		selector := " "
		label := mode.String()
		if index == m.filterCursor {
			selector = ">"
			label = m.styles.SelectedRow.Width(width - 4).Render(" " + label)
		}
		lines = append(lines, fmt.Sprintf("%s %s", selector, label))
	}
	lines = append(lines, m.styles.Subtle.Render("enter applies  esc closes"))
	modal := m.styles.Modal.Width(width).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, max(m.height, lipgloss.Height(modal)), lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderConfirmActionModal() string {
	width := max(34, min(64, m.width-8))
	action := m.pendingAction
	body := strings.Join([]string{
		m.styles.PaneTitle.Render("[enter] Confirm action"),
		fmt.Sprintf("%s %s?", actionPrompt(action.kind), action.containerName),
		m.styles.Subtle.Render("enter confirms  esc cancels"),
	}, "\n")
	modal := m.styles.Modal.Width(width).Render(body)
	return lipgloss.Place(m.width, max(m.height, lipgloss.Height(modal)), lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderConfirmTemplateWriteModal() string {
	width := max(42, min(78, m.width-8))
	action := "Create"
	detail := fmt.Sprintf("Create %s in %s?", filepath.Base(m.pendingTemplatePath), filepath.Dir(filepath.Dir(m.pendingTemplatePath)))
	if m.pendingTemplateOverwrite {
		action = "Overwrite"
		detail = fmt.Sprintf("Overwrite existing %s?", m.pendingTemplatePath)
	}
	body := strings.Join([]string{
		m.styles.PaneTitle.Render("[enter] " + action + " devcontainer"),
		fmt.Sprintf("%s template: %s", action, m.pendingTemplate.Name),
		wrap(detail, max(24, width-4)),
		m.styles.Subtle.Render("enter confirms  esc cancels"),
	}, "\n")
	modal := m.styles.Modal.Width(width).Render(body)
	return lipgloss.Place(m.width, max(m.height, lipgloss.Height(modal)), lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderConfigInputModal() string {
	width := max(42, min(84, m.width-8))
	input := m.configInput
	input.Width = max(10, width-8)
	title := "[enter] Edit config"
	switch m.configEdit {
	case configEditAddExtension:
		title = "[enter] Add/edit extension"
	case configEditFeatureOptions:
		title = "[enter] Edit feature options"
	case configEditAddFeatureManual:
		title = "[enter] Add feature"
	}
	body := strings.Join([]string{
		m.styles.PaneTitle.Render(title),
		input.View(),
		m.styles.Subtle.Render("enter applies  esc cancels"),
	}, "\n")
	modal := m.styles.Modal.Width(width).Render(body)
	return lipgloss.Place(m.width, max(m.height, lipgloss.Height(modal)), lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderConfigFeatureModal() string {
	width := max(56, min(94, m.width-8))
	height := max(8, min(18, m.height-8))
	input := m.featureSearchInput
	input.Width = max(10, width-8)
	lines := []string{
		m.styles.PaneTitle.Render("[/] Add feature"),
		input.View(),
	}
	if m.featureCatalogLoading {
		lines = append(lines, m.styles.Empty.Render("Loading feature catalog..."))
	} else if len(m.visibleFeatures) == 0 {
		lines = append(lines, m.styles.Empty.Render("No catalog match. Press enter to add typed feature ID."))
	} else {
		rowCount := max(1, height-5)
		end := min(len(m.visibleFeatures), m.featureOffset+rowCount)
		for index := m.featureOffset; index < end; index++ {
			feature := m.visibleFeatures[index]
			selector := " "
			text := feature.ID
			if feature.Name != "" {
				text = feature.Name + "  " + feature.ID
			}
			text = truncate(text, width-6)
			if index == m.featureCursor {
				selector = ">"
				text = m.styles.SelectedRow.Width(width - 5).Render(" " + text)
			}
			lines = append(lines, fmt.Sprintf("%s %s", selector, text))
		}
	}
	lines = append(lines, m.styles.Subtle.Render("enter adds selection or typed ID  esc cancels"))
	body := strings.Join(lines, "\n")
	modal := m.styles.Modal.Width(width).Height(height).Render(body)
	return lipgloss.Place(m.width, max(m.height, lipgloss.Height(modal)), lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderConfigCandidateModal() string {
	width := max(56, min(96, m.width-8))
	lines := []string{m.styles.PaneTitle.Render("[enter] Choose devcontainer config")}
	for index, candidate := range m.configCandidates {
		selector := " "
		label := candidate.Path
		if !candidate.Exists {
			label += " (new)"
		}
		label = truncate(label, width-6)
		if index == m.configCandidateCursor {
			selector = ">"
			label = m.styles.SelectedRow.Width(width - 5).Render(" " + label)
		}
		lines = append(lines, fmt.Sprintf("%s %s", selector, label))
	}
	lines = append(lines, m.styles.Subtle.Render("enter opens  esc keeps first by spec precedence"))
	modal := m.styles.Modal.Width(width).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, max(m.height, lipgloss.Height(modal)), lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderConfirmConfigSaveModal() string {
	width := max(42, min(78, m.width-8))
	body := strings.Join([]string{
		m.styles.PaneTitle.Render("[enter] Save devcontainer"),
		wrap(fmt.Sprintf("Write changes to %s?", m.configDoc.Path), max(24, width-4)),
		m.styles.Subtle.Render("enter confirms  esc cancels"),
	}, "\n")
	modal := m.styles.Modal.Width(width).Render(body)
	return lipgloss.Place(m.width, max(m.height, lipgloss.Height(modal)), lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderConfirmConfigDiscardModal() string {
	width := max(42, min(78, m.width-8))
	body := strings.Join([]string{
		m.styles.PaneTitle.Render("[enter] Discard config changes"),
		"Discard unsaved devcontainer config changes?",
		m.styles.Subtle.Render("enter discards  esc returns"),
	}, "\n")
	modal := m.styles.Modal.Width(width).Render(body)
	return lipgloss.Place(m.width, max(m.height, lipgloss.Height(modal)), lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) emptyMessage() string {
	switch {
	case len(m.containers) == 0:
		return "No Docker containers found. Start a container and press r to refresh."
	case m.query != "":
		return "No containers match the current search. Press esc to clear it."
	case m.filterMode == containerfilter.ModeDevcontainers:
		return "No devcontainers detected. Press a to show all Docker containers."
	case m.filterMode == containerfilter.ModeContainers:
		return "No ordinary Docker containers detected. Press a to show all containers."
	default:
		return "No containers to display."
	}
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}

	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func wrap(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}

	words := strings.Fields(value)
	if len(words) == 0 {
		return value
	}

	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if lipgloss.Width(current)+1+lipgloss.Width(word) > width {
			lines = append(lines, current)
			current = word
			continue
		}
		current += " " + word
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func appendDetail(lines []string, label string, value string, width int, styles styles) []string {
	if strings.TrimSpace(value) == "" {
		value = "None"
	}
	lines = append(lines, styles.Subtle.Render(label))
	for _, valueLine := range strings.Split(value, "\n") {
		for _, line := range strings.Split(wrap(valueLine, width), "\n") {
			lines = append(lines, truncate(line, width))
		}
	}
	lines = append(lines, "")
	return lines
}

func mountSummary(mounts []domain.Mount) string {
	if len(mounts) == 0 {
		return "None attached"
	}

	values := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		values = append(values, mountDescription(mount))
	}
	return strings.Join(values, "\n")
}

func mountDescription(mount domain.Mount) string {
	source := mount.Name
	if source == "" {
		source = mount.Source
	}
	if source == "" {
		source = mount.Destination
	}
	if source == "" {
		source = "Unknown"
	}

	value := source
	if mount.Destination != "" && mount.Destination != source {
		value = fmt.Sprintf("%s -> %s", value, mount.Destination)
	}
	if mount.Type != "" {
		value = fmt.Sprintf("%s: %s", mount.Type, value)
	}
	if mount.ReadOnly {
		value += " (read-only)"
	}
	return value
}

func portSummary(ports []domain.Port) string {
	if len(ports) == 0 {
		return "None published"
	}

	values := make([]string, 0, len(ports))
	for _, port := range ports {
		private := fmt.Sprintf("%d", port.PrivatePort)
		if port.Type != "" && port.Type != "tcp" {
			private += "/" + port.Type
		}
		if port.PublicPort == 0 {
			values = append(values, private)
			continue
		}

		public := fmt.Sprintf("%d", port.PublicPort)
		if port.IP != "" && port.IP != "0.0.0.0" && port.IP != "::" {
			public = port.IP + ":" + public
		}
		values = append(values, fmt.Sprintf("%s -> %s", public, private))
	}
	return strings.Join(values, "\n")
}

func containerType(container domain.Container) string {
	if container.IsDevcontainer {
		return "Devcontainer"
	}
	return "Docker Container"
}

func fillHeight(value string, height int) string {
	if lipgloss.Height(value) >= height {
		return value
	}
	return value + strings.Repeat("\n", height-lipgloss.Height(value))
}

func renderTitledPane(style lipgloss.Style, borderStyle lipgloss.Style, titleStyle lipgloss.Style, title string, body string, width int, height int) string {
	pane := style.Width(width).Height(height).Render(body)
	lines := strings.Split(pane, "\n")
	if len(lines) == 0 {
		return pane
	}

	lineWidth := lipgloss.Width(lines[0])
	maxTitleWidth := max(0, lineWidth-4)
	renderedTitle := titleStyle.Render(truncate(title, maxTitleWidth))
	titleWidth := lipgloss.Width(renderedTitle)
	fillWidth := max(0, lineWidth-titleWidth-2)

	lines[0] = borderStyle.Render("╭") + renderedTitle + borderStyle.Render(strings.Repeat("─", fillWidth)+"╮")
	return strings.Join(lines, "\n")
}

func filterModes() []containerfilter.Mode {
	return []containerfilter.Mode{
		containerfilter.ModeAll,
		containerfilter.ModeDevcontainers,
		containerfilter.ModeContainers,
	}
}

func filterIndex(mode containerfilter.Mode) int {
	for index, candidate := range filterModes() {
		if candidate == mode {
			return index
		}
	}
	return 0
}

func paneInnerWidth(width int) int {
	return max(20, width-2)
}

func paneContentWidth(width int) int {
	return max(16, width-4)
}

func splitPaneOuterWidths(width int) (int, int) {
	if width < 56 {
		left := max(18, width*55/100)
		return left, max(1, width-left)
	}
	left := max(32, min(width-28, width*45/100))
	return left, width - left
}

func splitPaneInnerWidth(width int) int {
	return max(1, width-2)
}

func splitPaneContentWidth(width int) int {
	return max(1, width-4)
}

func selectedPosition(cursor int, total int) int {
	if total == 0 {
		return 0
	}
	return cursor + 1
}
