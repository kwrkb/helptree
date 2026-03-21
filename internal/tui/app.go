package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kwrkb/helptree/internal/model"
	"github.com/kwrkb/helptree/internal/runner"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12"))

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("15"))

	normalStyle = lipgloss.NewStyle()

	matchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	searchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	activePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("12"))
)

type mode int

const (
	modeNormal mode = iota
	modeSearch
	modeHelp
)

// nodeLoadedMsg is sent when a node's --help has been fetched.
// target is the original (unloaded) node in the tree; result is the parsed data.
type nodeLoadedMsg struct {
	target *model.Node
	result *model.Node
	err    error
}

// Model is the main Bubble Tea model.
type Model struct {
	root         *model.Node
	rootCmd      string
	items        []flatItem
	cursor       int
	detailScroll int
	width        int
	height       int
	ready        bool
	mode         mode
	searchQuery  string
	searchHits   map[int]bool // set of item indices that match
	searchOrder  []int        // ordered list of hit indices for n/N navigation
	loading       bool
	cachedTreeW   int // cached max tree line width, updated on reflatten
}

// New creates a new Model from a root node.
func New(root *model.Node, rootCmd string) Model {
	root.Expanded = true
	items := flattenTree(root)
	m := Model{
		root:    root,
		rootCmd: rootCmd,
		items:   items,
	}
	m.recalcTreeWidth()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case nodeLoadedMsg:
		m.loading = false
		if msg.err == nil && msg.result != nil {
			// Merge loaded data into the target node (safe: runs on main goroutine)
			msg.target.Children = msg.result.Children
			msg.target.Options = msg.result.Options
			msg.target.Usage = msg.result.Usage
			if msg.target.Description == "" {
				msg.target.Description = msg.result.Description
			}
			msg.target.Loaded = true
			msg.target.Expanded = true
		} else {
			msg.target.Loaded = true // prevent retry on error
		}
		m.items = flattenTree(m.root)
		m.recalcTreeWidth()
		if m.searchQuery != "" {
			m.rebuildSearchHits()
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeHelp:
			m.mode = modeNormal
			return m, nil
		case modeSearch:
			return m.updateSearch(msg)
		case modeNormal:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
			m.detailScroll = 0
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.detailScroll = 0
		}

	case "g":
		m.cursor = 0
		m.detailScroll = 0

	case "G":
		m.cursor = len(m.items) - 1
		m.detailScroll = 0

	case "enter", "l", "right", " ":
		if cmd := m.toggleNode(); cmd != nil {
			return m, cmd
		}

	case "h", "left":
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if item.node.Expanded {
				item.node.Expanded = false
				m.reflattenTree()
			}
		}

	case "/":
		m.mode = modeSearch
		m.searchQuery = ""
		m.searchHits = nil
		m.searchOrder = nil

	case "?":
		m.mode = modeHelp

	case "n":
		m.jumpToNextHit(1)

	case "N":
		m.jumpToNextHit(-1)

	case "pgdown", "ctrl+f":
		contentHeight := m.height - 4
		if contentHeight < 1 {
			contentHeight = 1
		}
		m.cursor += contentHeight / 2
		if m.cursor >= len(m.items) {
			m.cursor = len(m.items) - 1
		}
		m.detailScroll = 0

	case "pgup", "ctrl+b":
		contentHeight := m.height - 4
		if contentHeight < 1 {
			contentHeight = 1
		}
		m.cursor -= contentHeight / 2
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.detailScroll = 0

	case "ctrl+d":
		m.detailScroll += 5

	case "ctrl+u":
		m.detailScroll -= 5
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}

	case "esc":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.searchHits = nil
			m.searchOrder = nil
		}
	}
	return m, nil
}

// toggleNode handles expand/collapse/load for the current cursor node.
func (m *Model) toggleNode() tea.Cmd {
	if m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	if !item.node.Loaded {
		m.loading = true
		target := item.node
		rootCmd := m.rootCmd
		// Build the full subcommand name for this node
		nodeName := m.nodeFullName(m.cursor)
		return func() tea.Msg {
			result, err := runner.LoadNode(context.Background(), rootCmd, nodeName)
			return nodeLoadedMsg{target: target, result: result, err: err}
		}
	}
	if len(item.node.Children) > 0 {
		item.node.Expanded = !item.node.Expanded
		m.reflattenTree()
	}
	return nil
}

// nodeFullName builds the full command path for a node at the given index.
// e.g. for "container" under "docker", returns "docker container".
func (m *Model) nodeFullName(idx int) string {
	if idx >= len(m.items) {
		return ""
	}
	item := m.items[idx]
	// Walk up the tree to build the path
	var parts []string
	parts = append(parts, item.node.Name)
	// Find ancestor nodes by depth
	for i := idx - 1; i >= 0; i-- {
		if m.items[i].depth < item.depth {
			parts = append([]string{m.items[i].node.Name}, parts...)
			item = m.items[i]
			if item.depth == 0 {
				break
			}
		}
	}
	// The root node already has the command name, children have just short names
	// Build: rootCmd + child names
	if len(parts) > 1 {
		return m.rootCmd + " " + strings.Join(parts[1:], " ")
	}
	return m.rootCmd
}

// recalcTreeWidth recalculates the cached max tree line width.
func (m *Model) recalcTreeWidth() {
	maxW := 0
	for _, item := range m.items {
		if w := treeLineWidth(item); w > maxW {
			maxW = w
		}
	}
	m.cachedTreeW = maxW
}

// reflattenTree rebuilds the flat item list and updates search hits if active.
func (m *Model) reflattenTree() {
	m.items = flattenTree(m.root)
	m.recalcTreeWidth()
	if m.searchQuery != "" {
		m.rebuildSearchHits()
	}
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = modeNormal
		if len(m.searchOrder) > 0 {
			m.cursor = m.searchOrder[0]
			m.detailScroll = 0
		}

	case "esc":
		m.mode = modeNormal
		m.searchQuery = ""
		m.searchHits = nil
		m.searchOrder = nil

	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.rebuildSearchHits()
		}

	default:
		if len(msg.String()) == 1 {
			m.searchQuery += msg.String()
			m.rebuildSearchHits()
		}
	}
	return m, nil
}

// rebuildSearchHits rebuilds both the hit map and ordered slice.
func (m *Model) rebuildSearchHits() {
	m.searchHits = nil
	m.searchOrder = nil
	if m.searchQuery == "" {
		return
	}
	q := strings.ToLower(m.searchQuery)
	hits := make(map[int]bool)
	for i, item := range m.items {
		name := strings.ToLower(item.node.Name)
		desc := strings.ToLower(item.node.Description)
		if strings.Contains(name, q) || strings.Contains(desc, q) {
			hits[i] = true
			m.searchOrder = append(m.searchOrder, i)
		}
	}
	m.searchHits = hits
}

func (m *Model) jumpToNextHit(dir int) {
	if len(m.searchOrder) == 0 {
		return
	}
	if dir > 0 {
		for _, idx := range m.searchOrder {
			if idx > m.cursor {
				m.cursor = idx
				m.detailScroll = 0
				return
			}
		}
		m.cursor = m.searchOrder[0]
	} else {
		for i := len(m.searchOrder) - 1; i >= 0; i-- {
			if m.searchOrder[i] < m.cursor {
				m.cursor = m.searchOrder[i]
				m.detailScroll = 0
				return
			}
		}
		m.cursor = m.searchOrder[len(m.searchOrder)-1]
	}
	m.detailScroll = 0
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.mode == modeHelp {
		return m.renderHelpOverlay()
	}

	// Tree width from cached content width (updated on reflatten)
	treeWidth := m.cachedTreeW + 4 // padding + border
	if treeWidth < 20 {
		treeWidth = 20
	}
	// Cap at 50% of terminal width so right panes have enough space
	if treeWidth > m.width/2 {
		treeWidth = m.width / 2
	}
	rightWidth := m.width - treeWidth - 4
	contentHeight := m.height - 4

	if contentHeight < 1 {
		contentHeight = 1
	}

	// Summary pane height: ~20% of content, min 4
	summaryHeight := contentHeight / 5
	if summaryHeight < 4 {
		summaryHeight = 4
	}
	// Detail pane gets the rest (minus border gap)
	detailHeight := contentHeight - summaryHeight - 2
	if detailHeight < 3 {
		detailHeight = 3
	}

	// Title
	titleText := fmt.Sprintf(" helptree: %s ", m.rootCmd)
	if m.loading {
		titleText += "[loading...] "
	}
	title := titleStyle.Render(titleText)

	// Tree pane (left, full height)
	treeContent := m.renderTreePane(treeWidth-4, contentHeight)
	treePane := activePaneStyle.
		Width(treeWidth - 2).
		Height(contentHeight).
		Render(treeContent)

	// Right side: summary + detail
	var selected *model.Node
	if m.cursor < len(m.items) {
		selected = m.items[m.cursor].node
	}

	// Summary pane (top-right)
	summaryContent := renderSummary(selected, rightWidth-4)
	summaryPane := paneStyle.
		Width(rightWidth - 2).
		Height(summaryHeight).
		Render(summaryContent)

	// Detail pane (bottom-right)
	detailContent := renderDetail(selected, rightWidth-4, detailHeight, m.detailScroll)
	detailPane := paneStyle.
		Width(rightWidth - 2).
		Height(detailHeight).
		Render(detailContent)

	// Join right panes vertically, then combine with tree
	rightPanes := lipgloss.JoinVertical(lipgloss.Left, summaryPane, detailPane)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, treePane, rightPanes)

	// Bottom bar
	var bottom string
	switch m.mode {
	case modeSearch:
		bottom = searchStyle.Render(fmt.Sprintf("  /%s▏ (%d matches)", m.searchQuery, len(m.searchHits)))
	default:
		if m.searchQuery != "" {
			bottom = helpStyle.Render(fmt.Sprintf("  /%s (%d matches)  n/N: next/prev  esc: clear  ?: help", m.searchQuery, len(m.searchHits)))
		} else {
			bottom = helpStyle.Render("  j/k: navigate  enter/l: expand  h: collapse  /: search  ?: help  q: quit")
		}
	}

	return title + "\n" + panes + "\n" + bottom
}

func (m Model) renderTreePane(width, height int) string {
	var lines []string

	// Calculate visible range (scrolling)
	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1
	}
	end := start + height
	if end > len(m.items) {
		end = len(m.items)
	}

	// Reserve space for scroll indicators
	hasAbove := start > 0
	hasBelow := end < len(m.items)
	visibleHeight := height
	if hasAbove {
		visibleHeight--
	}
	if hasBelow {
		visibleHeight--
	}

	// Recalculate visible range with reserved space
	end = start + visibleHeight
	if end > len(m.items) {
		end = len(m.items)
	}
	// Recompute hasBelow after adjusting end
	hasBelow = end < len(m.items)

	// Top scroll indicator
	if hasAbove {
		indicator := fmt.Sprintf("  ▲ %d more above", start)
		if len(indicator) > width {
			indicator = indicator[:width]
		}
		lines = append(lines, helpStyle.Render(indicator))
	}

	for i := start; i < end; i++ {
		line := renderTreeLine(m.items[i], i == m.cursor, width)

		switch {
		case i == m.cursor:
			if len(line) < width {
				line = line + strings.Repeat(" ", width-len(line))
			}
			line = selectedStyle.Render(line)
		case m.searchHits[i]:
			line = matchStyle.Render(line)
		default:
			line = normalStyle.Render(line)
		}

		lines = append(lines, line)
	}

	// Bottom scroll indicator
	if hasBelow {
		remaining := len(m.items) - end
		indicator := fmt.Sprintf("  ▼ %d more below", remaining)
		if len(indicator) > width {
			indicator = indicator[:width]
		}
		lines = append(lines, helpStyle.Render(indicator))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderHelpOverlay() string {
	help := `╭─ Keybindings ───────────────────────╮

 Navigation
   j / ↓       Move down
   k / ↑       Move up
   PgDn/Ctrl+f Page down
   PgUp/Ctrl+b Page up
   g           Go to top
   G           Go to bottom

 Tree
   Enter / l   Expand / Load children
   h           Collapse
   Space       Toggle expand/collapse

 Search
   /           Start search
   n           Next match
   N           Previous match
   Esc         Clear search

 Detail Pane
   Ctrl+d      Scroll detail down
   Ctrl+u      Scroll detail up

 General
   ?           Show this help
   q / Ctrl+c  Quit

╰─ Press any key to close ───────────╯`

	lines := strings.Split(help, "\n")
	var padded []string
	for _, line := range lines {
		pad := (m.width - len(line)) / 2
		if pad < 0 {
			pad = 0
		}
		padded = append(padded, strings.Repeat(" ", pad)+line)
	}

	topPad := (m.height - len(lines)) / 2
	if topPad < 0 {
		topPad = 0
	}

	return strings.Repeat("\n", topPad) + strings.Join(padded, "\n")
}
