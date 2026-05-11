package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Souheab/lazydevcontainer/internal/domain"
	containerfilter "github.com/Souheab/lazydevcontainer/internal/filter"
)

const (
	loadTimeout         = 10 * time.Second
	headerContentHeight = 2
	shortFooterHeight   = 1
	fullFooterHeight    = 3
	rowHeight           = 3
)

// ContainerProvider is the read-only data source required by the TUI.
type ContainerProvider interface {
	ListContainers(context.Context) ([]domain.Container, error)
}

type containersLoadedMsg struct {
	containers []domain.Container
	err        error
}

type modalMode int

const (
	modalNone modalMode = iota
	modalSearch
	modalFilter
)

// Model is the Bubble Tea application state.
type Model struct {
	provider ContainerProvider

	containers []domain.Container
	visible    []domain.Container

	keys        keyMap
	styles      styles
	help        help.Model
	searchInput textinput.Model

	filterMode containerfilter.Mode
	query      string
	modal      modalMode

	filterCursor int

	cursor int
	offset int
	width  int
	height int

	loading bool
	err     error
}

// New returns a TUI model wired to a container provider.
func New(provider ContainerProvider) Model {
	styles := newStyles()
	searchInput := textinput.New()
	searchInput.Prompt = "/ "
	searchInput.Placeholder = "name, image, status, path, label..."
	searchInput.CharLimit = 256
	searchInput.PromptStyle = styles.Subtle
	searchInput.TextStyle = styles.Search
	searchInput.PlaceholderStyle = styles.Subtle
	searchInput.Blur()

	return Model{
		provider:    provider,
		keys:        newKeyMap(),
		styles:      styles,
		help:        help.New(),
		searchInput: searchInput,
		filterMode:  containerfilter.ModeAll,
		loading:     true,
	}
}

// Init starts the initial asynchronous Docker container load.
func (m Model) Init() tea.Cmd {
	return loadContainers(m.provider)
}

// Update handles keyboard, mouse, resize, and load messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.searchInput.Width = max(10, min(56, msg.Width-12))
		m.ensureCursorVisible()
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

	case tea.KeyMsg:
		if m.modal == modalSearch {
			return m.updateSearch(msg)
		}
		if m.modal == modalFilter {
			return m.updateFilterModal(msg)
		}
		return m.updateKey(msg)

	case tea.MouseMsg:
		return m.updateMouse(msg)
	}

	return m, nil
}

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

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		m.searchInput.Blur()
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		m.modal = modalNone
		m.searchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.query = m.searchInput.Value()
	m.applyFilters()
	m.ensureCursorBounds()
	m.ensureCursorVisible()
	return m, cmd
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case key.Matches(msg, m.keys.Refresh):
		m.loading = true
		m.err = nil
		return m, loadContainers(m.provider)
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

func loadContainers(provider ContainerProvider) tea.Cmd {
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

func (m *Model) setFilter(mode containerfilter.Mode) {
	m.filterMode = mode
	m.applyFilters()
	m.ensureCursorBounds()
	m.ensureCursorVisible()
}

func (m *Model) applyFilters() {
	m.visible = containerfilter.Apply(m.containers, m.filterMode, m.query)
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

func (m Model) visibleRowCount() int {
	rows := (containerContentHeight(m.height, m.help.ShowAll) - 1) / rowHeight
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
	available := height - headerPaneHeight() - footerPaneHeight(showAll)
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

func (m Model) renderHeaderPane() string {
	devCount := 0
	for _, container := range m.containers {
		if container.IsDevcontainer {
			devCount++
		}
	}

	status := fmt.Sprintf("%d total  %d devcontainers  %d shown  filter: %s", len(m.containers), devCount, len(m.visible), m.filterMode)
	if m.loading {
		status += "  refreshing..."
	}
	if m.query != "" {
		status += fmt.Sprintf("  search: %q", m.query)
	}
	if m.err != nil {
		status += "  refresh failed"
	}

	title := lipgloss.JoinHorizontal(lipgloss.Center, m.styles.PaneTitle.Render("Status"), " ", m.styles.Title.Render("lazydc"), " ", m.styles.Subtle.Render("read-only devcontainer viewer"))
	body := strings.Join([]string{title, m.styles.Header.Render(truncate(status, paneContentWidth(m.width)))}, "\n")

	return m.styles.Pane.Width(paneInnerWidth(m.width)).Height(headerContentHeight).Render(body)
}

func (m Model) renderMainPane() string {
	contentHeight := containerContentHeight(m.height, m.help.ShowAll)
	bodyHeight := max(1, contentHeight-1)
	leftOuter, rightOuter := splitPaneOuterWidths(m.width)
	leftContentWidth := splitPaneContentWidth(leftOuter)
	rightContentWidth := splitPaneContentWidth(rightOuter)
	listRows := max(1, bodyHeight/rowHeight)
	body := m.renderRows(listRows, leftContentWidth)
	title := fmt.Sprintf("Containers %d of %d", selectedPosition(m.cursor, len(m.visible)), len(m.visible))

	if lipgloss.Height(body) < bodyHeight {
		body += strings.Repeat("\n", bodyHeight-lipgloss.Height(body))
	}

	listPane := m.styles.ActivePane.Width(splitPaneInnerWidth(leftOuter)).Height(contentHeight).Render(m.styles.PaneTitle.Render(title) + "\n" + body)
	detailsPane := m.styles.Pane.Width(splitPaneInnerWidth(rightOuter)).Height(contentHeight).Render(m.styles.PaneTitle.Render("Details") + "\n" + m.renderDetails(bodyHeight, rightContentWidth))

	return lipgloss.JoinHorizontal(lipgloss.Top, listPane, detailsPane)
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
	firstLine := fmt.Sprintf(
		"%s %s%s%s",
		selector,
		m.styles.Name.Render(name),
		strings.Repeat(" ", gapWidth),
		m.styles.Status.Render(status),
	)
	path := container.DevcontainerPath
	if path == "" {
		path = container.Image
	}
	secondLine := "  " + m.styles.Path.Render(truncate(path, max(0, rowWidth-2)))
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
	position := ""
	if len(m.visible) > 0 {
		position = fmt.Sprintf("%d/%d", m.cursor+1, len(m.visible))
	}
	footer := m.styles.PaneTitle.Render("Keybindings")
	if position != "" {
		footer += " " + m.styles.Subtle.Render(position)
	}
	footer += "\n"
	if position != "" {
		footer += m.styles.Subtle.Render("global: ")
	}
	footer += m.styles.Help.Render(m.help.View(m.keys))
	return m.styles.Pane.Width(paneInnerWidth(m.width)).Height(footerContentHeight(m.help.ShowAll)).Render(footer)
}

func (m Model) renderModal() string {
	switch m.modal {
	case modalSearch:
		return m.renderSearchModal()
	case modalFilter:
		return m.renderFilterModal()
	default:
		return ""
	}
}

func (m Model) renderSearchModal() string {
	width := max(32, min(64, m.width-8))
	m.searchInput.Width = max(10, width-8)
	body := strings.Join([]string{
		m.styles.PaneTitle.Render("[/] Search containers"),
		m.searchInput.View(),
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
	if len(mounts) == 1 {
		return "1 attached"
	}
	return fmt.Sprintf("%d attached", len(mounts))
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

func fillHeight(value string, height int) string {
	if lipgloss.Height(value) >= height {
		return value
	}
	return value + strings.Repeat("\n", height-lipgloss.Height(value))
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
	return max(20, width-4)
}

func paneContentWidth(width int) int {
	return max(16, width-6)
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
	return max(1, width-4)
}

func splitPaneContentWidth(width int) int {
	return max(1, width-6)
}

func selectedPosition(cursor int, total int) int {
	if total == 0 {
		return 0
	}
	return cursor + 1
}
