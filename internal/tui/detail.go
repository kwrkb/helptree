package tui

import (
	"fmt"
	"strings"

	"github.com/kwrkb/helptree/internal/model"
)

// renderSummary renders the summary pane (name, description, usage).
func renderSummary(node *model.Node, width int) string {
	if node == nil {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(fmt.Sprintf("╭─ %s ─╮\n", node.Name))

	// Description
	if node.Description != "" {
		b.WriteString("\n")
		b.WriteString(wrapText(node.Description, width))
		b.WriteString("\n")
	}

	// Usage
	if node.Usage != "" {
		b.WriteString("\nUsage:\n")
		for _, line := range strings.Split(node.Usage, "\n") {
			b.WriteString("  " + line + "\n")
		}
	}

	// Status
	if !node.Loaded {
		b.WriteString("\n  [Press Enter to load subcommands]")
	}

	return b.String()
}

// renderDetail renders the detail pane (subcommands and options) with scroll support.
func renderDetail(node *model.Node, width, height, scroll int) string {
	if node == nil {
		return ""
	}

	var b strings.Builder

	// Subcommands
	if len(node.Children) > 0 {
		b.WriteString(fmt.Sprintf("Subcommands (%d):\n", len(node.Children)))
		for _, child := range node.Children {
			name := child.Name
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

	content := b.String()
	if content == "" {
		return ""
	}

	// Apply scroll
	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	if scroll > totalLines-1 {
		scroll = totalLines - 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > 0 && scroll < totalLines {
		lines = lines[scroll:]
	}

	// Top scroll indicator
	if scroll > 0 {
		indicator := fmt.Sprintf("  ↑ %d more lines above (Ctrl+U)", scroll)
		lines = append([]string{indicator}, lines...)
	}

	// Bottom scroll indicator: trim to height and show remaining
	if height > 0 && len(lines) > height {
		remaining := len(lines) - height + 1 // +1 for indicator line itself
		lines = lines[:height-1]
		indicator := fmt.Sprintf("  ↓ %d more lines below (Ctrl+D)", remaining)
		lines = append(lines, indicator)
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
