package minify

import (
	"bytes"
	"strings"
)

// listItemPrefix describes the marker that opens a list item.
type listItemPrefix struct {
	indent int    // leading spaces before the marker (0-3)
	marker string // the marker itself, e.g. "-" or "12."
	pad    int    // spaces between the marker and the item's content
}

// contentOffset returns the column at which the item's content begins.
// Continuation and nested content belong to the item when indented at least
// this far, which is what makes indentation meaningful inside a list.
func (p listItemPrefix) contentOffset() int {
	return p.indent + len(p.marker) + p.pad
}

// parseListItemPrefix reads the marker opening a list item, if there is one.
//
// Per CommonMark the content offset includes the spaces after the marker, but
// only up to four: five or more means the marker is followed by a single space
// and the rest is indented code within the item. An item with no content at
// all takes an offset of marker plus one.
func parseListItemPrefix(line string) (listItemPrefix, bool) {
	indent := countLeadingSpaces(line)
	if indent > 3 {
		return listItemPrefix{}, false
	}
	s := line[indent:]
	if len(s) == 0 {
		return listItemPrefix{}, false
	}

	var marker string
	switch s[0] {
	case '-', '*', '+':
		marker = s[:1]
	default:
		i := 0
		for i < len(s) && i < 9 && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 || i >= len(s) || (s[i] != '.' && s[i] != ')') {
			return listItemPrefix{}, false
		}
		marker = s[:i+1]
	}

	rest := s[len(marker):]
	if rest != "" && rest[0] != ' ' {
		// A marker must be followed by whitespace to open an item. A tab is
		// valid there, but its width depends on the column it lands in, so
		// such items are left alone rather than taken over: isListMarker still
		// recognises them, which is enough for them to pass through untouched.
		return listItemPrefix{}, false
	}

	pad := countLeadingSpaces(rest)
	switch {
	case strings.TrimSpace(rest) == "":
		pad = 1 // empty item
	case pad > 4:
		pad = 1 // the remainder is indented code within the item
	}
	return listItemPrefix{indent: indent, marker: marker, pad: pad}, true
}

// listItemHoldsIndentedCode reports whether the line is a list item whose
// content begins five or more columns after the marker, which makes that
// content an indented code block within the item rather than a paragraph.
// Trailing whitespace inside code is content, so such a line must be passed
// through exactly as it arrived.
func listItemHoldsIndentedCode(line string) bool {
	p, ok := parseListItemPrefix(line)
	if !ok {
		return false
	}
	rest := line[p.indent+len(p.marker):]
	return strings.TrimSpace(rest) != "" && countLeadingSpaces(rest) > 4
}

// interruptsParagraph reports whether an item opening with this prefix may
// begin a list in the middle of a paragraph. CommonMark allows it only when
// the item has content and, if ordered, is numbered 1 — otherwise an ordinary
// line beginning "2024. " would silently become a list.
func (p listItemPrefix) interruptsParagraph(line string) bool {
	if strings.TrimSpace(line[min(len(line), p.contentOffset()):]) == "" {
		return false
	}
	last := p.marker[len(p.marker)-1]
	if last == '.' || last == ')' {
		return p.marker[:len(p.marker)-1] == "1"
	}
	return true
}

// canInterruptParagraph reports whether the line opens a block that may begin
// in the middle of a paragraph. It decides whether the blank line after a
// bold-as-heading paragraph can be removed: a block that cannot interrupt a
// paragraph would instead be absorbed into it.
func canInterruptParagraph(line string) bool {
	if isBlockquotePrefix(line) {
		return true
	}
	if p, ok := parseListItemPrefix(line); ok {
		return p.interruptsParagraph(line)
	}
	return isTableRow(line)
}

// processListItem minifies one list item. The item's own lines are dedented to
// the content offset, minified recursively so that every predicate sees them
// at relative indent zero, then re-indented and reunited with the marker.
//
// This mirrors processBlockquote: containers are handled by stripping their
// prefix, recursing, and putting the prefix back.
func processListItem(lines []string, depth int) ([]string, bool, error) {
	prefix, ok := parseListItemPrefix(lines[0])
	if !ok {
		return lines, false, nil
	}
	offset := prefix.contentOffset()

	// A tab's width depends on the column it lands in, so dedenting an item's
	// lines to the content offset and re-indenting them moves any tab in that
	// indentation to a different column, changing how much indentation it
	// represents. The absolute columns are not tracked inside a container, so
	// an item indented with tabs is passed through untouched — the same
	// treatment processBlockquote gives a tab in its own prefix.
	if hasTabInPrefix(lines[0][prefix.indent+len(prefix.marker):]) {
		return lines, false, nil
	}
	for _, line := range lines[1:] {
		if hasTabInPrefix(line) {
			return lines, false, nil
		}
	}

	inner := make([]string, 0, len(lines))
	first := lines[0]
	if len(first) > offset {
		inner = append(inner, first[offset:])
	} else {
		inner = append(inner, "")
	}
	for _, line := range lines[1:] {
		inner = append(inner, dedent(line, offset))
	}

	// A blank line ending an item normally falls outside it and marks the list
	// as loose. Inside an unclosed fence it is code content instead, so the
	// item keeps it and the list is not loose on its account.
	looseBlank := false
	if !endsInOpenFence(inner) {
		end := len(inner)
		for end > 0 && strings.TrimSpace(inner[end-1]) == "" {
			end--
		}
		looseBlank = end < len(inner) && end > 0
		inner = inner[:end]
	}
	if len(inner) == 0 {
		return lines, false, nil
	}

	var buf bytes.Buffer
	if err := minifyDepth(strings.NewReader(strings.Join(inner, "\n")+"\n"), &buf, depth+1, true); err != nil {
		return nil, false, err
	}
	// Minify emits exactly one trailing newline, so only that one is removed.
	// Trimming every trailing newline would discard a final blank line, which
	// is content when the item ends inside an open fence.
	outLines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")

	// Leading blank lines inside an item are not content. One is just the
	// empty remainder of the marker line; two or more mean a real blank line
	// separated the marker from the content, which makes the list loose, so
	// the content is kept on its own line rather than pulled up to the marker.
	lead := 0
	for lead < len(inner) && strings.TrimSpace(inner[lead]) == "" {
		lead++
	}

	pad := strings.Repeat(" ", offset)
	if lead > 0 && lead < len(inner) {
		// The marker line carries no content, so the content stays where it
		// is. Pulling it up would change the structure when it is itself a
		// list marker: "*" then an indented "+" is two sibling lists, while
		// "* +" is one nested inside the other. Two or more blank lines mean a
		// real blank separated marker from content, which makes the list
		// loose, so one is kept.
		result := []string{strings.Repeat(" ", prefix.indent) + prefix.marker}
		if lead >= 2 {
			result = append(result, "")
		}
		for _, ol := range outLines {
			if ol == "" {
				result = append(result, "")
			} else {
				result = append(result, pad+ol)
			}
		}
		return result, looseBlank, nil
	}

	result := make([]string, 0, len(outLines))
	for i, ol := range outLines {
		switch {
		case i == 0 && strings.TrimSpace(ol) == "":
			// An item whose content is empty would otherwise leave a marker
			// followed by its padding, so the padding is dropped. Only this
			// case may be trimmed: trailing whitespace on a line that does
			// have content is a hard break, and the line after it — which may
			// lazily continue this item's paragraph from outside the buffer —
			// then depends on it.
			result = append(result, strings.Repeat(" ", prefix.indent)+prefix.marker)
		case i == 0:
			result = append(result, strings.Repeat(" ", prefix.indent)+prefix.marker+
				strings.Repeat(" ", prefix.pad)+ol)
		case ol == "":
			result = append(result, "")
		default:
			result = append(result, pad+ol)
		}
	}
	return result, looseBlank, nil
}

// endsInOpenFence reports whether the lines finish inside a fenced code block
// that was never closed.
func endsInOpenFence(lines []string) bool {
	var fence fenceInfo
	open := false
	for _, l := range lines {
		if open {
			if isClosingFence(l, fence) {
				open = false
			}
			continue
		}
		if f, ok := isOpeningFence(l); ok {
			fence, open = f, true
		}
	}
	return open
}

// dedent removes up to n columns of leading whitespace, expanding tabs the way
// indentWidth measures them so the two agree.
func dedent(line string, n int) string {
	w := 0
	i := 0
	for i < len(line) && w < n {
		switch line[i] {
		case ' ':
			w++
			i++
		case '\t':
			w += 4 - w%4
			i++
		default:
			return line[i:]
		}
	}
	if w < n {
		// The line held nothing but whitespace, and less of it than the
		// offset: it is a blank line within the item.
		return ""
	}
	// A tab may have carried past the offset; restore the overshoot.
	return strings.Repeat(" ", w-n) + line[i:]
}
