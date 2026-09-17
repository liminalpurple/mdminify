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
		switch {
		case separator && isSeparatorCell(c):
			cells[i] = minimalSeparator(c)
		case c == "" && cell != "":
			// A cell holding only whitespace is not the same as one holding
			// nothing: "|  |" over "|-:|" renders right-aligned, while "||"
			// over the same delimiter row loses the alignment. One space is
			// kept so the cell stays non-empty, and a cell that arrived truly
			// empty is left that way rather than gaining a space, which would
			// change the rendering in the other direction.
			cells[i] = " "
		default:
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
