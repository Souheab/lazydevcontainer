package tui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	App          lipgloss.Style
	Pane         lipgloss.Style
	ActivePane   lipgloss.Style
	PaneBorder   lipgloss.Style
	ActiveBorder lipgloss.Style
	PaneTitle    lipgloss.Style
	Modal        lipgloss.Style
	ModalTitle   lipgloss.Style
	FeatureModal lipgloss.Style
	FeaturePanel lipgloss.Style
	InputFrame   lipgloss.Style
	SectionTitle lipgloss.Style
	FeatureRow   lipgloss.Style
	ConfirmKey   lipgloss.Style
	Title        lipgloss.Style
	Subtle       lipgloss.Style
	Header       lipgloss.Style
	Search       lipgloss.Style
	Row          lipgloss.Style
	SelectedRow  lipgloss.Style
	BadgeDev     lipgloss.Style
	BadgeDocker  lipgloss.Style
	Name         lipgloss.Style
	Muted        lipgloss.Style
	Path         lipgloss.Style
	Status       lipgloss.Style
	Error        lipgloss.Style
	Empty        lipgloss.Style
	Help         lipgloss.Style
}

func newStyles() styles {
	paneBorder := lipgloss.RoundedBorder()
	activeBorder := lipgloss.RoundedBorder()

	return styles{
		App:          lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Pane:         lipgloss.NewStyle().Border(paneBorder).BorderForeground(lipgloss.Color("252")).Padding(0, 1),
		ActivePane:   lipgloss.NewStyle().Border(activeBorder).BorderForeground(lipgloss.Color("114")).Padding(0, 1),
		PaneBorder:   lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		ActiveBorder: lipgloss.NewStyle().Foreground(lipgloss.Color("114")),
		PaneTitle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("114")),
		Modal:        lipgloss.NewStyle().Border(paneBorder).BorderForeground(lipgloss.Color("75")).Padding(1, 2),
		ModalTitle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")),
		FeatureModal: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("255")).Padding(1, 2),
		FeaturePanel: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("255")).Padding(0, 1),
		InputFrame:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("255")).Padding(0, 1),
		SectionTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")),
		FeatureRow:   lipgloss.NewStyle().Padding(0, 1),
		ConfirmKey:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("114")),
		Title:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75")),
		Subtle:       lipgloss.NewStyle().Foreground(lipgloss.Color("248")),
		Header:       lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Search:       lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("238")),
		Row:          lipgloss.NewStyle(),
		SelectedRow:  lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("18")).Bold(true),
		BadgeDev:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("114")).Padding(0, 1),
		BadgeDocker:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("75")).Padding(0, 1),
		Name:         lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")),
		Muted:        lipgloss.NewStyle().Foreground(lipgloss.Color("248")),
		Path:         lipgloss.NewStyle().Foreground(lipgloss.Color("151")),
		Status:       lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
		Error:        lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		Empty:        lipgloss.NewStyle().Foreground(lipgloss.Color("248")).Italic(true),
		Help:         lipgloss.NewStyle().Foreground(lipgloss.Color("248")),
	}
}
