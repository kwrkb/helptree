package runner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kwrkb/helptree/internal/model"
	"github.com/kwrkb/helptree/internal/parser"
)

const defaultTimeout = 10 * time.Second

// RunHelp executes "command --help" and returns the parsed node.
func RunHelp(cmdName string, args ...string) (*model.Node, error) {
	return RunHelpWithContext(context.Background(), cmdName, args...)
}

// RunHelpWithContext executes "command [args...] --help" with a context.
func RunHelpWithContext(ctx context.Context, cmdName string, args ...string) (*model.Node, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	fullArgs := make([]string, len(args)+1)
	copy(fullArgs, args)
	fullArgs[len(args)] = "--help"
	cmd := exec.CommandContext(ctx, cmdName, fullArgs...)

	// Many CLIs print help to stderr, so capture both
	out, err := cmd.CombinedOutput()
	helpText := string(out)

	// --help often exits with non-zero, so only fail if no output
	if helpText == "" && err != nil {
		return nil, fmt.Errorf("failed to run %s --help: %w", cmdName, err)
	}

	// Build the display name
	name := cmdName
	if len(args) > 0 {
		name = cmdName + " " + strings.Join(args, " ")
	}

	node := parser.Parse(name, helpText)
	return node, nil
}

// LoadNode runs --help on the given node itself and returns a new Node
// with populated Children, Options, Usage, and Description.
// This avoids mutating the input node directly (preventing data races with the TUI).
func LoadNode(ctx context.Context, rootCmd string, parentName string) (*model.Node, error) {
	// Build subcommand path from parent name: "docker container" → ["container"]
	parts := strings.Fields(parentName)
	var subPath []string
	if len(parts) > 1 {
		subPath = parts[1:]
	}

	return RunHelpWithContext(ctx, rootCmd, subPath...)
}
