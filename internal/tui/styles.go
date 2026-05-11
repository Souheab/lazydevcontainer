package tui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	App         lipgloss.Style
	Title       lipgloss.Style
	Subtle      lipgloss.Style
	Header      lipgloss.Style
	Search      lipgloss.Style
	Divider     lipgloss.Style
	Row         lipgloss.Style
	SelectedRow lipgloss.Style
	BadgeDev    lipgloss.Style
	BadgeDocker lipgloss.Style
	Name        lipgloss.Style
	Muted       lipgloss.Style
	Path        lipgloss.Style
	Status      lipgloss.Style
	Error       lipgloss.Style
	Empty       lipgloss.Style
	Help        lipgloss.Style
}

func newStyles() styles {
	return styles{
		App:         lipgloss.NewStyle().Padding(0, 1),
		Title:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
		Subtle:      lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Header:      lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Search:      lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("238")).Padding(0, 1),
		Divider:     lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		Row:         lipgloss.NewStyle().Padding(0, 1),
		SelectedRow: lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("236")),
		BadgeDev:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("114")).Padding(0, 1),
		BadgeDocker: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("75")).Padding(0, 1),
		Name:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")),
		Muted:       lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Path:        lipgloss.NewStyle().Foreground(lipgloss.Color("151")),
		Status:      lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
		Error:       lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		Empty:       lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true).Padding(2, 1),
		Help:        lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
	}
}
