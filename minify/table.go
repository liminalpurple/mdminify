package minify

import "strings"

// squashTableRow takes a table row and returns a minified version with trimmed
// cell content and no padding.
//
// separator selects the delimiter row, whose cells are reduced to the shortest
// form that preserves alignment. Only that row may be reduced: in a header or
// body row a cell reading "---" or ":-:" is ordinary text, and rewriting it
// would change what the table says.
func squashTableRow(line string, separator bool) string {
	cells := splitTableCells(line)
	for i, cell := range cells {
		c := strings.TrimSpace(cell)
		if separator && isSeparatorCell(c) {
			cells[i] = minimalSeparator(c)
		} else {
			cells[i] = c
		}
	}
	return "|" + strings.Join(cells, "|") + "|"
}

// minimalSeparator returns the shortest valid separator cell that preserves
// alignment. E.g. ":------:" becomes ":-:", "---" becomes "-".
func minimalSeparator(s string) string {
	left := s[0] == ':'
	right := s[len(s)-1] == ':'

	switch {
	case left && right:
		return ":-:"
	case left:
		return ":-"
	case right:
		return "-:"
	default:
		return "-"
	}
}
