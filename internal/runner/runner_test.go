package runner

import (
	"context"
	"testing"
)

func TestRunHelpGo(t *testing.T) {
	// "go --help" should be available in any Go environment
	node, err := RunHelp("go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Name != "go" {
		t.Errorf("expected name 'go', got %q", node.Name)
	}
	if len(node.Children) == 0 {
		t.Error("expected at least some subcommands for 'go'")
	}
}

func TestRunHelpNonExistent(t *testing.T) {
	_, err := RunHelp("nonexistent-command-that-does-not-exist-12345")
	if err == nil {
		t.Error("expected error for non-existent command")
	}
}

func TestRunHelpWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := RunHelpWithContext(ctx, "go")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}
