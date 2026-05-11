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
	loadTimeout  = 10 * time.Second
	headerHeight = 4
	rowHeight    = 2
)

// ContainerProvider is the read-only data source required by the TUI.
type ContainerProvider interface {
	ListContainers(context.Context) ([]domain.Container, error)
}

type containersLoadedMsg struct {
	containers []domain.Container
	err        error
}

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
	searchMode bool

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
	searchInput.Prompt = "search › "
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
		m.searchInput.Width = max(10, msg.Width-14)
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
		if m.searchMode {
			return m.updateSearch(msg)
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
		m.renderHeader(),
		m.renderRows(),
		m.renderFooter(),
	}

	return m.styles.App.Width(max(0, m.width-2)).Render(strings.Join(sections, "\n"))
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Cancel):
		m.searchMode = false
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
		m.searchMode = true
		m.searchInput.Focus()
		return m, textinput.Blink
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

func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.MouseWheelUp:
		m.moveCursor(-3)
	case tea.MouseWheelDown:
		m.moveCursor(3)
	case tea.MouseLeft:
		row := (msg.Y - headerHeight) / rowHeight
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
	available := m.height - headerHeight - footerHeight(m.help.ShowAll)
	if available <= 0 {
		available = 12
	}
	rows := available / rowHeight
	if rows < 1 {
		return 1
	}
	return rows
}

func footerHeight(showAll bool) int {
	if showAll {
		return 4
	}
	return 2
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

func (m Model) renderHeader() string {
	devCount := 0
	for _, container := range m.containers {
		if container.IsDevcontainer {
			devCount++
		}
	}

	status := fmt.Sprintf("%d total • %d devcontainers • %d shown • filter: %s", len(m.containers), devCount, len(m.visible), m.filterMode)
	if m.loading {
		status += " • refreshing…"
	}
	if m.query != "" {
		status += fmt.Sprintf(" • search: %q", m.query)
	}

	title := lipgloss.JoinHorizontal(lipgloss.Center, m.styles.Title.Render("lazydc"), " ", m.styles.Subtle.Render("read-only devcontainer viewer"))
	search := m.renderSearchLine()
	divider := m.styles.Divider.Render(strings.Repeat("─", max(0, m.width-2)))

	return strings.Join([]string{
		title,
		m.styles.Header.Render(status),
		search,
		divider,
	}, "\n")
}

func (m Model) renderSearchLine() string {
	if m.searchMode {
		return m.searchInput.View()
	}

	query := "press / to search"
	if m.query != "" {
		query = "search: " + m.query + "  (esc clears)"
	}
	return m.styles.Subtle.Render(query + " • a all • d devcontainers • o containers • tab/h/l cycle")
}

func (m Model) renderRows() string {
	if m.err != nil && len(m.containers) == 0 {
		return m.styles.Empty.Render(m.styles.Error.Render("Could not load Docker containers") + "\n" + wrap(m.err.Error(), max(40, m.width-6)))
	}

	if m.loading && len(m.containers) == 0 {
		return m.styles.Empty.Render("Loading Docker containers…")
	}

	if len(m.visible) == 0 {
		return m.styles.Empty.Render(m.emptyMessage())
	}

	rows := m.visibleRowCount()
	end := min(len(m.visible), m.offset+rows)
	rendered := make([]string, 0, end-m.offset+1)

	if m.err != nil {
		rendered = append(rendered, m.styles.Error.Render("Refresh failed: ")+wrap(m.err.Error(), max(40, m.width-20)))
	}

	for index := m.offset; index < end; index++ {
		rendered = append(rendered, m.renderRow(index, m.visible[index], index == m.cursor))
	}

	return strings.Join(rendered, "\n")
}

func (m Model) renderRow(index int, container domain.Container, selected bool) string {
	rowWidth := max(24, m.width-4)
	badge := m.styles.BadgeDocker.Render("DOCKER")
	if container.IsDevcontainer {
		badge = m.styles.BadgeDev.Render("DEV")
	}

	nameWidth := max(12, rowWidth/4)
	imageWidth := max(14, rowWidth/4)
	statusWidth := max(12, rowWidth/5)

	name := m.styles.Name.Render(truncate(container.DisplayName(), nameWidth))
	shortID := m.styles.Muted.Render(container.ShortID)
	image := m.styles.Muted.Render(truncate(container.Image, imageWidth))
	status := container.Status
	if status == "" {
		status = container.State
	}
	status = m.styles.Status.Render(truncate(status, statusWidth))

	selector := " "
	if selected {
		selector = "›"
	}
	line1 := fmt.Sprintf("%s %s %s %s %s %s", selector, badge, name, shortID, image, status)

	line2Prefix := "  ↳ "
	line2Text := "regular Docker container"
	line2Style := m.styles.Muted
	if container.IsDevcontainer {
		line2Style = m.styles.Path
		line2Text = container.DevcontainerPath
		if line2Text == "" {
			line2Text = "devcontainer workspace path not reported"
		}
		if container.DevcontainerSource != "" {
			line2Text += "  " + m.styles.Muted.Render("("+container.DevcontainerSource+")")
		}
	}
	line2 := line2Prefix + line2Style.Render(truncate(line2Text, rowWidth-lipgloss.Width(line2Prefix)))

	row := line1 + "\n" + line2
	style := m.styles.Row.Width(rowWidth)
	if selected {
		style = m.styles.SelectedRow.Width(rowWidth)
	}

	_ = index
	return style.Render(row)
}

func (m Model) renderFooter() string {
	position := ""
	if len(m.visible) > 0 {
		position = fmt.Sprintf("%d/%d", m.cursor+1, len(m.visible))
	}
	footer := m.styles.Subtle.Render(position)
	if position != "" {
		footer += "  "
	}
	footer += m.styles.Help.Render(m.help.View(m.keys))
	return footer
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
