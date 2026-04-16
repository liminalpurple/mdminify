package minify

import "strings"

// squashTableRow takes a table row and returns a minified version with
// trimmed cell content and no padding.
func squashTableRow(line string) string {
	cells := splitTableCells(line)
	for i, cell := range cells {
		c := strings.TrimSpace(cell)
		if isSeparatorCell(c) {
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
