package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/kwrkb/helptree/internal/model"
)

func testModel() Model {
	root := &model.Node{
		Name:        "go",
		Description: "Go is a tool for managing Go source code.",
		Usage:       "go <command> [arguments]",
		Loaded:      true,
		Children: []*model.Node{
			{Name: "go bug", Description: "start a bug report"},
			{Name: "go build", Description: "compile packages and dependencies"},
			{Name: "go clean", Description: "remove object files and cached files"},
			{Name: "go doc", Description: "show documentation for package or symbol"},
			{Name: "go env", Description: "print Go environment information"},
			{Name: "go generate", Description: "generate Go files by processing source"},
		},
		Options: []model.Option{
			{Short: "-C", Arg: "dir", Description: "change to dir before running the command"},
			{Long: "--help", Description: "show help"},
			{Long: "--very-long-option-name", Arg: "value", Description: "description that should be truncated inside the detail pane"},
		},
	}
	return New(root, "go")
}

func resize(m Model, width, height int) Model {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updated.(Model)
}

func assertViewFits(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(ansi.Strip(view), "\n")
	if len(lines) > height {
		t.Fatalf("view height = %d, want <= %d\n%s", len(lines), height, view)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q\n%s", i+1, got, width, line, view)
		}
	}
}

func TestViewFitsCommonTerminalSizes(t *testing.T) {
	for _, size := range []struct {
		width  int
		height int
	}{
		{80, 24},
		{60, 18},
		{40, 12},
	} {
		m := resize(testModel(), size.width, size.height)
		assertViewFits(t, m.View(), size.width, size.height)
	}
}
