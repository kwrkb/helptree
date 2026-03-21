package parser

import (
	"strings"
	"testing"
)

func TestSplitBlocksDocker(t *testing.T) {
	lines := strings.Split(dockerHelp, "\n")
	blocks := splitBlocks(lines)

	// Expect: "Usage:..." prose, "A self-sufficient..." prose,
	//         "Common Commands:" header, commands block,
	//         "Management Commands:" header, commands block,
	//         "Global Options:" header, options block
	headerCount := 0
	contentCount := 0
	for _, b := range blocks {
		if b.Kind == BlockHeader {
			headerCount++
		} else {
			contentCount++
		}
	}
	if headerCount < 3 {
		t.Errorf("expected at least 3 headers, got %d", headerCount)
	}
	if contentCount < 3 {
		t.Errorf("expected at least 3 content blocks, got %d", contentCount)
	}

	// No lines should be lost
	totalLines := 0
	for _, b := range blocks {
		totalLines += len(b.Lines)
	}
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}
	if totalLines != nonEmptyLines {
		t.Errorf("line count mismatch: blocks have %d lines, original has %d non-empty lines",
			totalLines, nonEmptyLines)
	}
}

func TestSplitBlocksGh(t *testing.T) {
	lines := strings.Split(ghHelp, "\n")
	blocks := splitBlocks(lines)

	// Should find "Available Commands:" and "Flags:" headers
	headers := []string{}
	for _, b := range blocks {
		if b.Kind == BlockHeader {
			headers = append(headers, b.Header)
		}
	}
	foundCommands := false
	foundFlags := false
	for _, h := range headers {
		if strings.Contains(strings.ToLower(h), "command") {
			foundCommands = true
		}
		if strings.Contains(strings.ToLower(h), "flag") {
			foundFlags = true
		}
	}
	if !foundCommands {
		t.Errorf("expected to find commands header, got headers: %v", headers)
	}
	if !foundFlags {
		t.Errorf("expected to find flags header, got headers: %v", headers)
	}
}

func TestDetectColumnsCommandsBlock(t *testing.T) {
	// Standard 2-column command table
	lines := []string{
		"  run         Create and run a new container",
		"  exec        Execute a command in a running container",
		"  ps          List containers",
		"  build       Build an image from a Dockerfile",
	}
	b := Block{Lines: lines, KeyCol: -1, DescCol: -1}
	detectColumns(&b)

	if b.Kind != BlockTable {
		t.Errorf("expected BlockTable, got %d", b.Kind)
	}
	if b.DescCol < 14 || b.DescCol > 16 {
		t.Errorf("expected DescCol around 14-16, got %d", b.DescCol)
	}
	if b.Separator != "spaces" {
		t.Errorf("expected separator 'spaces', got %q", b.Separator)
	}
}

func TestDetectColumnsOptionsBlock(t *testing.T) {
	lines := []string{
		"      --config string      Location of client config files",
		"  -c, --context string     Name of the context to use",
		"  -D, --debug              Enable debug mode",
		"  -l, --log-level string   Set the logging level",
	}
	b := Block{Lines: lines, KeyCol: -1, DescCol: -1}
	detectColumns(&b)

	if b.Kind != BlockTable {
		t.Errorf("expected BlockTable, got %d", b.Kind)
	}
	// Description should start around column 27
	if b.DescCol < 25 || b.DescCol > 30 {
		t.Errorf("expected DescCol around 25-30, got %d", b.DescCol)
	}
}

func TestDetectColumnsDashSeparator(t *testing.T) {
	lines := []string{
		"  access - provides access to private functions",
		"  audit - run a security audit",
		"  bugs - report bugs for a package",
	}
	b := Block{Lines: lines, KeyCol: -1, DescCol: -1}
	detectColumns(&b)

	if b.Kind != BlockTable {
		t.Errorf("expected BlockTable, got %d", b.Kind)
	}
	if b.Separator != "dash" {
		t.Errorf("expected separator 'dash', got %q", b.Separator)
	}
}

func TestDetectColumnsColonSeparator(t *testing.T) {
	lines := []string{
		"-b     : issue warnings about str(bytes_instance)",
		"-B     : don't write .pyc files on import",
		"-d     : turn on parser debugging output",
		"-E     : ignore PYTHON* environment variables",
	}
	b := Block{Lines: lines, KeyCol: -1, DescCol: -1}
	detectColumns(&b)

	if b.Kind != BlockTable {
		t.Errorf("expected BlockTable, got %d", b.Kind)
	}
	if b.Separator != "colon" {
		t.Errorf("expected separator 'colon', got %q", b.Separator)
	}
}

func TestDetectColumnsBareSubcommands(t *testing.T) {
	lines := []string{
		"  init",
		"  build",
		"  test",
		"  clean",
	}
	b := Block{Lines: lines, KeyCol: -1, DescCol: -1}
	detectColumns(&b)

	if b.Kind != BlockSingle {
		t.Errorf("expected BlockSingle, got %d", b.Kind)
	}
}

func TestDetectColumnsGlabWideGap(t *testing.T) {
	lines := []string{
		"    alias [command] [--flags]                        Create, list, and delete aliases.",
		"    api <endpoint> [--flags]                         Make an authenticated request.",
		"    auth <command> [command]                         Manage authentication state.",
		"    check-update                                     Check for latest releases.",
	}
	b := Block{Lines: lines, KeyCol: -1, DescCol: -1}
	detectColumns(&b)

	if b.Kind != BlockTable {
		t.Errorf("expected BlockTable, got %d", b.Kind)
	}
	// glab has description starting very far right
	if b.DescCol < 40 {
		t.Errorf("expected DescCol >= 40 for glab, got %d", b.DescCol)
	}
}

func TestDetectColumnsProse(t *testing.T) {
	lines := []string{
		"A self-sufficient runtime for containers",
	}
	b := Block{Lines: lines, KeyCol: -1, DescCol: -1}
	detectColumns(&b)

	if b.Kind == BlockTable {
		t.Errorf("prose should not be detected as BlockTable")
	}
}

func TestClassifyBlocksDocker(t *testing.T) {
	lines := strings.Split(dockerHelp, "\n")
	blocks := splitBlocks(lines)
	for i := range blocks {
		detectColumns(&blocks[i])
	}
	classifyBlocks(blocks)

	commandBlocks := 0
	optionBlocks := 0
	for _, b := range blocks {
		if b.Kind == BlockHeader {
			continue
		}
		switch b.Section {
		case "commands":
			commandBlocks++
		case "options":
			optionBlocks++
		}
	}
	if commandBlocks < 2 {
		t.Errorf("expected at least 2 command blocks (Common + Management), got %d", commandBlocks)
	}
	if optionBlocks < 1 {
		t.Errorf("expected at least 1 option block, got %d", optionBlocks)
	}
}

func TestClassifyBlocksNoHeader(t *testing.T) {
	// Headerless subcommand block should be inferred as "commands"
	help := `Usage: mytool [command]

  init        Initialize a new project
  build       Build the project
  test        Run tests
  clean       Remove build artifacts`

	lines := strings.Split(help, "\n")
	blocks := splitBlocks(lines)
	for i := range blocks {
		detectColumns(&blocks[i])
	}
	classifyBlocks(blocks)

	foundCommands := false
	for _, b := range blocks {
		if b.Section == "commands" {
			foundCommands = true
		}
	}
	if !foundCommands {
		t.Error("expected headerless command table to be classified as 'commands'")
		for _, b := range blocks {
			t.Logf("  block kind=%d section=%q lines=%d", b.Kind, b.Section, len(b.Lines))
		}
	}
}

func TestClassifyBlocksNoHeaderOptions(t *testing.T) {
	// Headerless option block should be inferred as "options"
	help := `Usage: oldtool [options] <file>

  -v, --verbose         Enable verbose output
  -o, --output string   Output file path`

	lines := strings.Split(help, "\n")
	blocks := splitBlocks(lines)
	for i := range blocks {
		detectColumns(&blocks[i])
	}
	classifyBlocks(blocks)

	foundOptions := false
	for _, b := range blocks {
		if b.Section == "options" {
			foundOptions = true
		}
	}
	if !foundOptions {
		t.Error("expected headerless option table to be classified as 'options'")
		for _, b := range blocks {
			t.Logf("  block kind=%d section=%q lines=%d descCol=%d", b.Kind, b.Section, len(b.Lines), b.DescCol)
			for _, l := range b.Lines {
				t.Logf("    %q", l)
			}
		}
	}
}

func TestDetectColumnsWrappedDescription(t *testing.T) {
	// Wrapped descriptions should still detect as a table
	lines := []string{
		"  deploy      Deploy the application to the specified",
		"              environment with all configurations",
		"  status      Show current status",
	}
	b := Block{Lines: lines, KeyCol: -1, DescCol: -1}
	detectColumns(&b)

	if b.Kind != BlockTable {
		t.Errorf("expected BlockTable for wrapped descriptions, got %d", b.Kind)
	}
}
