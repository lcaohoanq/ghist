package tui

import "github.com/lcaohoanq/ghist/internal/history"

// graphCell holds the pre-rendered graph prefix for a single history row.
// It is computed once for the entire version list via buildGraphCells.
type graphCell struct {
	symbol    string // the node glyph line  e.g. "● " or "◆ "
	connector string // the line between two nodes, e.g. "│ " or "  "
}

// buildGraphCells returns one graphCell per FileVersion in the same order.
//
// Visual rules (file-scoped single-lane graph):
//
//	● — normal commit  (0 or 1 parent)
//	◆ — merge commit   (2+ parents, len(Commit.Parents) > 1)
//	│   — vertical connector between consecutive commits
//	    — no connector above the root commit (no parent)
//
// Because ghist only tracks one file across the DAG, the "graph" is always a
// single lane — we are not drawing all branches, just annotating the linear
// file history with merge-awareness. The connector below a merge commit stays
// │ because the file history continues through the first parent.
func buildGraphCells(versions []history.FileVersion) []graphCell {
	cells := make([]graphCell, len(versions))
	for i, v := range versions {
		// Node symbol
		if len(v.Commit.Parents) > 1 {
			cells[i].symbol = "◆ "
		} else {
			cells[i].symbol = "● "
		}

		// Connector drawn *below* this node (shown above the next node).
		// No connector below the last commit or below a root commit (no parents).
		isLast := i == len(versions)-1
		isRoot := len(v.Commit.Parents) == 0
		if isLast || isRoot {
			cells[i].connector = "  "
		} else {
			cells[i].connector = "│ "
		}
	}
	return cells
}

// graphWidth is the fixed display width of the graph prefix column.
const graphWidth = 2
