package parser

import (
	"regexp"
	"strings"

	"github.com/kwrkb/helptree/internal/model"
)

var (
	// Section detection
	sectionHeaderRe    = regexp.MustCompile(`^(?:The\s+)?([A-Za-z][\w\s()]*[\w)])\s*:`)
	uppercaseSectionRe = regexp.MustCompile(`^\s+([A-Z][A-Z ]{3,}[A-Z])\s*$`)
	bareSectionRe      = regexp.MustCompile(`(?i)^\s*(Synopsis)\s*$`)

	// Comma-separated command list: "    access, adduser, audit, bugs, ..."
	commaSepListRe = regexp.MustCompile(`^\s{2,}(\w[\w-]*(?:,\s*\w[\w-]*){2,}),?\s*$`)

	// Usage line
	usageRe = regexp.MustCompile(`(?i)^usage:\s*(.*)`)

	// BSD compact usage patterns
	compactOptsRe        = regexp.MustCompile(`\[-([@A-Za-z0-9%,]+)\]`)
	pipeSepOptsRe        = regexp.MustCompile(`\[((?:-[A-Za-z]\s*\|\s*)+(?:-[A-Za-z]))\]`)
	bracketShortArgRe    = regexp.MustCompile(`\[-([A-Za-z])\s+(\w+)\]`)
	bracketShortOptArgRe = regexp.MustCompile(`\[-([A-Za-z])\[(\w+)\]\]`)
	bracketLongArgRe      = regexp.MustCompile(`\[--([\w-]+)=([\w]+)\]`)
	bracketLongSpaceArgRe = regexp.MustCompile(`\[--([\w-]+)\s+(\w+)\]`)
	bracketLongRe         = regexp.MustCompile(`\[--([\w-]+)\]`)

	// npx-style bracket options
	bracketPipeOptRe    = regexp.MustCompile(`(-[A-Za-z])\|(--[\w-]+)`)
	standaloneLongOptRe = regexp.MustCompile(`--([\w-]+)`)

	// Inline multi-option parsing (tar style: "  -c Create  -r Add/Replace")
	inlineSegRe  = regexp.MustCompile(`\s{2,}(-[A-Za-z]\s)`)
	inlineFlagRe = regexp.MustCompile(`^(-[A-Za-z])\s+(.+)$`)
)

// Parse parses a --help output string and returns a Node.
func Parse(name, helpText string) *model.Node {
	node := &model.Node{
		Name:   name,
		Loaded: true,
	}

	helpText = strings.ReplaceAll(helpText, "\r\n", "\n")
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

	// Block-based structural parsing
	blocks := splitBlocks(lines)
	for i := range blocks {
		detectColumns(&blocks[i])
	}
	classifyBlocks(blocks)

	// Reclassify "other" blocks where most lines start with the root command name
	// (e.g., brew's help: "  brew search TEXT|/REGEX/")
	for i := range blocks {
		b := &blocks[i]
		if b.Kind == BlockHeader || b.Section != "other" {
			continue
		}
		prefixed := 0
		total := 0
		for _, line := range b.Lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			total++
			if strings.HasPrefix(trimmed, name+" ") {
				prefixed++
			}
		}
		// Require at least one line to have arguments after stripping prefix.
		// Pure "rootName word" lines (all bare) are likely usage examples,
		// not command lists (e.g., "helptree docker" in Examples section).
		hasArgs := false
		for _, line := range b.Lines {
			trimmed := strings.TrimSpace(line)
			stripped := trimCommandPrefix(trimmed, name)
			if stripped != trimmed && strings.Contains(stripped, " ") {
				hasArgs = true
				break
			}
		}
		if prefixed >= 2 && prefixed*2 > total && hasArgs {
			b.Section = "commands"
			if b.Kind != BlockTable {
				b.Kind = BlockSingle
			}
		}
	}

	for i := range blocks {
		b := &blocks[i]
		switch b.Section {
		case "commands":
			if b.Kind != BlockHeader {
				parseCommandBlock(node, b, name)
			}
		case "options":
			if b.Kind != BlockHeader {
				parseOptionBlock(node, b)
			}
		}
	}

	// Fallback: extract from iproute2-style OBJECT/OPTIONS inline definitions
	extractIproute2Definitions(lines, node)

	// Fallback: extract commands from category:comma format (e.g., snap)
	// Only when no structure was found at all (no children AND no options).
	if len(node.Children) == 0 && len(node.Options) == 0 {
		extractCategoryCommands(lines, node)
	}

	// Fallback: if few options were found, try extracting from usage lines.
	if len(node.Options) < 5 {
		usageOpts := extractUsageOptions(lines)
		node.Options = mergeOptions(node.Options, usageOpts)
	}

	// Fallback: extract options from brace-pipe patterns like { --help | --manual | --version }
	if len(node.Options) < 5 {
		braceOpts := extractBracePipeOptions(lines)
		node.Options = mergeOptions(node.Options, braceOpts)
	}

	// Fallback: extract bracket options from lines starting with command name
	// (e.g., "adduser [--uid id] [--firstuid id] ...")
	if len(node.Options) < 5 {
		cmdOpts := extractCommandNameBracketOptions(lines, name)
		node.Options = mergeOptions(node.Options, cmdOpts)
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
		if pastUsage && !strings.HasPrefix(trimmed, "-") && !looksLikeTableRow(line) {
			return trimmed
		}
		if !pastUsage && !strings.HasPrefix(trimmed, "-") {
			return trimmed
		}
	}
	return ""
}

// looksLikeTableRow checks if a line looks like an indented two-column row
// (e.g., "  command    Description text").
func looksLikeTableRow(line string) bool {
	indent := leadingSpaces(line)
	if indent < 1 {
		return false
	}
	descStart, _ := findGap(line)
	return descStart > 0
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

	// Extract [--uid id] style (long option with space-separated argument)
	for _, m := range bracketLongSpaceArgRe.FindAllStringSubmatch(usageText, -1) {
		opts = append(opts, model.Option{Long: "--" + m[1], Arg: m[2]})
	}

	// Extract [--null] style (long option, no argument)
	// Avoid matching ones already captured by bracketLongArgRe or bracketLongSpaceArgRe
	for _, m := range bracketLongRe.FindAllStringSubmatch(usageText, -1) {
		longName := "--" + m[1]
		if !strings.Contains(usageText, "[--"+m[1]+"=") && !strings.Contains(usageText, "[--"+m[1]+" ") {
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

// parseInlineMultiOptions handles lines like "  -c Create  -r Add/Replace  -t List  -u Update  -x Extract"
// where multiple short options with brief descriptions appear on a single line.
func parseInlineMultiOptions(line string) []model.Option {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "-") {
		return nil
	}
	// Split on "  -" (2+ spaces before a flag) to find individual option segments
	indices := inlineSegRe.FindAllStringIndex(trimmed, -1)
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

	var opts []model.Option
	for i, s := range starts {
		var end int
		if i+1 < len(starts) {
			end = starts[i+1]
		} else {
			end = len(trimmed)
		}
		seg := strings.TrimSpace(trimmed[s:end])
		if m := inlineFlagRe.FindStringSubmatch(seg); m != nil {
			opts = append(opts, model.Option{Short: m[1], Description: strings.TrimSpace(m[2])})
		}
	}
	if len(opts) < 2 {
		return nil
	}
	return opts
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

// trimCommandPrefix strips the root command name prefix and returns the trimmed result.
// e.g., "brew search TEXT" with rootName="brew" becomes "search TEXT".
// If no prefix matches, returns the original trimmed string.
func trimCommandPrefix(s, rootName string) string {
	prefix := rootName + " "
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):]
	}
	return s
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

// extractCategoryCommands extracts commands from category:comma format like
// "           Basics: find, info, install, remove, list, components"
func extractCategoryCommands(lines []string, node *model.Node) {
	catCmdRe := regexp.MustCompile(`^\s+[\w .]+:\s+([\w-]+(?:,\s+[\w-]+){2,})`)
	seen := make(map[string]bool)
	for _, line := range lines {
		if m := catCmdRe.FindStringSubmatch(line); m != nil {
			parts := strings.Split(m[1], ",")
			for _, p := range parts {
				name := strings.TrimSpace(p)
				if name != "" && !seen[name] {
					seen[name] = true
					node.Children = append(node.Children, &model.Node{Name: name})
				}
			}
		}
	}
}

// extractIproute2Definitions parses iproute2-style inline definitions:
//
//	OBJECT := { address | link | route | ... }
//	OPTIONS := { -V[ersion] | -s[tatistics] | ... }
func extractIproute2Definitions(lines []string, node *model.Node) {
	objectDefRe := regexp.MustCompile(`OBJECT\s*:=\s*\{`)
	optionsDefRe := regexp.MustCompile(`OPTIONS\s*:=\s*\{`)
	abbrOptRe := regexp.MustCompile(`^-([A-Za-z])\[(\w+)\]$`)

	// Collect tokens from a definition that may span multiple lines
	collectTokens := func(startIdx int) []string {
		var buf strings.Builder
		for i := startIdx; i < len(lines); i++ {
			buf.WriteString(lines[i])
			buf.WriteString(" ")
			if strings.Contains(lines[i], "}") {
				break
			}
		}
		s := buf.String()
		// Extract content between { and }
		braceStart := strings.Index(s, "{")
		braceEnd := strings.LastIndex(s, "}")
		if braceStart < 0 || braceEnd <= braceStart {
			return nil
		}
		inner := s[braceStart+1 : braceEnd]
		parts := strings.Split(inner, "|")
		var tokens []string
		for _, p := range parts {
			t := strings.TrimSpace(p)
			if t != "" {
				tokens = append(tokens, t)
			}
		}
		return tokens
	}

	for i, line := range lines {
		if objectDefRe.MatchString(line) && len(node.Children) == 0 {
			tokens := collectTokens(i)
			seen := make(map[string]bool)
			for _, t := range tokens {
				name := strings.Fields(t)[0]
				if seen[name] || name == "help" {
					continue
				}
				seen[name] = true
				node.Children = append(node.Children, &model.Node{Name: name})
			}
		}
		if optionsDefRe.MatchString(line) && len(node.Options) == 0 {
			tokens := collectTokens(i)
			for _, t := range tokens {
				t = strings.Fields(t)[0]
				if m := abbrOptRe.FindStringSubmatch(t); m != nil {
					short := "-" + m[1]
					long := "--" + strings.ToLower(string(m[1])) + m[2]
					node.Options = append(node.Options, model.Option{Short: short, Long: long})
				} else if strings.HasPrefix(t, "--") {
					node.Options = append(node.Options, model.Option{Long: t})
				} else if strings.HasPrefix(t, "-") && len(t) == 2 {
					node.Options = append(node.Options, model.Option{Short: t})
				}
			}
		}
	}
}

// extractBracePipeOptions extracts options from brace-pipe patterns like
// "xdg-open { --help | --manual | --version }".
func extractBracePipeOptions(lines []string) []model.Option {
	bracePipeRe := regexp.MustCompile(`\{\s*(--[\w-]+(?:\s*\|\s*--[\w-]+)*)\s*\}`)
	var opts []model.Option
	for _, line := range lines {
		for _, m := range bracePipeRe.FindAllStringSubmatch(line, -1) {
			parts := strings.Split(m[1], "|")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "--") {
					opts = append(opts, model.Option{Long: p})
				}
			}
		}
	}
	return opts
}

// extractCommandNameBracketOptions extracts [--option] patterns from lines
// that start with the command name (no "Usage:" prefix).
// e.g., "adduser [--uid id] [--firstuid id] ..."
func extractCommandNameBracketOptions(lines []string, name string) []model.Option {
	var usageParts []string
	inUsage := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inUsage && strings.HasPrefix(trimmed, name+" ") && strings.Contains(trimmed, "[--") {
			usageParts = append(usageParts, line)
			inUsage = true
			continue
		}
		if inUsage {
			if trimmed == "" || sectionHeaderRe.MatchString(line) || uppercaseSectionRe.MatchString(line) {
				inUsage = false
				continue
			}
			// Continuation lines: indented and contain brackets
			if (line[0] == ' ' || line[0] == '\t') && strings.Contains(trimmed, "[") {
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
	for _, m := range bracketLongArgRe.FindAllStringSubmatch(usageText, -1) {
		opts = append(opts, model.Option{Long: "--" + m[1], Arg: m[2]})
	}
	for _, m := range bracketLongSpaceArgRe.FindAllStringSubmatch(usageText, -1) {
		opts = append(opts, model.Option{Long: "--" + m[1], Arg: m[2]})
	}
	for _, m := range bracketLongRe.FindAllStringSubmatch(usageText, -1) {
		longName := "--" + m[1]
		if !strings.Contains(usageText, "[--"+m[1]+"=") && !strings.Contains(usageText, "[--"+m[1]+" ") {
			opts = append(opts, model.Option{Long: longName})
		}
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
