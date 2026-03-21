package parser_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kwrkb/helptree/internal/parser"
)

// smokeCase defines a CLI command and expected parse results.
type smokeCase struct {
	cmd         string
	minChildren int // -1 = don't check
	minOptions  int // -1 = don't check
}

// knownCLIs lists commands to test with minimum expected parse results.
// Only commands found in PATH will be tested; others are skipped.
var knownCLIs = []smokeCase{
	// Package managers / dev tools
	{cmd: "go", minChildren: 10, minOptions: -1},
	{cmd: "git", minChildren: 15, minOptions: -1},
	{cmd: "npm", minChildren: 30, minOptions: -1},
	{cmd: "uv", minChildren: 10, minOptions: 3},
	{cmd: "fnm", minChildren: 5, minOptions: 3},
	{cmd: "pip", minChildren: 5, minOptions: -1},
	{cmd: "cargo", minChildren: 10, minOptions: -1},
	{cmd: "fvm", minChildren: 5, minOptions: 2},
	{cmd: "node", minChildren: -1, minOptions: 10},
	{cmd: "npx", minChildren: -1, minOptions: 2},
	{cmd: "brew", minChildren: 8, minOptions: -1},

	// System tools
	{cmd: "apt", minChildren: 10, minOptions: -1},
	{cmd: "apt-get", minChildren: 10, minOptions: -1},
	{cmd: "systemctl", minChildren: 5, minOptions: 10},

	// GNU coreutils / common
	{cmd: "ls", minChildren: -1, minOptions: 20},
	{cmd: "grep", minChildren: -1, minOptions: 10},
	{cmd: "tar", minChildren: -1, minOptions: 15},
	{cmd: "wget", minChildren: -1, minOptions: 20},
	{cmd: "rm", minChildren: -1, minOptions: 3},
	{cmd: "sudo", minChildren: -1, minOptions: 5},

	// CLI tools with subcommands
	{cmd: "gh", minChildren: 5, minOptions: -1},
	{cmd: "docker", minChildren: 5, minOptions: -1},
	{cmd: "kubectl", minChildren: 5, minOptions: -1},
	{cmd: "helm", minChildren: 5, minOptions: -1},
	{cmd: "terraform", minChildren: 5, minOptions: -1},
	{cmd: "glab", minChildren: 30, minOptions: -1},
	{cmd: "starship", minChildren: 5, minOptions: -1},
	{cmd: "gemini", minChildren: 3, minOptions: 3},
	{cmd: "codex", minChildren: 5, minOptions: -1},

	// Charm ecosystem
	{cmd: "vhs", minChildren: 3, minOptions: -1},

	// Options-heavy tools
	{cmd: "bat", minChildren: -1, minOptions: 15},
	{cmd: "eza", minChildren: -1, minOptions: 30},
	{cmd: "rg", minChildren: -1, minOptions: 10},
	{cmd: "fd", minChildren: -1, minOptions: 5},
	{cmd: "fzf", minChildren: -1, minOptions: 10},
	{cmd: "curl", minChildren: -1, minOptions: -1}, // curl --help is minimal, just check no crash

	// BSD / compact-style tools
	{cmd: "tmux", minChildren: -1, minOptions: 5},
	{cmd: "ssh", minChildren: -1, minOptions: 20},
	{cmd: "screencapture", minChildren: -1, minOptions: 10},
	{cmd: "python3", minChildren: -1, minOptions: 10},
}

func TestSmoke(t *testing.T) {
	tested := 0
	skipped := 0

	for _, tc := range knownCLIs {
		t.Run(tc.cmd, func(t *testing.T) {
			path, err := exec.LookPath(tc.cmd)
			if err != nil {
				skipped++
				t.Skipf("%s not found in PATH", tc.cmd)
			}

			// Run command --help with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, path, "--help").CombinedOutput()
			helpText := string(out)
			if helpText == "" && err != nil {
				t.Skipf("%s --help produced no output: %v", tc.cmd, err)
			}

			// Parse
			node := parser.Parse(tc.cmd, helpText)
			tested++

			// Basic sanity
			if node.Name != tc.cmd {
				t.Errorf("name: got %q, want %q", node.Name, tc.cmd)
			}

			// Check minimum children
			if tc.minChildren >= 0 && len(node.Children) < tc.minChildren {
				t.Errorf("children: got %d, want >= %d", len(node.Children), tc.minChildren)
				// Log first few children for debugging
				for i, c := range node.Children {
					if i >= 5 {
						break
					}
					t.Logf("  child[%d]: %q desc=%q", i, c.Name, truncDesc(c.Description))
				}
			}

			// Check minimum options
			if tc.minOptions >= 0 && len(node.Options) < tc.minOptions {
				t.Errorf("options: got %d, want >= %d", len(node.Options), tc.minOptions)
				for i, o := range node.Options {
					if i >= 5 {
						break
					}
					t.Logf("  option[%d]: %s desc=%q", i, o.FullFlag(), truncDesc(o.Description))
				}
			}

			t.Logf("✅ children=%d options=%d desc=%q",
				len(node.Children), len(node.Options), truncDesc(node.Description))
		})
	}

	t.Logf("Smoke test: %d tested, %d skipped (not in PATH)", tested, skipped)
}

func truncDesc(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}
