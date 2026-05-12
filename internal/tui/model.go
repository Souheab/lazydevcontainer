package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	devconfig "github.com/Souheab/lazydevcontainer/internal/config"
	"github.com/Souheab/lazydevcontainer/internal/domain"
	containerfilter "github.com/Souheab/lazydevcontainer/internal/filter"
	devtemplates "github.com/Souheab/lazydevcontainer/internal/templates"
)

const (
	loadTimeout         = 10 * time.Second
	headerContentHeight = 1
	shortFooterHeight   = 1
	fullFooterHeight    = 3
	rowHeight           = 3
)

// ContainerService is the Docker-backed service required by the TUI.
type ContainerService interface {
	ListContainers(context.Context) ([]domain.Container, error)
	StartContainer(context.Context, string) error
	StopContainer(context.Context, string) error
	RestartContainer(context.Context, string) error
}

type containersLoadedMsg struct {
	containers []domain.Container
	err        error
}

type containerActionCompletedMsg struct {
	action pendingAction
	err    error
}

type externalCommandFinishedMsg struct {
	kind string
	err  error
}

type templateWriteCompletedMsg struct {
	path string
	err  error
}

type configLoadedMsg struct {
	candidates []devconfig.ConfigCandidate
	document   devconfig.ConfigDocument
	err        error
}

type configSavedMsg struct {
	path string
	err  error
}

type featureCatalogLoadedMsg struct {
	features []devconfig.Feature
	err      error
}

type modalMode int

const (
	modalNone modalMode = iota
	modalSearch
	modalFilter
	modalConfirmAction
	modalConfirmTemplateWrite
	modalConfigInput
	modalConfigFeature
	modalConfigCandidate
	modalConfirmConfigSave
	modalConfirmConfigDiscard
)

type activeTab int

const (
	tabContainers activeTab = iota
	tabTemplates
	tabConfig
)

type actionKind int

const (
	actionNone actionKind = iota
	actionStart
	actionStop
	actionRestart
)

type pendingAction struct {
	kind          actionKind
	containerID   string
	containerName string
}

type configEditKind int

const (
	configEditNone configEditKind = iota
	configEditName
	configEditImage
	configEditRemoteUser
	configEditPostCreate
	configEditAddFeatureManual
	configEditFeatureOptions
	configEditAddExtension
)

type configRowKind int

const (
	configRowField configRowKind = iota
	configRowAddFeature
	configRowFeature
	configRowAddExtension
	configRowExtension
)

type configRow struct {
	kind  configRowKind
	edit  configEditKind
	label string
	value string
	id    string
	index int
}

// Model is the Bubble Tea application state.
type Model struct {
	provider  ContainerService
	targetDir string

	containers       []domain.Container
	visible          []domain.Container
	templates        []devtemplates.Template
	visibleTemplates []devtemplates.Template

	keys                keyMap
	styles              styles
	help                help.Model
	searchInput         textinput.Model
	templateSearchInput textinput.Model
	configInput         textinput.Model
	featureSearchInput  textinput.Model

	activeTab     activeTab
	filterMode    containerfilter.Mode
	query         string
	templateQuery string
	featureQuery  string
	modal         modalMode

	filterCursor             int
	pendingAction            pendingAction
	pendingTemplate          devtemplates.Template
	pendingTemplatePath      string
	pendingTemplateOverwrite bool
	pendingConfigTab         activeTab
	configEdit               configEditKind
	configEditFeatureID      string
	configEditExtensionIndex int
	configCandidateCursor    int

	cursor         int
	offset         int
	templateCursor int
	templateOffset int
	configCursor   int
	configOffset   int
	featureCursor  int
	featureOffset  int
	width          int
	height         int

	loading                 bool
	actionInProgress        bool
	templateWriteInProgress bool
	configLoading           bool
	configSaving            bool
	featureCatalogLoading   bool
	err                     error
	actionStatus            string
	actionErr               error
	configErr               error
	configCandidates        []devconfig.ConfigCandidate
	configDoc               devconfig.ConfigDocument
	configDirty             bool
	configCandidatePicked   bool
	featureCatalog          []devconfig.Feature
	visibleFeatures         []devconfig.Feature
}

// New returns a TUI model wired to a container provider.
func New(provider ContainerService) Model {
	styles := newStyles()
	searchInput := textinput.New()
	searchInput.Prompt = "/ "
	searchInput.Placeholder = "name, image, status, path, label..."
	searchInput.CharLimit = 256
	searchInput.PromptStyle = styles.Subtle
	searchInput.TextStyle = styles.Search
	searchInput.PlaceholderStyle = styles.Subtle
	searchInput.Blur()

	templateSearchInput := textinput.New()
	templateSearchInput.Prompt = "/ "
	templateSearchInput.Placeholder = "language, stack, feature..."
	templateSearchInput.CharLimit = 256
	templateSearchInput.PromptStyle = styles.Subtle
	templateSearchInput.TextStyle = styles.Search
	templateSearchInput.PlaceholderStyle = styles.Subtle
	templateSearchInput.Blur()

	configInput := textinput.New()
	configInput.Prompt = "> "
	configInput.Placeholder = "value"
	configInput.CharLimit = 512
	configInput.PromptStyle = styles.Subtle
	configInput.TextStyle = styles.Search
	configInput.PlaceholderStyle = styles.Subtle
	configInput.Blur()

	featureSearchInput := textinput.New()
	featureSearchInput.Prompt = "/ "
	featureSearchInput.Placeholder = "feature ID, name, or manual ghcr.io/..."
	featureSearchInput.CharLimit = 512
	featureSearchInput.PromptStyle = styles.Subtle
	featureSearchInput.TextStyle = styles.Search
	featureSearchInput.PlaceholderStyle = styles.Subtle
	featureSearchInput.Blur()

	targetDir, err := os.Getwd()
	if err != nil {
		targetDir = "."
	}
	catalog := devtemplates.Catalog()

	return Model{
		provider:                 provider,
		targetDir:                targetDir,
		templates:                catalog,
		visibleTemplates:         devtemplates.Filter(catalog, ""),
		keys:                     newKeyMap(),
		styles:                   styles,
		help:                     help.New(),
		searchInput:              searchInput,
		templateSearchInput:      templateSearchInput,
		configInput:              configInput,
		featureSearchInput:       featureSearchInput,
		configEditExtensionIndex: -1,
		filterMode:               containerfilter.ModeAll,
		loading:                  true,
		configLoading:            true,
		featureCatalogLoading:    true,
	}
}

// Init starts the initial asynchronous Docker container load.
func (m Model) Init() tea.Cmd {
	return tea.Batch(loadContainers(m.provider), loadConfig(m.targetDir), loadFeatureCatalog())
}

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
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		m.pendingAction = pendingAction{}
		m.actionStatus = "Action cancelled"
		m.actionErr = nil
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
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
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		m.pendingTemplate = devtemplates.Template{}
		m.pendingTemplatePath = ""
		m.pendingTemplateOverwrite = false
		m.actionStatus = "Template write cancelled"
		m.actionErr = nil
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
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
		m.configInput.Blur()
		m.configEdit = configEditNone
		m.configEditFeatureID = ""
		m.configEditExtensionIndex = -1
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		m.applyConfigInputValue(m.configInput.Value())
		m.modal = modalNone
		m.configInput.Blur()
		m.configEdit = configEditNone
		m.configEditFeatureID = ""
		m.configEditExtensionIndex = -1
		m.ensureConfigCursorBounds()
		m.ensureConfigCursorVisible()
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
	case key.Matches(msg, m.keys.Confirm):
		m.addSelectedFeature()
		m.modal = modalNone
		m.featureSearchInput.Blur()
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
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		m.actionStatus = "Config save cancelled"
		m.actionErr = nil
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
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
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		m.pendingConfigTab = tabConfig
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
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
