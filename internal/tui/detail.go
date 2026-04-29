package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kwrkb/helptree/internal/model"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12"))

	subcmdNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("14"))

	flagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

	argStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	statusHintStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("240"))

	summaryTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("240")).
				Padding(0, 1)
)

const (
	subcmdNameCol = 20
	optionFlagCol = 30
)

// truncateWidth shortens s so its display width fits within width cells,
// appending "..." when truncation occurs. Width is computed via lipgloss.Width
// so multi-byte runes are handled correctly.
func truncateWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	if width <= 3 {
		if len(runes) > width {
			return string(runes[:width])
		}
		return s
	}
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i])
		if lipgloss.Width(candidate)+3 <= width {
			return candidate + "..."
		}
	}
	return "..."
}

// renderSummary renders the summary pane (name, description, usage).
func renderSummary(node *model.Node, width int) string {
	if node == nil {
		return ""
	}

	var b strings.Builder

	// Title with a background
	b.WriteString(summaryTitleStyle.Render(node.Name) + "\n")

	// Description
	if node.Description != "" {
		b.WriteString("\n")
		b.WriteString(descStyle.Render(wrapText(node.Description, width)))
		b.WriteString("\n")
	}

	// Usage
	if node.Usage != "" {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("Usage:") + "\n")
		for _, line := range strings.Split(node.Usage, "\n") {
			b.WriteString("  " + line + "\n")
		}
	}

	// Status
	if !node.Loaded {
		b.WriteString("\n" + statusHintStyle.Render("  [Press Enter to load subcommands]"))
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
		b.WriteString(headerStyle.Render(fmt.Sprintf("Subcommands (%d):", len(node.Children))) + "\n")
		for _, child := range node.Children {
			name := child.Name
			if parts := strings.Fields(name); len(parts) > 0 {
				name = parts[len(parts)-1]
			}

			nameW := lipgloss.Width(name)
			pad := subcmdNameCol - nameW
			if pad < 1 {
				pad = 1
			}

			descBudget := width - 2 - nameW - pad
			desc := truncateWidth(child.Description, descBudget)

			b.WriteString("  " + subcmdNameStyle.Render(name) + strings.Repeat(" ", pad) + descStyle.Render(desc) + "\n")
		}
	}

	// Options
	if len(node.Options) > 0 {
		if len(node.Children) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(headerStyle.Render(fmt.Sprintf("Options (%d):", len(node.Options))) + "\n")
		for _, opt := range node.Options {
			// Stylize flags and arguments. The unstyled form (used for width math)
			// must mirror the visible character layout below.
			var styledFlags, plainFlags string
			switch {
			case opt.Short != "" && opt.Long != "":
				styledFlags = flagStyle.Render(opt.Short) + ", " + flagStyle.Render(opt.Long)
				plainFlags = opt.Short + ", " + opt.Long
			case opt.Long != "":
				styledFlags = "    " + flagStyle.Render(opt.Long)
				plainFlags = "    " + opt.Long
			case opt.Short != "":
				styledFlags = flagStyle.Render(opt.Short)
				plainFlags = opt.Short
			}

			if opt.Arg != "" {
				styledFlags += " " + argStyle.Render(opt.Arg)
				plainFlags += " " + opt.Arg
			}

			flagW := lipgloss.Width(plainFlags)
			pad := optionFlagCol - flagW
			if pad < 1 {
				pad = 1
			}

			descBudget := width - 2 - flagW - pad
			desc := truncateWidth(opt.Description, descBudget)

			b.WriteString("  " + styledFlags + strings.Repeat(" ", pad) + descStyle.Render(desc) + "\n")
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

	// Bottom scroll indicator: trim to height and show remaining.
	// Need height >= 2 to fit at least one content line plus the indicator.
	if height >= 2 && len(lines) > height {
		remaining := len(lines) - height + 1 // +1 for indicator line itself
		lines = lines[:height-1]
		indicator := fmt.Sprintf("  ↓ %d more lines below (Ctrl+D)", remaining)
		lines = append(lines, indicator)
	} else if height > 0 && len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// wrapText wraps text to the given width, taking multi-byte characters into account
// and preserving existing newlines.
func wrapText(text string, width int) string {
	if width <= 0 {
		return ""
	}

	var finalLines []string
	paragraphs := strings.Split(text, "\n")

	for _, paragraph := range paragraphs {
		if paragraph == "" {
			finalLines = append(finalLines, "")
			continue
		}

		var currentLine strings.Builder
		currentWidth := 0

		words := strings.Fields(paragraph)
		for _, word := range words {
			wordW := lipgloss.Width(word)
			if currentWidth+wordW+1 > width && currentWidth > 0 {
				finalLines = append(finalLines, currentLine.String())
				currentLine.Reset()
				currentWidth = 0
			}

			if currentWidth > 0 {
				currentLine.WriteString(" ")
				currentWidth++
			}

			if wordW > width {
				runes := []rune(word)
				for _, r := range runes {
					rw := lipgloss.Width(string(r))
					if currentWidth+rw > width && currentWidth > 0 {
						finalLines = append(finalLines, currentLine.String())
						currentLine.Reset()
						currentWidth = 0
					}
					currentLine.WriteRune(r)
					currentWidth += rw
				}
			} else {
				currentLine.WriteString(word)
				currentWidth += wordW
			}
		}

		if currentLine.Len() > 0 {
			finalLines = append(finalLines, currentLine.String())
		}
	}

	return strings.Join(finalLines, "\n")
}
