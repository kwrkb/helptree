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

type focus int

const (
	focusTree focus = iota
	focusDetail
)

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
	focus        focus
	searchQuery  string
	searchHits   map[int]bool // set of item indices that match
	searchOrder  []int        // ordered list of hit indices for n/N navigation
	loading      bool
	cachedTreeW  int // cached max tree line width, updated on reflatten
}

// New creates a new Model from a root node.
func New(root *model.Node, rootCmd string) Model {
	root.Expanded = true
	items := flattenTree(root)
	m := Model{
		root:    root,
		rootCmd: rootCmd,
		items:   items,
		focus:   focusTree,
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

	case tea.MouseMsg:
		if m.mode != modeNormal {
			return m, nil
		}
		if msg.Type == tea.MouseWheelUp {
			if m.focus == focusTree {
				if m.cursor > 0 {
					m.cursor--
					m.detailScroll = 0
				}
			} else {
				m.scrollDetailBy(-3)
			}
		} else if msg.Type == tea.MouseWheelDown {
			if m.focus == focusTree {
				if m.cursor < len(m.items)-1 {
					m.cursor++
					m.detailScroll = 0
				}
			} else {
				m.scrollDetailBy(3)
			}
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

	case "tab":
		if m.focus == focusTree {
			m.focus = focusDetail
		} else {
			m.focus = focusTree
		}

	case "j", "down":
		if m.focus == focusTree {
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.detailScroll = 0
			}
		} else {
			m.scrollDetailBy(1)
		}

	case "k", "up":
		if m.focus == focusTree {
			if m.cursor > 0 {
				m.cursor--
				m.detailScroll = 0
			}
		} else {
			m.scrollDetailBy(-1)
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
		m.scrollDetailBy(5)

	case "ctrl+u":
		m.scrollDetailBy(-5)

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

// detailViewportHeight returns the detail pane's inner row count, mirroring
// the layout math in View().
func (m Model) detailViewportHeight() int {
	panesHeight := m.height - 4
	if panesHeight < 4 {
		panesHeight = 4
	}
	summaryHeight := panesHeight / 3
	if summaryHeight < 4 {
		summaryHeight = 4
	}
	if summaryHeight > panesHeight-3 {
		summaryHeight = panesHeight - 3
	}
	if summaryHeight < 3 {
		summaryHeight = 3
	}
	inner := (panesHeight - summaryHeight) - 2
	if inner < 1 {
		inner = 1
	}
	return inner
}

// scrollDetailBy adjusts m.detailScroll by delta and clamps it so the pane
// stays filled with content (max scroll = totalRows - viewport).
func (m *Model) scrollDetailBy(delta int) {
	m.detailScroll += delta
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
	if m.cursor >= len(m.items) {
		m.detailScroll = 0
		return
	}
	node := m.items[m.cursor].node
	rows := 0
	if len(node.Children) > 0 {
		rows += 1 + len(node.Children)
	}
	if len(node.Options) > 0 {
		if rows > 0 {
			rows++ // blank line between sections
		}
		rows += 1 + len(node.Options)
	}
	max := rows - m.detailViewportHeight()
	if max < 0 {
		max = 0
	}
	if m.detailScroll > max {
		m.detailScroll = max
	}
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

	// Outer layout (border-inclusive sizes).
	// title (1) + panes (panesHeight) + bottom (1) ≈ m.height; the extra
	// margin gives bubbletea breathing room at the bottom of the screen.
	panesHeight := m.height - 4
	if panesHeight < 4 {
		panesHeight = 4
	}

	// Tree pane outer width (border included). Cached content width + 2 borders.
	treeWidth := m.cachedTreeW + 2
	if treeWidth < 20 {
		treeWidth = 20
	}
	if treeWidth > m.width/2 {
		treeWidth = m.width / 2
	}
	rightWidth := m.width - treeWidth

	// Summary pane outer height: ~1/3 of pane area, min 4.
	summaryHeight := panesHeight / 3
	if summaryHeight < 4 {
		summaryHeight = 4
	}
	if summaryHeight > panesHeight-3 {
		summaryHeight = panesHeight - 3
	}
	if summaryHeight < 3 {
		summaryHeight = 3
	}
	detailHeight := panesHeight - summaryHeight

	// Title
	titleText := fmt.Sprintf(" helptree: %s ", m.rootCmd)
	if m.loading {
		titleText += "[loading...] "
	}
	title := titleStyle.Render(truncateWidth(titleText, m.width))

	// Tree pane (left). Inner = outer - border(2). No padding.
	treeInnerW := treeWidth - 2
	treeInnerH := panesHeight - 2
	treeContent := m.renderTreePane(treeInnerW, treeInnerH)
	treePaneS := paneStyle
	if m.focus == focusTree {
		treePaneS = activePaneStyle
	}
	treePane := treePaneS.
		Width(treeInnerW).
		Height(treeInnerH).
		Render(treeContent)

	// Selected node for right panes
	var selected *model.Node
	if m.cursor < len(m.items) {
		selected = m.items[m.cursor].node
	}

	// Right panes: lipgloss.Width(w) sets the area inside the border
	// (padding is consumed within w). So:
	//   outer width  = Width(w) + border(2)        → w = rightWidth - 2
	//   content width = w - padding(2)              = rightWidth - 4
	//   outer height = Height(h) + border(2)        → h = paneH - 2
	rightPaneW := rightWidth - 2
	if rightPaneW < 1 {
		rightPaneW = 1
	}
	rightContentW := rightWidth - 4
	if rightContentW < 1 {
		rightContentW = 1
	}
	summaryInnerH := summaryHeight - 2
	if summaryInnerH < 1 {
		summaryInnerH = 1
	}
	detailInnerH := detailHeight - 2
	if detailInnerH < 1 {
		detailInnerH = 1
	}

	// Summary pane (top-right)
	summaryContent := renderSummary(selected, rightContentW, summaryInnerH)
	summaryPane := paneStyle.
		Width(rightPaneW).
		Height(summaryInnerH).
		Padding(0, 1).
		Render(summaryContent)

	// Detail pane (bottom-right)
	detailContent := renderDetail(selected, rightContentW, detailInnerH, m.detailScroll)
	detailPaneS := paneStyle
	if m.focus == focusDetail {
		detailPaneS = activePaneStyle
	}
	detailPane := detailPaneS.
		Width(rightPaneW).
		Height(detailInnerH).
		Padding(0, 1).
		Render(detailContent)

	// Combine right panes, then combine with tree
	rightPanes := lipgloss.JoinVertical(lipgloss.Left, summaryPane, detailPane)
	panes := lipgloss.JoinHorizontal(lipgloss.Top, treePane, rightPanes)

	// Bottom bar
	var bottomText string
	bottomStyle := helpStyle
	switch m.mode {
	case modeSearch:
		bottomStyle = searchStyle
		bottomText = fmt.Sprintf("  /%s▏ (%d matches)", m.searchQuery, len(m.searchHits))
	default:
		if m.searchQuery != "" {
			bottomText = fmt.Sprintf("  /%s (%d matches)  n/N: next/prev  esc: clear  ?: help", m.searchQuery, len(m.searchHits))
		} else {
			nav := "j/k: navigate"
			if m.focus == focusDetail {
				nav = "j/k: scroll detail"
			}
			bottomText = fmt.Sprintf("  %s  tab: switch focus  enter/l: expand  h: collapse  /: search  ?: help  q: quit", nav)
		}
	}
	bottom := bottomStyle.Render(truncateWidth(bottomText, m.width))

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
		indicator = truncateWidth(indicator, width)
		lines = append(lines, helpStyle.Render(indicator))
	}

	for i := start; i < end; i++ {
		line := renderTreeLine(m.items[i], i == m.cursor, width)

		switch {
		case i == m.cursor:
			lineW := lipgloss.Width(line)
			if lineW < width {
				line = line + strings.Repeat(" ", width-lineW)
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
		indicator = truncateWidth(indicator, width)
		lines = append(lines, helpStyle.Render(indicator))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderHelpOverlay() string {
	help := `╭─ Keybindings ───────────────────────╮

 Navigation
   j / ↓       Move down (or scroll detail when focused)
   k / ↑       Move up   (or scroll detail when focused)
   PgDn/Ctrl+f Page down
   PgUp/Ctrl+b Page up
   g           Go to top
   G           Go to bottom

 Focus
   Tab         Switch focus between Tree and Detail

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

 Mouse
   Wheel       Scroll the focused pane
   (Hold Shift to select text in most terminals)

 General
   ?           Show this help
   q / Ctrl+c  Quit

╰─ Press any key to close ───────────╯`

	lines := strings.Split(help, "\n")
	var padded []string
	for _, line := range lines {
		pad := (m.width - lipgloss.Width(line)) / 2
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
