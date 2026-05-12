package tui

import (
	"context"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

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
	configRowFeatures
	configRowFeature
	configRowAddExtension
	configRowExtension
)

type configFeatureModalItemKind int

const (
	configFeatureModalConfigured configFeatureModalItemKind = iota
	configFeatureModalManual
	configFeatureModalCatalog
)

type configFeatureModalItem struct {
	kind    configFeatureModalItemKind
	id      string
	name    string
	options map[string]any
}

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
	configInputReturnFeature bool
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
