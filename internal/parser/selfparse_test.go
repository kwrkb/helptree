package parser

import (
	"testing"
)

func TestSelfParse(t *testing.T) {
	help := `helptree v0.1.0 — Interactive CLI help viewer

Usage: helptree <command>

Explore any CLI tool's help output as an interactive tree.
Subcommand help is loaded on demand.

Options:
  -h, --help       Show this help message
  -v, --version    Show version

Keybindings (press ? in TUI for full list):
  j/k              Navigate up/down
  Enter/l          Expand / load subcommand
  h                Collapse
  /                Search
  q                Quit

Examples:
  helptree docker
  helptree kubectl
  helptree git`

	node := Parse("helptree", help)

	if node.Name != "helptree" {
		t.Errorf("expected name 'helptree', got %q", node.Name)
	}

	// Should have parsed options
	if len(node.Options) < 2 {
		t.Errorf("expected at least 2 options, got %d", len(node.Options))
	}

	// Check -h/--help option
	foundHelp := false
	foundVersion := false
	for _, o := range node.Options {
		if o.Short == "-h" && o.Long == "--help" {
			foundHelp = true
		}
		if o.Short == "-v" && o.Long == "--version" {
			foundVersion = true
		}
	}
	if !foundHelp {
		t.Error("expected to find -h/--help option")
	}
	if !foundVersion {
		t.Error("expected to find -v/--version option")
	}
}
