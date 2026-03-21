//go:build discover

package parser_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kwrkb/helptree/internal/parser"
)

// TestDiscover scans all binaries in common PATH directories,
// runs --help on each, parses the output, and reports how well
// the parser captures the structure.
//
// Results are written to:
//   - /tmp/claude-1000/helptree-discover.tsv  (summary per command)
//   - /tmp/claude-1000/helptree-help-outputs/ (raw --help output per command)
func TestDiscover(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping discovery test in short mode")
	}

	dirs := []string{
		"/usr/bin",
		"/usr/local/bin",
		"/usr/sbin",
		os.ExpandEnv("$HOME/go/bin"),
		os.ExpandEnv("$HOME/.local/bin"),
		os.ExpandEnv("$HOME/.cargo/bin"),
	}

	// Collect unique command names
	seen := make(map[string]bool)
	var cmds []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			name := e.Name()
			if seen[name] {
				continue
			}
			seen[name] = true
			cmds = append(cmds, filepath.Join(dir, name))
		}
	}
	sort.Strings(cmds)

	// Prepare output directory for raw help texts
	helpDir := "/tmp/claude-1000/helptree-help-outputs"
	os.MkdirAll(helpDir, 0o755)

	// Open TSV output
	tsvPath := "/tmp/claude-1000/helptree-discover.tsv"
	tsvFile, err := os.Create(tsvPath)
	if err != nil {
		t.Fatalf("cannot create %s: %v", tsvPath, err)
	}
	defer tsvFile.Close()
	fmt.Fprintf(tsvFile, "status\tcmd\tchildren\toptions\tdescription\n")

	var okCount, emptyCount, skipCount int

	for _, cmdPath := range cmds {
		name := filepath.Base(cmdPath)

		// Run --help with timeout; kill process group on timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, cmdPath, "--help")
		cmd.Stdin = nil
		cmd.Env = append(os.Environ(), "PAGER=cat", "MANPAGER=cat", "TERM=dumb", "GIT_PAGER=cat")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		out, err := cmd.CombinedOutput()
		cancel()

		helpText := string(out)

		// Skip if no output
		if helpText == "" || (err != nil && helpText == "") {
			skipCount++
			fmt.Fprintf(tsvFile, "SKIP\t%s\t0\t0\t\n", name)
			continue
		}

		// Save raw help output
		os.WriteFile(filepath.Join(helpDir, name+".txt"), out, 0o644)

		// Parse
		node := parser.Parse(name, helpText)
		desc := truncDesc(node.Description)

		if len(node.Children) > 0 || len(node.Options) > 0 {
			okCount++
			fmt.Fprintf(tsvFile, "OK\t%s\t%d\t%d\t%s\n",
				name, len(node.Children), len(node.Options), desc)
		} else {
			emptyCount++
			fmt.Fprintf(tsvFile, "EMPTY\t%s\t0\t0\t%s\n", name, desc)
		}
	}

	t.Logf("Discovery complete: %d OK, %d EMPTY, %d SKIP (total %d)",
		okCount, emptyCount, skipCount, len(cmds))
	t.Logf("OK rate: %.1f%% of tested (%d/%d)",
		float64(okCount)/float64(okCount+emptyCount)*100, okCount, okCount+emptyCount)
	t.Logf("Results: %s", tsvPath)
	t.Logf("Help outputs: %s", helpDir)
}
