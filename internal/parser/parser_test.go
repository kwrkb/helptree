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

func TestParseBSDCompactUsage(t *testing.T) {
	// BSD ls style: compact option groups in usage line
	help := `ls: unrecognized option ` + "`" + `--help'
usage: ls [-@ABCFGHILOPRSTUWXabcdefghiklmnopqrstuvwxy1%,] [--color=when] [-D format] [file ...]`
	node := Parse("ls", help)

	if len(node.Options) < 20 {
		t.Errorf("expected at least 20 options from compact usage, got %d", len(node.Options))
	}

	// Check that individual short flags are expanded
	shortFlags := make(map[string]bool)
	for _, o := range node.Options {
		if o.Short != "" {
			shortFlags[o.Short] = true
		}
	}
	for _, flag := range []string{"-a", "-l", "-R", "-F", "-G"} {
		if !shortFlags[flag] {
			t.Errorf("expected short flag %s to be extracted", flag)
		}
	}

	// Check long option with argument
	foundColor := false
	for _, o := range node.Options {
		if o.Long == "--color" && o.Arg == "when" {
			foundColor = true
		}
	}
	if !foundColor {
		t.Error("expected --color=when option")
	}

	// Check -D has arg "format"
	foundD := false
	for _, o := range node.Options {
		if o.Short == "-D" && o.Arg == "format" {
			foundD = true
		}
	}
	if !foundD {
		t.Error("expected -D format option")
	}
}

func TestParseBSDGrepUsage(t *testing.T) {
	help := `usage: grep [-abcdDEFGHhIiJLlMmnOopqRSsUVvwXxZz] [-A num] [-B num] [-C[num]]
	[-e pattern] [-f file] [--binary-files=value] [--color=when]
	[--context[=num]] [--directories=action] [--label] [--line-buffered]
	[--null] [pattern] [file ...]`
	node := Parse("grep", help)

	if len(node.Options) < 30 {
		t.Errorf("expected at least 30 options, got %d", len(node.Options))
	}

	// Check short with arg
	foundA := false
	for _, o := range node.Options {
		if o.Short == "-A" && o.Arg == "num" {
			foundA = true
		}
	}
	if !foundA {
		t.Error("expected -A num option")
	}

	// Check long options
	foundNull := false
	foundBinaryFiles := false
	for _, o := range node.Options {
		if o.Long == "--null" {
			foundNull = true
		}
		if o.Long == "--binary-files" && o.Arg == "value" {
			foundBinaryFiles = true
		}
	}
	if !foundNull {
		t.Error("expected --null option")
	}
	if !foundBinaryFiles {
		t.Error("expected --binary-files=value option")
	}
}

func TestParseBSDRmUsage(t *testing.T) {
	help := `rm: illegal option -- -
usage: rm [-f | -i] [-dIPRrvWx] file ...
       unlink [--] file`
	node := Parse("rm", help)

	if len(node.Options) < 3 {
		t.Errorf("expected at least 3 options, got %d", len(node.Options))
	}

	shortFlags := make(map[string]bool)
	for _, o := range node.Options {
		if o.Short != "" {
			shortFlags[o.Short] = true
		}
	}
	for _, flag := range []string{"-f", "-i", "-r", "-R"} {
		if !shortFlags[flag] {
			t.Errorf("expected short flag %s", flag)
		}
	}
}

func TestParseMultiFlagLine(t *testing.T) {
	help := `tar(bsdtar): manipulate archive files
Common Options:
  -v    Verbose
  -w    Interactive
Create: tar -c [options]
  -z, -j, -J, --lzma  Compress archive with gzip/bzip2/xz/lzma
  --format {ustar|pax|cpio|shar}  Select archive format`
	node := Parse("tar", help)

	// Multi-flag line should produce 4 options: -z, -j, -J, --lzma
	flags := make(map[string]bool)
	for _, o := range node.Options {
		if o.Short != "" {
			flags[o.Short] = true
		}
		if o.Long != "" {
			flags[o.Long] = true
		}
	}
	for _, f := range []string{"-z", "-j", "-J", "--lzma"} {
		if !flags[f] {
			t.Errorf("expected flag %s from multi-flag line", f)
		}
	}
}

func TestParseInlineMultiOptions(t *testing.T) {
	help := `tool: a tool
First option must be a mode specifier:
  -c Create  -r Add/Replace  -t List  -u Update  -x Extract
`
	node := Parse("tool", help)

	flags := make(map[string]bool)
	for _, o := range node.Options {
		if o.Short != "" {
			flags[o.Short] = true
		}
	}
	for _, f := range []string{"-c", "-r", "-t", "-u", "-x"} {
		if !flags[f] {
			t.Errorf("expected flag %s from inline multi-option line", f)
		}
	}
}

func TestParseUppercaseSectionHeaders(t *testing.T) {
	// glab-style: uppercase section headers without colon, subcommands with meta tokens
	help := `  GLab is an open source GitLab CLI tool.

  USAGE

    glab <command> <subcommand> [flags]

  COMMANDS

    alias [command] [--flags]                        Create, list, and delete aliases.
    api <endpoint> [--flags]                         Make an authenticated request.
    auth <command> [command]                         Manage authentication state.
    check-update                                     Check for latest releases.
    ci <command> [command] [--flags]                 Work with CI/CD pipelines.
    duo <command> prompt [command]                   Work with GitLab Duo
    version                                          Show version information.

  FLAGS

    -h --help                                        Show help for this command.
`
	node := Parse("glab", help)

	if node.Description != "GLab is an open source GitLab CLI tool." {
		t.Errorf("unexpected description: %q", node.Description)
	}
	if len(node.Children) < 7 {
		t.Errorf("expected at least 7 children, got %d", len(node.Children))
		for i, c := range node.Children {
			t.Logf("  child[%d]: %q desc=%q", i, c.Name, c.Description)
		}
	}

	// Verify meta-token subcommand is parsed correctly
	found := false
	for _, c := range node.Children {
		if c.Name == "alias" {
			found = true
			if !strings.Contains(c.Description, "Create, list, and delete aliases") {
				t.Errorf("alias description: %q", c.Description)
			}
		}
	}
	if !found {
		t.Error("'alias' subcommand not found")
	}

	// Verify simple subcommand (no meta tokens)
	found = false
	for _, c := range node.Children {
		if c.Name == "check-update" {
			found = true
			if !strings.Contains(c.Description, "Check for latest") {
				t.Errorf("check-update description: %q", c.Description)
			}
		}
	}
	if !found {
		t.Error("'check-update' subcommand not found")
	}
}

func TestParseBinaryNamePrefix(t *testing.T) {
	// gemini-style: subcommand lines prefixed with binary name
	help := `Usage: gemini [options] [command]

Gemini CLI

Commands:
  gemini [query..]             Launch Gemini CLI  [default]
  gemini mcp                   Manage MCP servers
  gemini extensions <command>  Manage extensions.

Options:
  -d, --debug          Run in debug mode  [boolean]
  -m, --model          Model  [string]
  -p, --prompt         Run in non-interactive mode  [string]
  -h, --help           Show help  [boolean]
`
	node := Parse("gemini", help)

	if len(node.Children) < 3 {
		t.Errorf("expected at least 3 children, got %d", len(node.Children))
		for i, c := range node.Children {
			t.Logf("  child[%d]: %q desc=%q", i, c.Name, c.Description)
		}
	}

	// "gemini mcp" should become just "mcp" after prefix stripping
	found := false
	for _, c := range node.Children {
		if c.Name == "mcp" {
			found = true
			if !strings.Contains(c.Description, "Manage MCP") {
				t.Errorf("mcp description: %q", c.Description)
			}
		}
	}
	if !found {
		t.Error("'mcp' subcommand not found (binary prefix not stripped?)")
	}

	// "gemini extensions <command>" should become "extensions"
	found = false
	for _, c := range node.Children {
		if c.Name == "extensions" {
			found = true
		}
	}
	if !found {
		t.Error("'extensions' subcommand not found")
	}

	if len(node.Options) < 4 {
		t.Errorf("expected at least 4 options, got %d", len(node.Options))
	}
}

func TestParseColonSepOptions(t *testing.T) {
	// python3-style: colon-separated short options
	help := `usage: python3 [option] ... [-c cmd | -m mod | file | -] [arg] ...
Options and arguments (and corresponding environment variables):
-b     : issue warnings about str(bytes_instance)
         and comparing bytes/bytearray with str. (-bb: issue errors)
-B     : don't write .pyc files on import
-c cmd : program passed in as string (terminates option list)
-d     : turn on parser debugging output
-E     : ignore PYTHON* environment variables
-h     : print this help message and exit (also --help)
-i     : inspect interactively after running script
-O     : remove assert and __debug__-dependent statements
-q     : don't print version and copyright messages
-s     : don't add user site directory to sys.path
-v     : verbose (trace import statements)
-V     : print the Python version number and exit
-W arg : warning control
-x     : skip first line of source
`
	node := Parse("python3", help)

	if len(node.Options) < 10 {
		t.Errorf("expected at least 10 options, got %d", len(node.Options))
		for i, o := range node.Options {
			t.Logf("  opt[%d]: %s arg=%q desc=%q", i, o.Short, o.Arg, o.Description)
		}
	}

	// Check -b has continuation
	for _, o := range node.Options {
		if o.Short == "-b" {
			if !strings.Contains(o.Description, "comparing bytes") {
				t.Errorf("-b should have wrapped description, got %q", o.Description)
			}
		}
	}

	// Check -c has arg "cmd"
	found := false
	for _, o := range node.Options {
		if o.Short == "-c" && o.Arg == "cmd" {
			found = true
		}
	}
	if !found {
		t.Error("-c with arg 'cmd' not found")
	}
}

func TestParseColumnZeroOptions(t *testing.T) {
	// fvm-style: options starting at column 0
	help := `Flutter Version Management: A cli to manage Flutter SDK versions.

Usage: fvm <command> [arguments]

Global options:
-h, --help       Print this usage information.
    --verbose    Print verbose output.
-v, --version    Print the current version.

Available commands:
  api        Provides JSON API access
  config     Configure global settings
  install    Installs a Flutter SDK version
  use        Sets the Flutter SDK version
`
	node := Parse("fvm", help)

	if len(node.Options) < 3 {
		t.Errorf("expected at least 3 options, got %d", len(node.Options))
		for i, o := range node.Options {
			t.Logf("  opt[%d]: %s %s desc=%q", i, o.Short, o.Long, o.Description)
		}
	}

	// Check -h, --help at column 0
	found := false
	for _, o := range node.Options {
		if o.Short == "-h" && o.Long == "--help" {
			found = true
		}
	}
	if !found {
		t.Error("-h, --help option not found (column 0 not supported?)")
	}

	if len(node.Children) < 4 {
		t.Errorf("expected at least 4 children, got %d", len(node.Children))
	}
}

func TestParseBracketInlineOptions(t *testing.T) {
	// npx-style: bracket-enclosed options
	help := `Run a command from a local or remote npm package

Usage:
npm exec -- <pkg>[@<version>] [args...]

Options:
[--package <package-spec> [--package <package-spec> ...]] [-c|--call <call>]
[-w|--workspace <workspace-name> [-w|--workspace <workspace-name> ...]]
[--workspaces] [--include-workspace-root]

Run "npm help exec" for more info`
	node := Parse("npx", help)

	if len(node.Options) < 4 {
		t.Errorf("expected at least 4 options, got %d", len(node.Options))
		for i, o := range node.Options {
			t.Logf("  opt[%d]: %s %s", i, o.Short, o.Long)
		}
	}

	// Check --package
	found := false
	for _, o := range node.Options {
		if o.Long == "--package" {
			found = true
		}
	}
	if !found {
		t.Error("--package option not found")
	}

	// Check -c|--call pipe pair
	found = false
	for _, o := range node.Options {
		if o.Short == "-c" && o.Long == "--call" {
			found = true
		}
	}
	if !found {
		t.Error("-c/--call pipe option not found")
	}

	// Check --workspaces
	found = false
	for _, o := range node.Options {
		if o.Long == "--workspaces" {
			found = true
		}
	}
	if !found {
		t.Error("--workspaces option not found")
	}
}

func TestParseUsageFallbackOnlyWhenFewOptions(t *testing.T) {
	// GNU-style help with enough options should NOT trigger usage fallback
	help := `Usage: tool [-abc] [options]

Options:
  -v, --verbose        Enable verbose
  -q, --quiet          Suppress output
  -o, --output string  Output file
  -d, --debug          Debug mode
  -f, --force          Force operation
`
	node := Parse("tool", help)

	// Should have 5 GNU-parsed options, no usage fallback
	if len(node.Options) != 5 {
		t.Errorf("expected exactly 5 options, got %d", len(node.Options))
		for _, o := range node.Options {
			t.Logf("  %s %s", o.Short, o.Long)
		}
	}
}
