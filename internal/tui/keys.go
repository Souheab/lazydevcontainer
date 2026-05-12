package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	Left          key.Binding
	Right         key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	Home          key.Binding
	End           key.Binding
	Top           key.Binding
	Bottom        key.Binding
	Search        key.Binding
	Cancel        key.Binding
	Confirm       key.Binding
	Quit          key.Binding
	Refresh       key.Binding
	StartStop     key.Binding
	Restart       key.Binding
	Shell         key.Binding
	Editor        key.Binding
	ContainersTab key.Binding
	TemplatesTab  key.Binding
	FilterMenu    key.Binding
	FilterAll     key.Binding
	FilterDev     key.Binding
	FilterDocker  key.Binding
	CycleFilter   key.Binding
	ReverseFilter key.Binding
	Help          key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:            key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:          key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:          key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "prev filter")),
		Right:         key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next filter")),
		PageUp:        key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup/b", "page up")),
		PageDown:      key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
		Home:          key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		End:           key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
		Top:           key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:        key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Search:        key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Cancel:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		Confirm:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Refresh:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		StartStop:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start/stop")),
		Restart:       key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "restart")),
		Shell:         key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "shell")),
		Editor:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		ContainersTab: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "containers")),
		TemplatesTab:  key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "templates")),
		FilterMenu:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
		FilterAll:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all")),
		FilterDev:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "devcontainers")),
		FilterDocker:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "containers")),
		CycleFilter:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "filter")),
		ReverseFilter: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev filter")),
		Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Search, k.TemplatesTab, k.ContainersTab, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom},
		{k.TemplatesTab, k.ContainersTab, k.FilterMenu, k.FilterAll, k.FilterDev, k.FilterDocker, k.CycleFilter},
		{k.Search, k.StartStop, k.Restart, k.Shell, k.Editor, k.Refresh},
		{k.Cancel, k.Help, k.Quit},
	}
}
