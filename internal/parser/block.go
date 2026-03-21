package parser

import "strings"

// BlockKind classifies a structural block in help output.
type BlockKind int

const (
	BlockUnknown BlockKind = iota
	BlockHeader            // section header line (e.g., "Commands:", "  FLAGS")
	BlockTable             // two-column table (key + description)
	BlockSingle            // single-column indented list (bare names, comma lists)
	BlockProse             // free-form text (description paragraphs)
)

// Separator type constants.
const (
	SepSpaces = "spaces"
	SepDash   = "dash"
	SepColon  = "colon"
)

// Block represents a contiguous group of lines sharing the same structure.
type Block struct {
	Kind      BlockKind
	Header    string   // header text (for BlockHeader blocks)
	Section   string   // categorized: "commands", "options", "other", ""
	Lines     []string // raw lines in this block
	KeyCol    int      // start column of the key column (-1 if not a table)
	DescCol   int      // start column of the description column (-1 if not a table)
	Separator string   // detected separator: SepSpaces, SepDash, SepColon
}

func newBlock(kind BlockKind, header string, lines []string) Block {
	return Block{Kind: kind, Header: header, Lines: lines, KeyCol: -1, DescCol: -1}
}

// splitBlocks segments help text lines into structural blocks.
// Blank lines and section headers act as block boundaries.
func splitBlocks(lines []string) []Block {
	var blocks []Block
	var current []string

	flush := func() {
		if len(current) > 0 {
			blocks = append(blocks, newBlock(BlockUnknown, "", current))
			current = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			flush()
			continue
		}

		if m := sectionHeaderRe.FindStringSubmatch(line); m != nil {
			flush()
			blocks = append(blocks, newBlock(BlockHeader, m[1], []string{line}))
			continue
		}
		if m := uppercaseSectionRe.FindStringSubmatch(line); m != nil {
			flush()
			blocks = append(blocks, newBlock(BlockHeader, m[1], []string{line}))
			continue
		}
		if strings.HasPrefix(trimmed, "The commands are") ||
			strings.HasPrefix(trimmed, "The topics are") {
			flush()
			blocks = append(blocks, newBlock(BlockHeader, "commands", []string{line}))
			continue
		}

		current = append(current, line)
	}
	flush()
	return blocks
}

// detectColumns analyzes a block's lines for two-column table structure.
// It sets Kind, DescCol, KeyCol, and Separator on the block.
func detectColumns(b *Block) {
	if b.Kind == BlockHeader {
		return
	}
	if len(b.Lines) == 0 {
		b.Kind = BlockProse
		return
	}

	// Collect description start positions from lines that have a 2+ space gap
	var infos []gapInfo

	for _, line := range b.Lines {
		descStart, sep := findGap(line)
		if descStart > 0 {
			infos = append(infos, gapInfo{descStart, sep})
		}
	}

	if len(infos) == 0 {
		// No two-column structure detected
		b.Kind = classifyNonTable(b)
		return
	}

	// Find the modal (most frequent) description start column with tolerance ±2
	modalCol, freq := modalValue(infos, 2)

	// If >=50% of lines with gaps align at the same column, it's a table
	if freq*2 >= len(infos) {
		b.Kind = BlockTable
		b.DescCol = modalCol
		b.KeyCol = leadingSpaces(b.Lines[0])

		// Determine dominant separator
		sepCounts := map[string]int{}
		for _, info := range infos {
			if abs(info.descStart-modalCol) <= 2 {
				sepCounts[info.separator]++
			}
		}
		b.Separator = SepSpaces
		for sep, count := range sepCounts {
			if count > sepCounts[b.Separator] {
				b.Separator = sep
			}
		}
	} else {
		b.Kind = classifyNonTable(b)
	}
}

// findGap locates the first significant gap in a line after non-space content.
// A gap is either 2+ spaces, or a " - " / " : " separator pattern.
// Returns the description start column and the separator type.
// Returns (0, "") if no gap is found.
func findGap(line string) (int, string) {
	if len(line) == 0 {
		return 0, ""
	}

	// Skip leading whitespace to find where content starts
	contentStart := 0
	for contentStart < len(line) && (line[contentStart] == ' ' || line[contentStart] == '\t') {
		contentStart++
	}
	if contentStart >= len(line) {
		return 0, ""
	}

	// Check for " - " / " – " / " — " dash separator pattern first.
	// These may have only 1 space before the dash but are still column separators.
	for _, dashPat := range []string{" - ", " – ", " — "} {
		if idx := strings.Index(line[contentStart:], dashPat); idx >= 0 {
			descStart := contentStart + idx + len(dashPat)
			if descStart < len(line) {
				return descStart, SepDash
			}
		}
	}

	// Check for " : " colon separator pattern (python3 style)
	if idx := strings.Index(line[contentStart:], " : "); idx >= 0 {
		descStart := contentStart + idx + 3
		if descStart < len(line) {
			return descStart, SepColon
		}
	}

	// Walk through content looking for a 2+ space gap
	i := contentStart
	for i < len(line) {
		if line[i] == ' ' || line[i] == '\t' {
			gapStart := i
			for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
				i++
			}
			gapLen := i - gapStart
			if gapLen >= 2 && i < len(line) {
				return i, SepSpaces
			}
		} else {
			i++
		}
	}
	return 0, ""
}

// gapInfo records the description start column and separator type for a line.
type gapInfo struct {
	descStart int
	separator string
}

// modalValue finds the most frequent value in gapInfos within a given tolerance.
// Returns the modal value and its frequency (count of values within tolerance).
func modalValue(infos []gapInfo, tolerance int) (int, int) {
	if len(infos) == 0 {
		return 0, 0
	}

	// Count frequencies using buckets with tolerance
	type bucket struct {
		center int
		count  int
	}
	var buckets []bucket

	for _, info := range infos {
		found := false
		for j := range buckets {
			if abs(info.descStart-buckets[j].center) <= tolerance {
				buckets[j].count++
				found = true
				break
			}
		}
		if !found {
			buckets = append(buckets, bucket{info.descStart, 1})
		}
	}

	best := buckets[0]
	for _, b := range buckets[1:] {
		if b.count > best.count {
			best = b
		}
	}
	return best.center, best.count
}

// classifyNonTable determines the kind for a block that is not a table.
func classifyNonTable(b *Block) BlockKind {
	if len(b.Lines) == 0 {
		return BlockProse
	}

	// Check if most lines look like bare names (single indented word)
	bareCount := 0
	for _, line := range b.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.Contains(trimmed, " ") {
			bareCount++
		}
		if commaSepListRe.MatchString(line) {
			return BlockSingle
		}
	}
	if bareCount*2 >= len(b.Lines) {
		return BlockSingle
	}

	return BlockProse
}

// classifyBlocks assigns Section to each block based on headers and content.
func classifyBlocks(blocks []Block) {
	currentHeader := ""

	for i := range blocks {
		b := &blocks[i]

		if b.Kind == BlockHeader {
			currentHeader = b.Header
			b.Section = categorizeSection(currentHeader)
			continue
		}

		// If preceded by a commands/options header, use it
		if currentHeader != "" {
			section := categorizeSection(currentHeader)
			if section != "other" {
				b.Section = section
				continue
			}
			// "other" headers (e.g., "Usage:") don't propagate — infer from content
		}

		// Infer from content
		if b.Kind == BlockTable || b.Kind == BlockSingle || b.Kind == BlockProse || b.Kind == BlockUnknown {
			section := inferSectionFromContent(b)
			if section != "other" {
				b.Section = section
			} else {
				b.Section = "other"
			}
		} else {
			b.Section = "other"
		}
	}
}

// inferSectionFromContent examines the key column to determine if a block
// contains commands or options.
func inferSectionFromContent(b *Block) string {
	optionLike := 0
	commandLike := 0
	limit := len(b.Lines)
	if limit > 5 {
		limit = 5
	}

	for _, line := range b.Lines[:limit] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			optionLike++
		} else if !strings.Contains(trimmed, " ") || (b.Kind == BlockTable && b.DescCol > 0) {
			// Single word or table with key column that doesn't start with "-"
			commandLike++
		}
	}

	if optionLike > commandLike {
		return "options"
	}
	if commandLike >= 2 {
		return "commands"
	}
	return "other"
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

