package tui

import (
	"strings"

	"github.com/kwrkb/helptree/internal/model"
)

// flatItem represents a node in the flattened tree for display.
type flatItem struct {
	node  *model.Node
	depth int
}

// flattenTree converts the tree into a flat slice for rendering.
func flattenTree(root *model.Node) []flatItem {
	var items []flatItem
	flattenNode(root, 0, &items)
	return items
}

func flattenNode(node *model.Node, depth int, items *[]flatItem) {
	*items = append(*items, flatItem{node: node, depth: depth})
	if node.Expanded {
		for _, child := range node.Children {
			flattenNode(child, depth+1, items)
		}
	}
}

// treeLineWidth returns the natural width of a tree line (without truncation).
func treeLineWidth(item flatItem) int {
	indent := 2 * item.depth // "  " per depth
	prefix := 2              // "▶ " or "  "
	name := item.node.Name
	if item.depth > 0 {
		parts := strings.Fields(name)
		if len(parts) > 0 {
			name = parts[len(parts)-1]
		}
	}
	return indent + prefix + len(name)
}

// renderTreeLine renders a single tree line with indent and expand indicator.
func renderTreeLine(item flatItem, selected bool, width int) string {
	indent := strings.Repeat("  ", item.depth)

	prefix := "  "
	if len(item.node.Children) > 0 || (!item.node.Loaded && item.depth > 0) {
		if item.node.Expanded {
			prefix = "▼ "
		} else {
			prefix = "▶ "
		}
	} else if item.depth > 0 {
		prefix = "○ "
	}

	name := item.node.Name
	// For non-root nodes, show just the command name (last part)
	if item.depth > 0 {
		parts := strings.Fields(name)
		if len(parts) > 0 {
			name = parts[len(parts)-1]
		}
	}

	line := indent + prefix + name

	// Truncate to width
	if len(line) > width && width > 3 {
		line = line[:width-3] + "..."
	}

	return line
}
