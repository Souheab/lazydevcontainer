package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Souheab/lazydevcontainer/internal/docker"
	"github.com/Souheab/lazydevcontainer/internal/tui"
)

func main() {
	provider, err := docker.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lazydc: %v\n", err)
		os.Exit(1)
	}

	program := tea.NewProgram(
		tui.New(provider),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "lazydc: %v\n", err)
		os.Exit(1)
	}
}
