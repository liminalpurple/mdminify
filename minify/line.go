package minify

import (
	"strings"
	"unicode"
)

// isBlank returns true if the line is empty or contains only whitespace.
func isBlank(line string) bool {
	return strings.TrimSpace(line) == ""
}

// fenceInfo holds information about a fenced code block opening.
type fenceInfo struct {
	char   byte // '`' or '~'
	count  int  // number of fence characters
	indent int  // leading spaces (0-3)
}

// isOpeningFence checks whether a line opens a fenced code block.
// Returns the fence info and true if it does.
func isOpeningFence(line string) (fenceInfo, bool) {
	indent := countLeadingSpaces(line)
	if indent > 3 {
		return fenceInfo{}, false
	}
	rest := line[indent:]
	if len(rest) == 0 {
		return fenceInfo{}, false
	}
	ch := rest[0]
	if ch != '`' && ch != '~' {
		return fenceInfo{}, false
	}
	count := 0
	for i := 0; i < len(rest) && rest[i] == ch; i++ {
		count++
	}
	if count < 3 {
		return fenceInfo{}, false
	}
	// Backtick fences cannot have backticks in the info string.
	if ch == '`' && strings.ContainsRune(rest[count:], '`') {
		return fenceInfo{}, false
	}
	return fenceInfo{char: ch, count: count, indent: indent}, true
}

// isClosingFence checks whether a line closes a fenced code block opened with fi.
func isClosingFence(line string, fi fenceInfo) bool {
	indent := countLeadingSpaces(line)
	if indent > 3 {
		return false
	}
	rest := line[indent:]
	if len(rest) == 0 {
		return false
	}
	if rest[0] != fi.char {
		return false
	}
	count := 0
	for i := 0; i < len(rest) && rest[i] == fi.char; i++ {
		count++
	}
	if count < fi.count {
		return false
	}
	// Closing fence may only have trailing spaces.
	return strings.TrimSpace(rest[count:]) == ""
}

// isATXHeading returns true if the line is an ATX heading (# ... ######).
func isATXHeading(line string) bool {
	s := strings.TrimLeft(line, " ")
	if len(s) == 0 || s[0] != '#' {
		return false
	}
	i := 0
	for i < len(s) && s[i] == '#' {
		i++
	}
	if i > 6 {
		return false
	}
	// Must be followed by space or end of line.
	return i == len(s) || s[i] == ' ' || s[i] == '\t'
}

// isSetextUnderline returns true if the line is a setext heading underline
// (one or more = or - characters, with optional leading/trailing spaces).
func isSetextUnderline(line string) bool {
	s := strings.TrimSpace(line)
	if len(s) == 0 {
		return false
	}
	ch := s[0]
	if ch != '=' && ch != '-' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] != ch {
			return false
		}
	}
	return true
}

// isThematicBreak returns true if the line is a thematic break (---, ***, ___).
func isThematicBreak(line string) bool {
	s := strings.TrimSpace(line)
	if len(s) < 3 {
		return false
	}
	ch := s[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	count := 0
	for _, r := range s {
		if r == rune(ch) {
			count++
		} else if r != ' ' && r != '\t' {
			return false
		}
	}
	return count >= 3
}

// isTableRow returns true if the line looks like a table row (starts and ends
// with |, or contains at least one | that isn't escaped).
func isTableRow(line string) bool {
	s := strings.TrimSpace(line)
	if len(s) == 0 {
		return false
	}
	// GFM tables require leading |.
	return s[0] == '|'
}

// isTableSeparator returns true if the line is a table separator row,
// e.g. |---|:---:|---:|
func isTableSeparator(line string) bool {
	cells := splitTableCells(line)
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		c := strings.TrimSpace(cell)
		if !isSeparatorCell(c) {
			return false
		}
	}
	return true
}

// isSeparatorCell returns true if the cell content matches a table separator
// cell pattern like ---, :---, ---:, or :---:.
func isSeparatorCell(s string) bool {
	if len(s) == 0 {
		return false
	}
	i := 0
	if s[i] == ':' {
		i++
	}
	dashes := 0
	for i < len(s) && s[i] == '-' {
		dashes++
		i++
	}
	if dashes == 0 {
		return false
	}
	if i < len(s) && s[i] == ':' {
		i++
	}
	return i == len(s)
}

// splitTableCells splits a table row into cells, respecting escaped pipes.
func splitTableCells(line string) []string {
	s := strings.TrimSpace(line)
	// Strip leading and trailing |
	if len(s) > 0 && s[0] == '|' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '|' && (len(s) < 2 || s[len(s)-2] != '\\') {
		s = s[:len(s)-1]
	}
	if len(s) == 0 {
		return nil
	}

	var cells []string
	var current strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '|' {
			current.WriteByte('\\')
			current.WriteByte('|')
			i++ // skip the |
		} else if s[i] == '|' {
			cells = append(cells, current.String())
			current.Reset()
		} else {
			current.WriteByte(s[i])
		}
	}
	cells = append(cells, current.String())
	return cells
}

// isListMarker returns true if the line starts with a list marker (-, *, +, or
// ordered like 1. / 1)). Allows up to 3 leading spaces.
func isListMarker(line string) bool {
	s := line
	spaces := 0
	for len(s) > 0 && s[0] == ' ' && spaces < 4 {
		s = s[1:]
		spaces++
	}
	if spaces > 3 || len(s) == 0 {
		return false
	}

	// Unordered: -, *, + followed by space.
	if (s[0] == '-' || s[0] == '*' || s[0] == '+') && len(s) > 1 && s[1] == ' ' {
		return true
	}

	// Ordered: digits followed by . or ) then space.
	i := 0
	for i < len(s) && i < 9 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(s) {
		return false
	}
	if (s[i] == '.' || s[i] == ')') && i+1 < len(s) && s[i+1] == ' ' {
		return true
	}
	return false
}

// isEmptyListItem returns true if the line is a list marker with no content
// after it, e.g. "-", "- ", "1." or "0)". Such an item contains no paragraph,
// so a following line cannot be a lazy continuation of it.
func isEmptyListItem(line string) bool {
	s := strings.TrimRight(line, " \t")
	indent := countLeadingSpaces(s)
	if indent > 3 {
		return false
	}
	s = s[indent:]
	if len(s) == 0 {
		return false
	}

	// Unordered: a bare -, * or +.
	if s[0] == '-' || s[0] == '*' || s[0] == '+' {
		return len(s) == 1
	}

	// Ordered: digits followed by . or ) and nothing else.
	i := 0
	for i < len(s) && i < 9 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i != len(s)-1 {
		return false
	}
	return s[i] == '.' || s[i] == ')'
}

// listItemContent returns what follows a list item's marker, and whether line
// is a list item with content at all. An empty item, or one whose content is
// indented far enough to be code, reports false.
func listItemContent(line string) (string, bool) {
	indent := countLeadingSpaces(line)
	if indent > 3 {
		return "", false
	}
	s := line[indent:]
	if len(s) == 0 {
		return "", false
	}

	if s[0] == '-' || s[0] == '*' || s[0] == '+' {
		s = s[1:]
	} else {
		i := 0
		for i < len(s) && i < 9 && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 || i >= len(s) || (s[i] != '.' && s[i] != ')') {
			return "", false
		}
		s = s[i+1:]
	}

	spaces := countLeadingSpaces(s)
	if spaces == 0 || spaces >= 5 {
		// No separating whitespace means no content; five or more means the
		// content is indented code within the item.
		return "", false
	}
	return s[spaces:], true
}

// isBlockquotePrefix returns true if the line starts with a blockquote marker.
//
// The indent limit matches stripBlockquotePrefix deliberately. Four or more
// leading spaces make the line indented code, not a quote, and a predicate
// that accepted what the stripper cannot remove would leave processBlockquote
// recursing on unchanged input.
func isBlockquotePrefix(line string) bool {
	indent := countLeadingSpaces(line)
	if indent > 3 {
		return false
	}
	s := line[indent:]
	return len(s) > 0 && s[0] == '>'
}

// stripBlockquotePrefix removes one level of blockquote prefix from a line.
// Returns the stripped line and true if a prefix was found.
func stripBlockquotePrefix(line string) (string, bool) {
	indent := countLeadingSpaces(line)
	if indent > 3 {
		return line, false
	}
	rest := line[indent:]
	if len(rest) == 0 || rest[0] != '>' {
		return line, false
	}
	rest = rest[1:]
	// Consume one optional space after >.
	if len(rest) > 0 && rest[0] == ' ' {
		rest = rest[1:]
	}
	return rest, true
}

// hasHardBreak returns true if the line ends with a hard line break
// (two or more trailing spaces, or a trailing backslash).
func hasHardBreak(line string) bool {
	if len(line) == 0 {
		return false
	}
	if line[len(line)-1] == '\\' {
		return true
	}
	return len(line) >= 2 && line[len(line)-1] == ' ' && line[len(line)-2] == ' '
}

// isHTMLBlockStart returns true if the line starts an HTML block (type 1-7).
// We use a simplified check: line starts with < followed by a known block tag
// or starts with <!-- or <? or <! (uppercase).
func isHTMLBlockStart(line string) bool {
	s := strings.TrimLeft(line, " ")
	if len(s) == 0 || s[0] != '<' {
		return false
	}
	// HTML comments and processing instructions.
	if strings.HasPrefix(s, "<!--") || strings.HasPrefix(s, "<?") || strings.HasPrefix(s, "<!") {
		return true
	}
	// Block-level HTML tags.
	lower := strings.ToLower(s)
	tags := []string{
		"<address", "<article", "<aside", "<base", "<basefont", "<blockquote",
		"<body", "<caption", "<center", "<col", "<colgroup", "<dd", "<details",
		"<dialog", "<dir", "<div", "<dl", "<dt", "<fieldset", "<figcaption",
		"<figure", "<footer", "<form", "<frame", "<frameset", "<h1", "<h2",
		"<h3", "<h4", "<h5", "<h6", "<head", "<header", "<hr", "<html",
		"<iframe", "<legend", "<li", "<link", "<main", "<menu", "<menuitem",
		"<nav", "<noframes", "<ol", "<optgroup", "<option", "<p", "<param",
		"<pre", "<script", "<section", "<select", "<source", "<style",
		"<summary", "<table", "<tbody", "<td", "<template", "<textarea",
		"<tfoot", "<th", "<thead", "<title", "<tr", "<track", "<ul",
	}
	for _, tag := range tags {
		if strings.HasPrefix(lower, tag) {
			// Must be followed by space, >, />, or end of line.
			rest := lower[len(tag):]
			if len(rest) == 0 || rest[0] == ' ' || rest[0] == '>' || rest[0] == '/' || rest[0] == '\t' {
				return true
			}
		}
	}
	// Also match closing tags.
	if len(lower) > 2 && lower[1] == '/' {
		for _, tag := range tags {
			closeTag := "</" + tag[1:]
			if strings.HasPrefix(lower, closeTag) {
				rest := lower[len(closeTag):]
				if len(rest) == 0 || rest[0] == '>' || rest[0] == ' ' || rest[0] == '\t' {
					return true
				}
			}
		}
	}
	return false
}

// isLinkRefDef returns true if the line looks like a link reference definition:
// [label]: url "title"
func isLinkRefDef(line string) bool {
	s := strings.TrimLeft(line, " ")
	if len(s) == 0 || s[0] != '[' {
		return false
	}
	// Find closing ]
	idx := strings.Index(s, "]:")
	return idx > 0
}

// isLoneHTMLTag reports whether the line is a single complete HTML tag and
// nothing else. Such a line opens a CommonMark type 7 HTML block, which
// isHTMLBlockStart does not detect because it only knows the tag names used by
// block types 1 to 6.
//
// Type 7 cannot interrupt a paragraph, so treating it as a block start
// everywhere is more conservative than the specification requires. That costs
// a join inside a paragraph and never costs correctness.
func isLoneHTMLTag(line string) bool {
	s := strings.TrimSpace(line)
	if len(s) < 3 || s[0] != '<' || s[len(s)-1] != '>' {
		return false
	}
	// Anything after the first tag means this is not a lone tag.
	return strings.IndexByte(s, '>') == len(s)-1
}

// startsNewBlock returns true if the line starts a new block-level element
// that should not be joined to a preceding paragraph line.
func startsNewBlock(line string) bool {
	if _, ok := isOpeningFence(line); ok {
		return true
	}
	return isATXHeading(line) ||
		isThematicBreak(line) ||
		isHTMLBlockStart(line) ||
		isLoneHTMLTag(line) ||
		isListMarker(line) ||
		isEmptyListItem(line) ||
		isBlockquotePrefix(line) ||
		isLinkRefDef(line)
}

// isBoldAsHeading returns true if the line is a single-line paragraph consisting
// entirely of bold text (e.g. "**Section title:**" or "__Label__"). The bold
// markers may be followed by a trailing colon or similar punctuation.
func isBoldAsHeading(line string) bool {
	s := strings.TrimSpace(line)
	if len(s) < 5 { // minimum: **x**
		return false
	}
	// Check ** ... ** wrapping.
	if strings.HasPrefix(s, "**") && strings.HasSuffix(s, "**") {
		inner := s[2 : len(s)-2]
		return len(inner) > 0 && !strings.Contains(inner, "**")
	}
	// Check __ ... __ wrapping.
	if strings.HasPrefix(s, "__") && strings.HasSuffix(s, "__") {
		inner := s[2 : len(s)-2]
		return len(inner) > 0 && !strings.Contains(inner, "__")
	}
	// Also match **text**: or **text**; etc (punctuation after closing marker).
	for _, marker := range []string{"**", "__"} {
		if !strings.HasPrefix(s, marker) {
			continue
		}
		// Find the closing marker.
		closeIdx := strings.LastIndex(s[2:], marker)
		if closeIdx < 0 {
			continue
		}
		closeIdx += 2 // offset from the start
		// Everything after the closing marker should be punctuation only.
		tail := s[closeIdx+2:]
		if len(tail) > 0 && isTrailingPunct(tail) {
			return true
		}
	}
	return false
}

// isTrailingPunct returns true if s consists entirely of common trailing
// punctuation characters (colon, semicolon, period, exclamation, question).
func isTrailingPunct(s string) bool {
	for _, r := range s {
		switch r {
		case ':', ';', '.', '!', '?':
			continue
		default:
			return false
		}
	}
	return true
}

// countLeadingSpaces returns the number of leading space characters.
func countLeadingSpaces(line string) int {
	n := 0
	for _, r := range line {
		if r == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

// indentWidth returns the visual indentation of the line in columns, expanding
// tabs to four-column tab stops as CommonMark does. Four columns is the
// threshold at which a line becomes indented code, so a leading tab counts as
// much as four spaces.
func indentWidth(line string) int {
	w := 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case ' ':
			w++
		case '\t':
			w += 4 - w%4
		default:
			return w
		}
	}
	return w
}

// trimTrailingWhitespace removes trailing spaces/tabs from a line, preserving
// hard break markers (two trailing spaces or trailing backslash).
func trimTrailingWhitespace(line string) string {
	if hasHardBreak(line) {
		// Whitespace before a trailing backslash is content when the backslash
		// is literal, which is the case when nothing follows the line, and is
		// insignificant when it marks a hard break. One line gives no way to
		// tell, so leave it alone.
		if line[len(line)-1] == '\\' {
			return line
		}
		// Preserve the break marker but normalise to exactly two spaces.
		trimmed := strings.TrimRight(line, " \t")
		return trimmed + "  "
	}
	return strings.TrimRightFunc(line, unicode.IsSpace)
}
