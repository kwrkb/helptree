package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kwrkb/helptree/internal/runner"
	"github.com/kwrkb/helptree/internal/tui"
)

const version = "0.1.1"

func printHelp(w io.Writer) {
	fmt.Fprintf(w, "helptree v%s — Interactive CLI help viewer\n\n", version)
	fmt.Fprintf(w, "Usage: helptree <command>\n\n")
	fmt.Fprintf(w, "Explore any CLI tool's help output as an interactive tree.\n")
	fmt.Fprintf(w, "Subcommand help is loaded on demand.\n\n")
	fmt.Fprintf(w, "Options:\n")
	fmt.Fprintf(w, "  -h, --help       Show this help message\n")
	fmt.Fprintf(w, "  -v, --version    Show version\n\n")
	fmt.Fprintf(w, "Keybindings (press ? in TUI for full list):\n")
	fmt.Fprintf(w, "  j/k              Navigate up/down\n")
	fmt.Fprintf(w, "  Enter/l          Expand / load subcommand\n")
	fmt.Fprintf(w, "  h                Collapse\n")
	fmt.Fprintf(w, "  /                Search\n")
	fmt.Fprintf(w, "  q                Quit\n\n")
	fmt.Fprintf(w, "Examples:\n")
	fmt.Fprintf(w, "  helptree docker\n")
	fmt.Fprintf(w, "  helptree kubectl\n")
	fmt.Fprintf(w, "  helptree git\n")
}

func main() {
	if len(os.Args) < 2 {
		printHelp(os.Stderr)
		os.Exit(1)
	}

	if os.Args[1] == "--help" || os.Args[1] == "-h" {
		printHelp(os.Stdout)
		return
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
