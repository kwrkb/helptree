package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kwrkb/helptree/internal/runner"
	"github.com/kwrkb/helptree/internal/tui"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "helptree v%s — Interactive CLI help viewer\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: helptree <command>\n")
		fmt.Fprintf(os.Stderr, "Example: helptree docker\n")
		os.Exit(1)
	}

	if os.Args[1] == "--version" || os.Args[1] == "-v" {
		fmt.Printf("helptree v%s\n", version)
		return
	}

	cmdName := os.Args[1]

	// Check if command exists
	if _, err := exec.LookPath(cmdName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: command %q not found in PATH\n", cmdName)
		os.Exit(1)
	}

	// Run initial --help
	root, err := runner.RunHelp(cmdName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get help for %q: %v\n", cmdName, err)
		os.Exit(1)
	}

	model := tui.New(root, cmdName)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
