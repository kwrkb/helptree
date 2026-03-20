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
	// Matches uppercase section headers without colon: "  COMMANDS  ", "  FLAGS  " (glab style)
	uppercaseSectionRe = regexp.MustCompile(`^\s+([A-Z][A-Z ]{3,}[A-Z])\s*$`)
	// Matches a subcommand line: "  command-name    Description text" (spaces or tabs)
	subcommandRe = regexp.MustCompile(`^\s+(\S+)\s{2,}(.+)`)
	// Matches "  command - description" style (apt, systemctl, etc.)
	dashSepSubcmdRe = regexp.MustCompile(`^\s{2,}([a-zA-Z][a-zA-Z0-9_-]*)\s+[-–—]\s+(.+)`)
	// Matches subcommand with meta tokens: "  alias [command] [--flags]    Description" (glab style)
	// Requires first meta token to start with [ or < to avoid matching plain "name   desc" lines.
	subcommandWithMetaRe = regexp.MustCompile(`^\s+([a-zA-Z][\w-]*)\s+[\[<].*\s{2,}(\S.+)$`)
	// Matches option with short and long: "-v, --verbose string   Description"
	// Allows column-0 start (\s*) for tools like fvm that don't indent options.
	optShortLongRe = regexp.MustCompile(`^\s*(-\w),\s+(--[\w-]+)(?:\s+(\S+))?\s{2,}(.+)`)
	// Matches option with long only: "      --verbose string   Description"
	optLongOnlyRe = regexp.MustCompile(`^\s{2,}(--[\w-]+)(?:\s+(\S+))?\s{2,}(.+)`)
	// Matches option with short only: "  -v   Description"
	optShortOnlyRe = regexp.MustCompile(`^\s{2,}(-\w)(?:\s+(\S+))?\s{2,}(.+)`)
	// Matches option flag on its own line (description on next line):
	//   "-v, --verbose"  or  "  --output <file>"  or  "  -p, --plain..."
	// Allows column-0 start (\s*) for tools like fvm.
	optBareShortLongRe = regexp.MustCompile(`^\s*(-\w),\s+(--[\w-]+)(?:\s+(\S+))?\s*$`)
	optBareLongRe      = regexp.MustCompile(`^\s{2,}(--[\w-]+)(?:\s+(\S+))?\s*$`)
	// Matches comma-separated command list: "    access, adduser, audit, bugs, ..."
	commaSepListRe = regexp.MustCompile(`^\s{2,}(\w[\w-]*(?:,\s*\w[\w-]*){2,}),?\s*$`)
	// Matches a bare subcommand name with no description: "  run"
	bareSubcommandRe = regexp.MustCompile(`^\s+([a-zA-Z][a-zA-Z0-9_-]*)\s*$`)
	// Matches usage line
	usageRe = regexp.MustCompile(`(?i)^usage:\s*(.*)`)
	// Matches compact option groups in usage: [-abcdefg] or [-f | -i]
	compactOptsRe = regexp.MustCompile(`\[-([@A-Za-z0-9%,]+)\]`)
	// Matches pipe-separated short options: [-f | -i]
	pipeSepOptsRe = regexp.MustCompile(`\[((?:-[A-Za-z]\s*\|\s*)+(?:-[A-Za-z]))\]`)
	// Matches bracketed short option with argument: [-A num], [-f file]
	bracketShortArgRe = regexp.MustCompile(`\[-([A-Za-z])\s+(\w+)\]`)
	// Matches bracketed short option with optional argument: [-C[num]]
	bracketShortOptArgRe = regexp.MustCompile(`\[-([A-Za-z])\[(\w+)\]\]`)
	// Matches bracketed long option with =value: [--color=when]
	bracketLongArgRe = regexp.MustCompile(`\[--([\w-]+)=([\w]+)\]`)
	// Matches bracketed long option without argument: [--null]
	bracketLongRe = regexp.MustCompile(`\[--([\w-]+)\]`)
	// Matches multi-flag lines: "  -z, -j, -J, --lzma  Description"
	multiFlagRe = regexp.MustCompile(`^\s{2,}((?:-\w|--[\w-]+)(?:,\s*(?:-\w|--[\w-]+))+)\s{2,}(.+)`)
	// Matches short option with arg on its own: "  -b #  Description" or "  -f <filename>  Description"
	shortArgDescRe = regexp.MustCompile(`^\s{2,}(-[A-Za-z])\s+(<?\w+>?|#)\s{2,}(.+)`)
	// Matches colon-separated short options: "-b     : description" (python3 style)
	colonSepShortOptRe = regexp.MustCompile(`^(-[A-Za-z]{1,2})(?:\s+(\S+))?\s+:\s+(.+)`)
	// Matches pipe-separated short|long in brackets: [-c|--call <call>] (npx style)
	bracketPipeOptRe = regexp.MustCompile(`(-[A-Za-z])\|(--[\w-]+)`)
	// Matches standalone --long-flag pattern (not preceded by |)
	standaloneLongOptRe = regexp.MustCompile(`--([\w-]+)`)
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
					if sectionHeaderRe.MatchString(lines[j]) || uppercaseSectionRe.MatchString(lines[j]) {
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
		// Uppercase section headers without colon: "  COMMANDS  " (glab style)
		if m := uppercaseSectionRe.FindStringSubmatch(line); m != nil {
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
			// Strip binary name prefix: "  gemini mcp  Desc" → "  mcp  Desc"
			lineForParse := stripBinaryPrefix(line, name)
			if child, col := parseSubcommandLine(lineForParse); child != nil {
				node.Children = append(node.Children, child)
				last.set(&child.Description, col)
			} else if last.appendContinuation(line) {
				// wrapped description line consumed
			} else if m := bareSubcommandRe.FindStringSubmatch(lineForParse); m != nil {
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
			if inlineOpts := parseInlineMultiOptions(line); len(inlineOpts) > 0 {
				for _, o := range inlineOpts {
					node.Options = append(node.Options, o)
				}
				last.reset()
			} else if opt, col, ok := parseOptionLine(line); ok {
				node.Options = append(node.Options, opt)
				last.set(&node.Options[len(node.Options)-1].Description, col)
			} else if opts, col, ok := parseMultiFlagLine(line); ok {
				for _, o := range opts {
					node.Options = append(node.Options, o)
				}
				last.set(&node.Options[len(node.Options)-1].Description, col)
			} else if opt, col, ok := parseShortArgDescLine(line); ok {
				node.Options = append(node.Options, opt)
				last.set(&node.Options[len(node.Options)-1].Description, col)
			} else if opt, ok := parseBareOptionLine(line); ok {
				// Flag on its own line — description will come on next line
				node.Options = append(node.Options, opt)
				last.set(&node.Options[len(node.Options)-1].Description, 0)
			} else if opt, col, ok := parseColonSepOption(line); ok {
				node.Options = append(node.Options, opt)
				last.set(&node.Options[len(node.Options)-1].Description, col)
			} else if bracketOpts := parseBracketOptions(line); len(bracketOpts) > 0 {
				for _, o := range bracketOpts {
					node.Options = append(node.Options, o)
				}
				last.reset()
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

			// Try multi-option inline first (e.g. "  -c Create  -r Add/Replace")
			if inlineOpts := parseInlineMultiOptions(line); len(inlineOpts) > 0 {
				currentSection = "options"
				for _, o := range inlineOpts {
					node.Options = append(node.Options, o)
				}
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
			if opts, col, ok := parseMultiFlagLine(line); ok {
				currentSection = "options"
				for _, o := range opts {
					node.Options = append(node.Options, o)
				}
				last.set(&node.Options[len(node.Options)-1].Description, col)
				continue
			}
			if opt, col, ok := parseShortArgDescLine(line); ok {
				currentSection = "options"
				node.Options = append(node.Options, opt)
				last.set(&node.Options[len(node.Options)-1].Description, col)
				continue
			}
			if opt, col, ok := parseColonSepOption(line); ok {
				currentSection = "options"
				node.Options = append(node.Options, opt)
				last.set(&node.Options[len(node.Options)-1].Description, col)
				continue
			}

			// Try subcommand pattern (with description)
			if child, col := parseSubcommandLine(line); child != nil {
				consecutiveSubcmdHits++
				if consecutiveSubcmdHits >= inferThreshold {
					currentSection = "commands"
				}
				node.Children = append(node.Children, child)
				last.set(&child.Description, col)
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

	// Fallback: if few options were found, try extracting from usage lines.
	if len(node.Options) < 5 {
		usageOpts := extractUsageOptions(lines)
		node.Options = mergeOptions(node.Options, usageOpts)
	}

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
		if sectionHeaderRe.MatchString(line) || uppercaseSectionRe.MatchString(line) {
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

// parseSubcommandLine tries to parse a line as a subcommand.
// Tries multiple formats: "  cmd    desc", "  cmd - desc", "  cmd [meta]   desc".
// Returns the node and description start column, or nil if no match.
func parseSubcommandLine(line string) (*model.Node, int) {
	// Standard: "  command    Description text"
	if m := subcommandRe.FindStringSubmatch(line); m != nil {
		child := &model.Node{
			Name:        m[1],
			Description: strings.TrimSpace(m[2]),
		}
		return child, strings.Index(line, m[2])
	}
	// Dash-separated: "  command - Description text"
	if m := dashSepSubcmdRe.FindStringSubmatch(line); m != nil {
		child := &model.Node{
			Name:        m[1],
			Description: strings.TrimSpace(m[2]),
		}
		return child, strings.Index(line, m[2])
	}
	// With meta tokens: "  alias [command] [--flags]    Description" (glab style)
	if m := subcommandWithMetaRe.FindStringSubmatchIndex(line); m != nil {
		sm := subcommandWithMetaRe.FindStringSubmatch(line)
		child := &model.Node{
			Name:        sm[1],
			Description: strings.TrimSpace(sm[2]),
		}
		return child, m[4] // start index of capture group 2 (description)
	}
	return nil, 0
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

// extractUsageOptions parses BSD-style compact usage lines and returns options.
// Handles: [-abcdef], [-A num], [-C[num]], [--color=when], [--null]
func extractUsageOptions(lines []string) []model.Option {
	// Collect all usage-related lines (usage line + continuation lines)
	var usageParts []string
	inUsage := false
	for _, line := range lines {
		if usageRe.MatchString(line) {
			usageParts = append(usageParts, line)
			inUsage = true
			continue
		}
		if inUsage {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || sectionHeaderRe.MatchString(line) || uppercaseSectionRe.MatchString(line) {
				inUsage = false
				continue
			}
			// Continuation lines are indented (tab or spaces)
			if line[0] == '\t' || line[0] == ' ' {
				usageParts = append(usageParts, line)
				continue
			}
			inUsage = false
		}
	}
	if len(usageParts) == 0 {
		return nil
	}

	usageText := strings.Join(usageParts, " ")
	var opts []model.Option

	// Extract [-f | -i] pipe-separated options
	for _, m := range pipeSepOptsRe.FindAllStringSubmatch(usageText, -1) {
		parts := strings.Split(m[1], "|")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if len(p) == 2 && p[0] == '-' {
				opts = append(opts, model.Option{Short: p})
			}
		}
	}

	// Extract [-A num] style (short option with argument) — must come before compact
	for _, m := range bracketShortArgRe.FindAllStringSubmatch(usageText, -1) {
		opts = append(opts, model.Option{Short: "-" + m[1], Arg: m[2]})
	}

	// Extract [-C[num]] style (short option with optional argument)
	for _, m := range bracketShortOptArgRe.FindAllStringSubmatch(usageText, -1) {
		opts = append(opts, model.Option{Short: "-" + m[1], Arg: m[2]})
	}

	// Extract [--color=when] style (long option with argument)
	for _, m := range bracketLongArgRe.FindAllStringSubmatch(usageText, -1) {
		opts = append(opts, model.Option{Long: "--" + m[1], Arg: m[2]})
	}

	// Extract [--null] style (long option, no argument)
	// Avoid matching ones already captured by bracketLongArgRe
	for _, m := range bracketLongRe.FindAllStringSubmatch(usageText, -1) {
		longName := "--" + m[1]
		if !strings.Contains(usageText, "[--"+m[1]+"=") {
			opts = append(opts, model.Option{Long: longName})
		}
	}

	// Extract [-abcdefg] compact groups — expand each letter as a short option
	// Skip letters already captured as short options with arguments
	shortWithArg := make(map[byte]bool)
	for _, o := range opts {
		if o.Short != "" && len(o.Short) == 2 {
			shortWithArg[o.Short[1]] = true
		}
	}
	for _, m := range compactOptsRe.FindAllStringSubmatch(usageText, -1) {
		for i := 0; i < len(m[1]); i++ {
			ch := m[1][i]
			if shortWithArg[ch] {
				continue
			}
			opts = append(opts, model.Option{Short: "-" + string(ch)})
		}
	}

	return opts
}

// mergeOptions merges additional options into existing, skipping duplicates.
func mergeOptions(existing, additional []model.Option) []model.Option {
	seen := make(map[string]bool)
	for _, o := range existing {
		if o.Short != "" {
			seen[o.Short] = true
		}
		if o.Long != "" {
			seen[o.Long] = true
		}
	}
	merged := existing
	for _, o := range additional {
		if o.Short != "" && seen[o.Short] {
			continue
		}
		if o.Long != "" && seen[o.Long] {
			continue
		}
		merged = append(merged, o)
		if o.Short != "" {
			seen[o.Short] = true
		}
		if o.Long != "" {
			seen[o.Long] = true
		}
	}
	return merged
}

// parseMultiFlagLine parses lines like "  -z, -j, -J, --lzma  Description"
// where multiple flags share a single description.
func parseMultiFlagLine(line string) ([]model.Option, int, bool) {
	m := multiFlagRe.FindStringSubmatchIndex(line)
	if m == nil {
		return nil, 0, false
	}
	sm := multiFlagRe.FindStringSubmatch(line)
	flagsPart := sm[1]
	desc := strings.TrimSpace(sm[2])
	descCol := m[4] // start index of capture group 2

	flags := strings.Split(flagsPart, ",")
	var opts []model.Option
	for _, f := range flags {
		f = strings.TrimSpace(f)
		if strings.HasPrefix(f, "--") {
			opts = append(opts, model.Option{Long: f, Description: desc})
		} else if strings.HasPrefix(f, "-") && len(f) == 2 {
			opts = append(opts, model.Option{Short: f, Description: desc})
		}
	}
	return opts, descCol, len(opts) > 0
}

// parseInlineMultiOptions handles lines like "  -c Create  -r Add/Replace  -t List  -u Update  -x Extract"
// where multiple short options with brief descriptions appear on a single line.
func parseInlineMultiOptions(line string) []model.Option {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "-") {
		return nil
	}
	// Split on "  -" (2+ spaces before a flag) to find individual option segments
	segRe := regexp.MustCompile(`\s{2,}(-[A-Za-z]\s)`)
	indices := segRe.FindAllStringIndex(trimmed, -1)
	if len(indices) < 1 {
		return nil
	}

	// Build segments: first segment starts at 0, each subsequent at the match position
	var starts []int
	starts = append(starts, 0)
	for _, idx := range indices {
		// Find where the flag starts (skip leading spaces)
		flagStart := idx[0]
		for flagStart < idx[1] && (trimmed[flagStart] == ' ' || trimmed[flagStart] == '\t') {
			flagStart++
		}
		starts = append(starts, flagStart)
	}
	if len(starts) < 2 {
		return nil
	}

	flagRe := regexp.MustCompile(`^(-[A-Za-z])\s+(.+)$`)
	var opts []model.Option
	for i, s := range starts {
		var end int
		if i+1 < len(starts) {
			end = starts[i+1]
		} else {
			end = len(trimmed)
		}
		seg := strings.TrimSpace(trimmed[s:end])
		if m := flagRe.FindStringSubmatch(seg); m != nil {
			opts = append(opts, model.Option{Short: m[1], Description: strings.TrimSpace(m[2])})
		}
	}
	if len(opts) < 2 {
		return nil
	}
	return opts
}

// parseShortArgDescLine parses lines like "  -b #  Description" or "  -f <filename>  Description"
func parseShortArgDescLine(line string) (model.Option, int, bool) {
	m := shortArgDescRe.FindStringSubmatchIndex(line)
	if m == nil {
		return model.Option{}, 0, false
	}
	sm := shortArgDescRe.FindStringSubmatch(line)
	arg := strings.Trim(sm[2], "<>")
	return model.Option{
		Short:       sm[1],
		Arg:         arg,
		Description: strings.TrimSpace(sm[3]),
	}, m[6], true
}

// stripBinaryPrefix removes the root command name prefix from subcommand lines.
// e.g., "  gemini mcp   Desc" with rootName="gemini" becomes "  mcp   Desc".
func stripBinaryPrefix(line, rootName string) string {
	trimmed := strings.TrimLeft(line, " \t")
	prefix := rootName + " "
	if strings.HasPrefix(trimmed, prefix) {
		indent := len(line) - len(trimmed)
		return line[:indent] + trimmed[len(prefix):]
	}
	return line
}

// parseColonSepOption parses python3-style colon-separated options: "-b     : description"
func parseColonSepOption(line string) (model.Option, int, bool) {
	m := colonSepShortOptRe.FindStringSubmatchIndex(line)
	if m == nil {
		return model.Option{}, 0, false
	}
	sm := colonSepShortOptRe.FindStringSubmatch(line)
	return model.Option{
		Short:       sm[1],
		Arg:         strings.TrimSpace(sm[2]),
		Description: strings.TrimSpace(sm[3]),
	}, m[6], true
}

// parseBracketOptions parses npx-style bracket-enclosed option lines.
// e.g., "[--package <pkg>] [-c|--call <call>] [--workspaces]"
func parseBracketOptions(line string) []model.Option {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return nil
	}

	seen := make(map[string]bool)
	var opts []model.Option

	// Match -c|--call pipe patterns first
	for _, m := range bracketPipeOptRe.FindAllStringSubmatch(line, -1) {
		seen[m[1]] = true
		seen[m[2]] = true
		opts = append(opts, model.Option{Short: m[1], Long: m[2]})
	}

	// Match standalone --long-flag patterns
	for _, m := range standaloneLongOptRe.FindAllStringSubmatch(line, -1) {
		long := "--" + m[1]
		if !seen[long] {
			seen[long] = true
			opts = append(opts, model.Option{Long: long})
		}
	}

	if len(opts) == 0 {
		return nil
	}
	return opts
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
