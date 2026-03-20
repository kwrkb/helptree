package parser

import (
	"strings"
	"testing"
)

const dockerHelp = `Usage:  docker [OPTIONS] COMMAND

A self-sufficient runtime for containers

Common Commands:
  run         Create and run a new container
  exec        Execute a command in a running container
  ps          List containers
  build       Build an image from a Dockerfile

Management Commands:
  container   Manage containers
  image       Manage images
  volume      Manage volumes

Global Options:
      --config string      Location of client config files (default "/root/.docker")
  -c, --context string     Name of the context to use
  -D, --debug              Enable debug mode
  -l, --log-level string   Set the logging level ("debug"|"info"|"warn"|"error"|"fatal") (default "info")
  -v, --version            Print version information and quit`

const ghHelp = `Work seamlessly with GitHub from the command line.

Usage:
  gh <command> <subcommand> [flags]

Available Commands:
  auth        Authenticate gh and git with GitHub
  browse      Open the repository in the browser
  codespace   Connect to and manage codespaces
  gist        Manage gists
  issue       Manage issues
  pr          Manage pull requests
  release     Manage releases
  repo        Manage repositories

Flags:
      --help      Show help for command
      --version   Show gh version

Learn more: https://cli.github.com/manual`

const kubectlHelp = `kubectl controls the Kubernetes cluster manager.

 Find more information at: https://kubernetes.io/docs/reference/kubectl/

Basic Commands (Beginner):
  create          Create a resource from a file or from stdin
  expose          Take a replication controller, service, deployment or pod and expose it as a new Kubernetes service
  run             Run a particular image on the cluster
  set             Set specific features on objects

Basic Commands (Intermediate):
  explain         Get documentation for a resource
  get             List one or many resources
  edit            Edit a resource on the server
  delete          Delete resources by file names, stdin, resources and names, or by resources and label selector

Usage:
  kubectl [flags] [options]`

func TestParseDocker(t *testing.T) {
	node := Parse("docker", dockerHelp)

	if node.Name != "docker" {
		t.Errorf("expected name 'docker', got %q", node.Name)
	}
	if node.Description != "A self-sufficient runtime for containers" {
		t.Errorf("unexpected description: %q", node.Description)
	}

	// Should have children from both Common Commands and Management Commands
	if len(node.Children) < 7 {
		t.Errorf("expected at least 7 children, got %d", len(node.Children))
	}

	// Check first child
	if node.Children[0].Name != "run" {
		t.Errorf("expected first child 'run', got %q", node.Children[0].Name)
	}
	if node.Children[0].Description != "Create and run a new container" {
		t.Errorf("unexpected child description: %q", node.Children[0].Description)
	}

	// Check options
	if len(node.Options) < 4 {
		t.Errorf("expected at least 4 options, got %d", len(node.Options))
	}

	// Find --config option
	found := false
	for _, opt := range node.Options {
		if opt.Long == "--config" {
			found = true
			if opt.Short != "" {
				t.Errorf("--config should have no short flag, got %q", opt.Short)
			}
			if opt.Arg != "string" {
				t.Errorf("--config arg should be 'string', got %q", opt.Arg)
			}
		}
	}
	if !found {
		t.Error("--config option not found")
	}

	// Find -D, --debug option
	for _, opt := range node.Options {
		if opt.Long == "--debug" {
			if opt.Short != "-D" {
				t.Errorf("--debug short should be '-D', got %q", opt.Short)
			}
			if opt.Arg != "" {
				t.Errorf("--debug should have no arg, got %q", opt.Arg)
			}
		}
	}
}

func TestParseGh(t *testing.T) {
	node := Parse("gh", ghHelp)

	if node.Name != "gh" {
		t.Errorf("expected name 'gh', got %q", node.Name)
	}
	if node.Description != "Work seamlessly with GitHub from the command line." {
		t.Errorf("unexpected description: %q", node.Description)
	}
	if len(node.Children) != 8 {
		t.Errorf("expected 8 children, got %d", len(node.Children))
	}
	if len(node.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(node.Options))
	}
	// gh has multi-line usage: "Usage:\n  gh <command> ..."
	if node.Usage == "" {
		t.Error("expected non-empty usage for gh")
	}
}

func TestParseKubectl(t *testing.T) {
	node := Parse("kubectl", kubectlHelp)

	if node.Name != "kubectl" {
		t.Errorf("expected name 'kubectl', got %q", node.Name)
	}

	// Should find children from both Beginner and Intermediate sections
	if len(node.Children) < 8 {
		t.Errorf("expected at least 8 children, got %d", len(node.Children))
	}

	// Check that 'create' is found
	found := false
	for _, child := range node.Children {
		if child.Name == "create" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'create' subcommand not found")
	}
}

func TestParseUsageMultiline(t *testing.T) {
	help := `Some tool description.

Usage:
  mytool [command] [flags]
  mytool [command] --help

Available Commands:
  init        Initialize a new project
`
	node := Parse("mytool", help)
	if node.Usage == "" {
		t.Error("expected non-empty usage")
	}
	if !strings.Contains(node.Usage, "mytool [command] [flags]") {
		t.Errorf("usage should contain first synopsis line, got %q", node.Usage)
	}
}

func TestParseUsageInline(t *testing.T) {
	help := `Usage: simple-tool [options] <file>

A simple tool.
`
	node := Parse("simple-tool", help)
	if node.Usage != "simple-tool [options] <file>" {
		t.Errorf("unexpected usage: %q", node.Usage)
	}
}

func TestParseWrappedDescription(t *testing.T) {
	help := `Usage: longtool [command]

A tool with long descriptions.

Commands:
  deploy      Deploy the application to the specified
              environment with all configurations
  status      Show current status

Options:
  -o, --output string   Set the output format for the
                        command results
      --verbose         Enable verbose logging
`
	node := Parse("longtool", help)

	// Check wrapped subcommand description
	if len(node.Children) < 2 {
		t.Fatalf("expected at least 2 children, got %d", len(node.Children))
	}
	deploy := node.Children[0]
	if !strings.Contains(deploy.Description, "environment with all configurations") {
		t.Errorf("deploy description should include wrapped text, got %q", deploy.Description)
	}
	if !strings.Contains(deploy.Description, "Deploy the application") {
		t.Errorf("deploy description should include first line, got %q", deploy.Description)
	}

	// Check wrapped option description
	if len(node.Options) < 2 {
		t.Fatalf("expected at least 2 options, got %d", len(node.Options))
	}
	outOpt := node.Options[0]
	if !strings.Contains(outOpt.Description, "command results") {
		t.Errorf("--output description should include wrapped text, got %q", outOpt.Description)
	}

	// Check non-wrapped option is not polluted
	verbose := node.Options[1]
	if verbose.Long != "--verbose" {
		t.Errorf("expected --verbose, got %q", verbose.Long)
	}
}

func TestParseKubectlWrappedCommands(t *testing.T) {
	// kubectl has long descriptions that wrap, e.g.:
	// "  expose          Take a replication controller, service, deployment or pod and
	//                    expose it as a new Kubernetes service"
	help := `kubectl controls the Kubernetes cluster manager.

Basic Commands:
  expose          Take a replication controller, service,
                  deployment or pod and expose it as a new
                  Kubernetes service
  run             Run a particular image on the cluster
`
	node := Parse("kubectl", help)

	if len(node.Children) < 2 {
		t.Fatalf("expected at least 2 children, got %d", len(node.Children))
	}
	expose := node.Children[0]
	if !strings.Contains(expose.Description, "Kubernetes service") {
		t.Errorf("expose description should include wrapped lines, got %q", expose.Description)
	}
	// Ensure 'run' is still parsed as a separate command
	run := node.Children[1]
	if run.Name != "run" {
		t.Errorf("expected 'run' as second child, got %q", run.Name)
	}
}

func TestParseNoSectionHeader(t *testing.T) {
	// Some simple tools have no section headers at all
	help := `mytool - a simple utility

Usage: mytool [command]

  init        Initialize a new project
  build       Build the project
  test        Run tests
  clean       Remove build artifacts
`
	node := Parse("mytool", help)

	if len(node.Children) != 4 {
		t.Errorf("expected 4 children, got %d", len(node.Children))
		for i, c := range node.Children {
			t.Logf("  child[%d]: %q", i, c.Name)
		}
	}
	if len(node.Children) > 0 && node.Children[0].Name != "init" {
		t.Errorf("expected first child 'init', got %q", node.Children[0].Name)
	}
}

func TestParseNoSectionHeaderWithOptions(t *testing.T) {
	// Tools that list options without a "Flags:" or "Options:" header
	help := `Usage: oldtool [options] <file>

  -v, --verbose         Enable verbose output
  -o, --output string   Output file path
`
	node := Parse("oldtool", help)

	if len(node.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(node.Options))
	}
	if len(node.Options) > 0 && node.Options[0].Long != "--verbose" {
		t.Errorf("expected --verbose, got %q", node.Options[0].Long)
	}
}

func TestParseBareSubcommands(t *testing.T) {
	// Commands listed with no description
	help := `Usage: minimal [command]

Commands:
  init
  build
  test
  clean
`
	node := Parse("minimal", help)

	if len(node.Children) != 4 {
		t.Errorf("expected 4 children, got %d", len(node.Children))
	}
	for _, child := range node.Children {
		if child.Description != "" {
			t.Errorf("expected empty description for %q, got %q", child.Name, child.Description)
		}
	}
	if len(node.Children) > 0 && node.Children[0].Name != "init" {
		t.Errorf("expected first child 'init', got %q", node.Children[0].Name)
	}
}

func TestParseMixedBareAndDescribed(t *testing.T) {
	// Some commands have descriptions, others don't
	help := `Commands:
  init        Initialize a project
  build
  test        Run all tests
  clean
`
	node := Parse("mixed", help)

	if len(node.Children) != 4 {
		t.Fatalf("expected 4 children, got %d", len(node.Children))
	}
	if node.Children[0].Description != "Initialize a project" {
		t.Errorf("init description: %q", node.Children[0].Description)
	}
	if node.Children[1].Name != "build" || node.Children[1].Description != "" {
		t.Errorf("build: name=%q desc=%q", node.Children[1].Name, node.Children[1].Description)
	}
	if node.Children[2].Description != "Run all tests" {
		t.Errorf("test description: %q", node.Children[2].Description)
	}
	if node.Children[3].Name != "clean" || node.Children[3].Description != "" {
		t.Errorf("clean: name=%q desc=%q", node.Children[3].Name, node.Children[3].Description)
	}
}

func TestParseBareSubcommandsNoHeader(t *testing.T) {
	// No section header, bare command names
	help := `Usage: baretool [command]

  init
  build
  test
`
	node := Parse("baretool", help)

	if len(node.Children) != 3 {
		t.Errorf("expected 3 children, got %d", len(node.Children))
		for i, c := range node.Children {
			t.Logf("  child[%d]: %q", i, c.Name)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	node := Parse("empty", "")
	if node.Name != "empty" {
		t.Errorf("expected name 'empty', got %q", node.Name)
	}
	if len(node.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(node.Children))
	}
}
