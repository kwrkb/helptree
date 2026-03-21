package parser

import (
	"regexp"
	"strings"

	"github.com/kwrkb/helptree/internal/model"
)

// Key-only regexes for parsing the left column of option tables.
// These are simpler than the full-line regexes because column detection
// already split the key from the description.
var (
	// Matches "-v, --verbose string" or "-v, --verbose" (short + long + optional arg)
	keyShortLongRe = regexp.MustCompile(`^\s*(-\w),\s+(--[\w-]+)(?:\s+(\S+))?`)
	// Matches "    --verbose string" or "    --verbose" (long only + optional arg)
	keyLongOnlyRe = regexp.MustCompile(`^\s*(--[\w-]+)(?:\s+(\S+))?`)
	// Matches "  -v string" or "  -v" (short only + optional arg)
	keyShortOnlyRe = regexp.MustCompile(`^\s*(-\w)(?:\s+(\S+))?`)
	// Matches "-z, -j, -J, --lzma" (multiple flags comma-separated)
	keyMultiFlagRe = regexp.MustCompile(`^\s*((?:-\w|--[\w-]+)(?:,\s*(?:-\w|--[\w-]+))+)`)
)

// parseCommandBlock extracts subcommands from a classified block.
func parseCommandBlock(node *model.Node, b *Block, rootName string) {
	if b.Kind == BlockSingle {
		parseCommandBlockSingle(node, b)
		return
	}
	if b.Kind != BlockTable {
		return
	}

	var lastDesc *string

	for _, line := range b.Lines {
		// Split at description column first (before prefix stripping)
		key, desc := splitAtColumn(line, b.DescCol)
		keyTrimmed := strings.TrimSpace(key)
		descTrimmed := strings.TrimSpace(desc)

		// Strip binary name prefix from the key part
		keyTrimmed = strings.TrimSpace(stripBinaryPrefix("  "+keyTrimmed, rootName))

		if keyTrimmed == "" {
			// Continuation line — append to previous description
			if lastDesc != nil && descTrimmed != "" {
				*lastDesc += " " + descTrimmed
			}
			continue
		}

		// Handle dash separator: strip trailing " - " from key if present
		if b.Separator == SepDash {
			keyTrimmed = strings.TrimRight(keyTrimmed, " ")
			keyTrimmed = strings.TrimSuffix(keyTrimmed, "-")
			keyTrimmed = strings.TrimSuffix(keyTrimmed, "–")
			keyTrimmed = strings.TrimSuffix(keyTrimmed, "—")
			keyTrimmed = strings.TrimSpace(keyTrimmed)
		}

		// Extract command name: first word of the key
		name := extractCommandName(keyTrimmed)
		if name == "" {
			lastDesc = nil
			continue
		}

		child := &model.Node{
			Name:        name,
			Description: descTrimmed,
		}
		node.Children = append(node.Children, child)
		lastDesc = &child.Description
	}
}

// parseCommandBlockSingle handles single-column command blocks (bare names, comma lists).
func parseCommandBlockSingle(node *model.Node, b *Block) {
	for _, line := range b.Lines {
		// Try comma-separated list first
		if names := parseCommaSeparatedList(line); len(names) > 0 {
			for _, name := range names {
				node.Children = append(node.Children, &model.Node{Name: name})
			}
			continue
		}
		// Bare subcommand name
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.Contains(trimmed, " ") {
			node.Children = append(node.Children, &model.Node{Name: trimmed})
		}
	}
}

// parseOptionBlock extracts options from a classified block.
func parseOptionBlock(node *model.Node, b *Block) {
	// Pre-scan: check for special formats (inline multi-options, bracket-style)
	// and track which lines were consumed so we can skip them in the main loop.
	specialLines := map[int]bool{}
	for i, line := range b.Lines {
		if inlineOpts := parseInlineMultiOptions(line); len(inlineOpts) > 0 {
			node.Options = append(node.Options, inlineOpts...)
			specialLines[i] = true
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if bracketOpts := parseBracketOptions(line); len(bracketOpts) > 0 {
				node.Options = append(node.Options, bracketOpts...)
				specialLines[i] = true
			}
		}
	}

	// For non-table blocks (bare flags with indented descriptions),
	// use bare-flag parsing mode
	if b.Kind != BlockTable {
		parseOptionBlockBare(node, b)
		return
	}

	var lastDesc *string

	for i, line := range b.Lines {
		// Skip lines already consumed by special format handling
		if specialLines[i] {
			lastDesc = nil
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			lastDesc = nil
			continue
		}

		// Split at description column
		key, desc := splitAtColumn(line, b.DescCol)
		keyTrimmed := strings.TrimSpace(key)
		descTrimmed := strings.TrimSpace(desc)

		if keyTrimmed == "" {
			// Continuation line
			if lastDesc != nil && descTrimmed != "" {
				*lastDesc += " " + descTrimmed
			}
			continue
		}

		// Handle colon separator: strip trailing " : " or ": " from key
		if b.Separator == SepColon {
			keyTrimmed = strings.TrimRight(keyTrimmed, " ")
			keyTrimmed = strings.TrimSuffix(keyTrimmed, ":")
			keyTrimmed = strings.TrimSpace(keyTrimmed)
		}

		opts := parseOptionKey(keyTrimmed, descTrimmed)
		if len(opts) > 0 {
			for _, o := range opts {
				node.Options = append(node.Options, o)
			}
			lastDesc = &node.Options[len(node.Options)-1].Description
		} else {
			lastDesc = nil
		}
	}
}

// parseOptionBlockBare handles Clap/Rust-style bare flags where the flag
// is on its own line and the description follows on indented lines below.
// e.g.:
//
//	-A, --show-all
//	        Show non-printable characters
//	    --nonprintable-notation <notation>
//	        Set notation for non-printable characters.
func parseOptionBlockBare(node *model.Node, b *Block) {
	var lastDesc *string
	baseIndent := -1 // indent of flag lines

	for _, line := range b.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		indent := leadingSpaces(line)

		// If this line starts with "-", it's a flag line
		if strings.HasPrefix(trimmed, "-") {
			opts := parseOptionKey(trimmed, "")
			if len(opts) > 0 {
				for _, o := range opts {
					node.Options = append(node.Options, o)
				}
				lastDesc = &node.Options[len(node.Options)-1].Description
				if baseIndent < 0 {
					baseIndent = indent
				}
			} else {
				lastDesc = nil
			}
			continue
		}

		// Indented line after a flag: continuation/description
		if lastDesc != nil && indent > baseIndent {
			if *lastDesc != "" {
				*lastDesc += " " + trimmed
			} else {
				*lastDesc = trimmed
			}
			continue
		}

		// Non-flag, non-continuation: skip
		lastDesc = nil
	}
}

// parseOptionKey parses an option key string (left column) into one or more Options.
func parseOptionKey(key, desc string) []model.Option {
	// Short + long: "-v, --verbose string" (must come before multi-flag)
	if m := keyShortLongRe.FindStringSubmatch(key); m != nil {
		return []model.Option{{
			Short:       m[1],
			Long:        m[2],
			Arg:         classifyArg(m[3]),
			Description: desc,
		}}
	}

	// Multi-flag: "-z, -j, -J, --lzma" (3+ flags sharing a description)
	if m := keyMultiFlagRe.FindStringSubmatch(key); m != nil {
		flags := strings.Split(m[1], ",")
		if len(flags) >= 3 {
			var opts []model.Option
			for _, f := range flags {
				f = strings.TrimSpace(f)
				if strings.HasPrefix(f, "--") {
					opts = append(opts, model.Option{Long: f, Description: desc})
				} else if strings.HasPrefix(f, "-") && len(f) == 2 {
					opts = append(opts, model.Option{Short: f, Description: desc})
				}
			}
			return opts
		}
	}

	// Long only: "--verbose string"
	if m := keyLongOnlyRe.FindStringSubmatch(key); m != nil {
		return []model.Option{{
			Long:        m[1],
			Arg:         classifyArg(m[2]),
			Description: desc,
		}}
	}

	// Short only: "-v" or "-v string"
	if m := keyShortOnlyRe.FindStringSubmatch(key); m != nil {
		return []model.Option{{
			Short:       m[1],
			Arg:         classifyArg(m[2]),
			Description: desc,
		}}
	}

	return nil
}

// splitAtColumn splits a line into key (left of descCol) and description (right of descCol).
// If the line is shorter than descCol, the description is empty.
func splitAtColumn(line string, descCol int) (string, string) {
	if descCol <= 0 || descCol >= len(line) {
		return line, ""
	}
	return line[:descCol], line[descCol:]
}

// extractCommandName extracts the command name from a key column string.
// Handles: "command", "command [flags]", "command <args>", "[query..]"
func extractCommandName(key string) string {
	fields := strings.Fields(key)
	if len(fields) == 0 {
		return ""
	}
	name := fields[0]
	// Filter out option-like strings
	if strings.HasPrefix(name, "-") {
		return ""
	}
	return name
}
