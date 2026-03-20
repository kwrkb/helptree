package tui

import (
	"fmt"
	"strings"

	"github.com/kwrkb/helptree/internal/model"
)

// renderDetail renders the detail pane for the selected node with scroll support.
func renderDetail(node *model.Node, width, height, scroll int) string {
	if node == nil {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(fmt.Sprintf("╭─ %s ─╮\n", node.Name))
	b.WriteString("\n")

	// Description
	if node.Description != "" {
		b.WriteString(wrapText(node.Description, width))
		b.WriteString("\n\n")
	}

	// Usage
	if node.Usage != "" {
		b.WriteString("Usage:\n")
		b.WriteString("  " + node.Usage + "\n\n")
	}

	// Subcommands
	if len(node.Children) > 0 {
		b.WriteString(fmt.Sprintf("Subcommands (%d):\n", len(node.Children)))
		for _, child := range node.Children {
			name := child.Name
			// Show just the last part of the name
			if parts := strings.Fields(name); len(parts) > 0 {
				name = parts[len(parts)-1]
			}
			line := fmt.Sprintf("  %-20s %s", name, child.Description)
			if len(line) > width && width > 4 {
				line = line[:width-4] + "..."
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
	}

	// Options
	if len(node.Options) > 0 {
		b.WriteString(fmt.Sprintf("Options (%d):\n", len(node.Options)))
		for _, opt := range node.Options {
			flag := opt.FullFlag()
			line := fmt.Sprintf("  %-28s %s", flag, opt.Description)
			if len(line) > width && width > 4 {
				line = line[:width-4] + "..."
			}
			b.WriteString(line + "\n")
		}
	}

	// Status
	if !node.Loaded {
		b.WriteString("\n  [Press Enter to load subcommands]")
	}

	// Apply scroll
	lines := strings.Split(b.String(), "\n")
	if scroll > len(lines)-1 {
		scroll = len(lines) - 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > 0 && scroll < len(lines) {
		lines = lines[scroll:]
	}

	// Add scroll indicator
	if scroll > 0 {
		indicator := fmt.Sprintf("  ↑ %d more lines above", scroll)
		lines = append([]string{indicator}, lines...)
	}

	return strings.Join(lines, "\n")
}

// wrapText wraps text to the given width.
func wrapText(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}

	var lines []string
	for len(text) > width {
		// Find the last space before width
		idx := strings.LastIndex(text[:width], " ")
		if idx <= 0 {
			idx = width
		}
		lines = append(lines, text[:idx])
		text = strings.TrimSpace(text[idx:])
	}
	if text != "" {
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}
