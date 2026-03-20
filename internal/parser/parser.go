package parser

import (
	"regexp"
	"strings"

	"github.com/kwrkb/helptree/internal/model"
)

var (
	// Matches section headers like "Commands:", "Available Commands:", "Basic Commands (Beginner):",
	// "The commands are:"
	sectionHeaderRe = regexp.MustCompile(`^(?:The\s+)?([A-Za-z][\w\s()]*[\w)])\s*:`)
	// Matches a subcommand line: "  command-name    Description text" (spaces or tabs)
	subcommandRe = regexp.MustCompile(`^\s+(\S+)\s{2,}(.+)`)
	// Matches option with short and long: "  -v, --verbose string   Description"
	optShortLongRe = regexp.MustCompile(`^\s{2,}(-\w),\s+(--[\w-]+)(?:\s+(\S+))?\s{2,}(.+)`)
	// Matches option with long only: "      --verbose string   Description"
	optLongOnlyRe = regexp.MustCompile(`^\s{2,}(--[\w-]+)(?:\s+(\S+))?\s{2,}(.+)`)
	// Matches option with short only: "  -v   Description"
	optShortOnlyRe = regexp.MustCompile(`^\s{2,}(-\w)(?:\s+(\S+))?\s{2,}(.+)`)
	// Matches option flag on its own line (description on next line):
	//   "  -v, --verbose"  or  "  --output <file>"  or  "  -p, --plain..."
	optBareShortLongRe = regexp.MustCompile(`^\s{2,}(-\w),\s+(--[\w-]+)(?:\s+(\S+))?\s*$`)
	optBareLongRe      = regexp.MustCompile(`^\s{2,}(--[\w-]+)(?:\s+(\S+))?\s*$`)
	// Matches comma-separated command list: "    access, adduser, audit, bugs, ..."
	commaSepListRe = regexp.MustCompile(`^\s{2,}(\w[\w-]*(?:,\s*\w[\w-]*){2,}),?\s*$`)
	// Matches a bare subcommand name with no description: "  run"
	bareSubcommandRe = regexp.MustCompile(`^\s+([a-zA-Z][a-zA-Z0-9_-]*)\s*$`)
	// Matches usage line
	usageRe = regexp.MustCompile(`(?i)^usage:\s*(.*)`)
)

// descAppender is a pointer to the last parsed item's description,
// used to append continuation (wrapped) lines.
type descAppender struct {
	desc *string
	col  int // column where the description started (for detecting alignment)
}

func (a *descAppender) reset() {
	a.desc = nil
	a.col = 0
}

func (a *descAppender) set(desc *string, col int) {
	a.desc = desc
	a.col = col
}

// appendContinuation tries to append a wrapped line to the last item.
// Returns true if the line was consumed as a continuation.
func (a *descAppender) appendContinuation(line string) bool {
	if a.desc == nil {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		a.reset()
		return false
	}
	// A continuation line must be indented and NOT look like a new item
	indent := leadingSpaces(line)
	if indent < 2 {
		return false
	}
	// If the line starts with a flag, it's a new option, not a continuation
	if strings.HasPrefix(trimmed, "-") {
		return false
	}
	// col == 0 means the previous item had no inline description (bare flag).
	// Accept any indented non-flag line as the description.
	if a.col == 0 && indent >= 4 {
		if *a.desc == "" {
			*a.desc = trimmed
		} else {
			*a.desc += " " + trimmed
		}
		return true
	}
	// Heuristic: continuation lines are typically indented at or beyond
	// the description column of the previous item, or deeply indented.
	if a.col > 0 && indent >= a.col {
		*a.desc += " " + trimmed
		return true
	}
	if indent >= 20 {
		*a.desc += " " + trimmed
		return true
	}
	return false
}

// Parse parses a --help output string and returns a Node.
func Parse(name, helpText string) *model.Node {
	node := &model.Node{
		Name:   name,
		Loaded: true,
	}

	lines := strings.Split(helpText, "\n")
	if len(lines) == 0 {
		return node
	}

	// Extract usage (handles both "Usage: cmd ..." and "Usage:\n  cmd ...")
	for i, line := range lines {
		if m := usageRe.FindStringSubmatch(line); m != nil {
			usage := strings.TrimSpace(m[1])
			if usage != "" {
				node.Usage = usage
			} else {
				// "Usage:" on its own line — collect next indented lines
				for j := i + 1; j < len(lines); j++ {
					next := strings.TrimSpace(lines[j])
					if next == "" {
						break
					}
					if sectionHeaderRe.MatchString(lines[j]) {
						break
					}
					if node.Usage != "" {
						node.Usage += "\n  " + next
					} else {
						node.Usage = next
					}
				}
			}
			break
		}
	}

	// Extract description
	node.Description = extractDescription(lines)

	// Parse sections with lastParsedItem buffering.
	// Start as "unknown" — if subcommand patterns match consecutively
	// without a section header, infer an implicit commands section.
	currentSection := "unknown"
	var last descAppender
	consecutiveSubcmdHits := 0 // tracks consecutive subcommand-like lines in unknown state
	const inferThreshold = 2   // promote to "commands" after N consecutive matches

	for _, line := range lines {
		// Check for section header
		if m := sectionHeaderRe.FindStringSubmatch(line); m != nil {
			currentSection = categorizeSection(m[1])
			last.reset()
			consecutiveSubcmdHits = 0
			continue
		}
		// "The commands are:" style headers
		if strings.HasPrefix(strings.TrimSpace(line), "The commands are") ||
			strings.HasPrefix(strings.TrimSpace(line), "The topics are") {
			currentSection = "commands"
			last.reset()
			consecutiveSubcmdHits = 0
			continue
		}

		switch currentSection {
		case "commands":
			if m := subcommandRe.FindStringSubmatch(line); m != nil {
				child := &model.Node{
					Name:        m[1],
					Description: strings.TrimSpace(m[2]),
				}
				node.Children = append(node.Children, child)
				descStart := strings.Index(line, m[2])
				last.set(&child.Description, descStart)
			} else if last.appendContinuation(line) {
				// wrapped description line consumed
			} else if m := bareSubcommandRe.FindStringSubmatch(line); m != nil {
				child := &model.Node{Name: m[1]}
				node.Children = append(node.Children, child)
				last.reset()
			} else if names := parseCommaSeparatedList(line); len(names) > 0 {
				for _, name := range names {
					node.Children = append(node.Children, &model.Node{Name: name})
				}
				last.reset()
			} else {
				last.reset()
			}

		case "options":
			if opt, col, ok := parseOptionLine(line); ok {
				node.Options = append(node.Options, opt)
				last.set(&node.Options[len(node.Options)-1].Description, col)
			} else if opt, ok := parseBareOptionLine(line); ok {
				// Flag on its own line — description will come on next line
				node.Options = append(node.Options, opt)
				last.set(&node.Options[len(node.Options)-1].Description, 0)
			} else if last.appendContinuation(line) {
				// wrapped description line consumed
			} else {
				last.reset()
			}

		default:
			// "unknown" or "other" — try to infer section type
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				consecutiveSubcmdHits = 0
				last.reset()
				continue
			}

			// Try option first (options can appear without a header too)
			if _, _, ok := parseOptionLine(line); ok {
				currentSection = "options"
				opt, col, _ := parseOptionLine(line)
				node.Options = append(node.Options, opt)
				last.set(&node.Options[len(node.Options)-1].Description, col)
				continue
			}

			// Try subcommand pattern (with description)
			if m := subcommandRe.FindStringSubmatch(line); m != nil {
				consecutiveSubcmdHits++
				if consecutiveSubcmdHits >= inferThreshold {
					currentSection = "commands"
				}
				child := &model.Node{
					Name:        m[1],
					Description: strings.TrimSpace(m[2]),
				}
				node.Children = append(node.Children, child)
				descStart := strings.Index(line, m[2])
				last.set(&child.Description, descStart)
			} else if m := bareSubcommandRe.FindStringSubmatch(line); m != nil {
				// Bare command name with no description
				consecutiveSubcmdHits++
				if consecutiveSubcmdHits >= inferThreshold {
					currentSection = "commands"
				}
				child := &model.Node{Name: m[1]}
				node.Children = append(node.Children, child)
				last.reset()
			} else {
				consecutiveSubcmdHits = 0
				if !last.appendContinuation(line) {
					last.reset()
				}
			}
		}
	}

	// If we inferred commands in unknown state but never reached the threshold,
	// keep whatever was collected (even 1 match might be valid in headerless output).

	return node
}

func extractDescription(lines []string) string {
	// Strategy: find the first non-empty, non-usage, non-section-header line
	// that appears before any section or after the usage line.
	pastUsage := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if usageRe.MatchString(line) {
			pastUsage = true
			continue
		}
		if sectionHeaderRe.MatchString(line) {
			if pastUsage {
				return ""
			}
			continue
		}
		if pastUsage && !strings.HasPrefix(trimmed, "-") && !subcommandRe.MatchString(line) {
			return trimmed
		}
		if !pastUsage && !strings.HasPrefix(trimmed, "-") {
			return trimmed
		}
	}
	return ""
}

func categorizeSection(header string) string {
	h := strings.ToLower(header)
	switch {
	case strings.Contains(h, "command"):
		return "commands"
	case strings.Contains(h, "option"), strings.Contains(h, "flag"):
		return "options"
	default:
		return "other"
	}
}

// parseOptionLine tries to parse a line as an option definition.
// Returns the option, the column where the description starts, and whether it matched.
func parseOptionLine(line string) (model.Option, int, bool) {
	if m := optShortLongRe.FindStringSubmatchIndex(line); m != nil {
		sm := optShortLongRe.FindStringSubmatch(line)
		return model.Option{
			Short:       sm[1],
			Long:        sm[2],
			Arg:         classifyArg(sm[3]),
			Description: strings.TrimSpace(sm[4]),
		}, m[8], true // m[8] is the start index of capture group 4 (description)
	}
	if m := optLongOnlyRe.FindStringSubmatchIndex(line); m != nil {
		sm := optLongOnlyRe.FindStringSubmatch(line)
		return model.Option{
			Long:        sm[1],
			Arg:         classifyArg(sm[2]),
			Description: strings.TrimSpace(sm[3]),
		}, m[6], true
	}
	if m := optShortOnlyRe.FindStringSubmatchIndex(line); m != nil {
		sm := optShortOnlyRe.FindStringSubmatch(line)
		return model.Option{
			Short:       sm[1],
			Arg:         classifyArg(sm[2]),
			Description: strings.TrimSpace(sm[3]),
		}, m[6], true
	}
	return model.Option{}, 0, false
}

// parseBareOptionLine matches a flag on its own line with no inline description.
// e.g. "  -v, --verbose" or "  --output <file>" or "  -p, --plain..."
func parseBareOptionLine(line string) (model.Option, bool) {
	if m := optBareShortLongRe.FindStringSubmatch(line); m != nil {
		return model.Option{
			Short: m[1],
			Long:  m[2],
			Arg:   classifyArg(m[3]),
		}, true
	}
	if m := optBareLongRe.FindStringSubmatch(line); m != nil {
		return model.Option{
			Long: m[1],
			Arg:  classifyArg(m[2]),
		}, true
	}
	return model.Option{}, false
}

// parseCommaSeparatedList extracts command names from comma-separated lists.
// e.g. "    access, adduser, audit, bugs, cache, ci, completion,"
func parseCommaSeparatedList(line string) []string {
	if !commaSepListRe.MatchString(line) {
		return nil
	}
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimRight(trimmed, ",")
	parts := strings.Split(trimmed, ",")
	var names []string
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// classifyArg determines if a token after a flag is an argument type or part of description.
func classifyArg(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	switch strings.ToLower(s) {
	case "string", "int", "uint", "float", "duration", "value", "path", "file", "dir",
		"url", "name", "key", "list", "map", "stringarray", "strings":
		return s
	}
	if !strings.Contains(s, " ") && strings.ToLower(s) == s && len(s) <= 16 {
		return s
	}
	return ""
}

// leadingSpaces returns the number of leading space/tab characters.
// Tabs count as 8 spaces (common terminal default).
func leadingSpaces(line string) int {
	n := 0
	for _, ch := range line {
		switch ch {
		case ' ':
			n++
		case '\t':
			n += 8
		default:
			return n
		}
	}
	return n
}
