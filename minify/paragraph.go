package minify

import "strings"

// paragraphBuffer accumulates lines and flushes them as unwrapped paragraphs.
type paragraphBuffer struct {
	lines []string
}

// add appends a line to the buffer.
func (p *paragraphBuffer) add(line string) {
	p.lines = append(p.lines, line)
}

// flush joins soft-wrapped lines and returns the resulting lines.
// Lines that start a new block element (list, heading, blockquote) begin
// a new output line. Continuation lines are joined to their predecessor.
// Hard breaks are preserved.
func (p *paragraphBuffer) flush() []string {
	if len(p.lines) == 0 {
		return nil
	}

	var result []string
	var current strings.Builder

	// A link reference definition spans up to three lines — label, destination
	// and title — and none of them may be joined: "[0]:" over "0" is a
	// definition, while "[0]:" over "0 0" is an ordinary paragraph, because a
	// title has to be quoted. canJoin declines at the definition's own line,
	// but the join that matters comes after it, where both ends look like
	// plain text. The construct is not modelled, so once one may have opened,
	// the rest of the buffer is passed through rather than guessed at.
	//
	// A delimiter row is sticky for the same reason. A single-column table
	// needs no pipe at all, so "0" over "-:" over two body rows arrives here
	// as four plain lines; joining the body rows merges two table rows into
	// one, and the pair being joined shows nothing of the delimiter row that
	// made them rows. Once either construct may have opened, joining stops.
	sticky := false

	for i, line := range p.lines {
		// A line indented four or more columns may be indented code, where
		// trailing whitespace is content rather than noise. So may a list item
		// we decline to parse: a marker followed by a tab puts its content at
		// a column we are not tracking, which can be far enough past the
		// item's content offset to be code within it. Either way the line is
		// never joined (see canJoin), so it is emitted exactly as it arrived.
		verbatim := indentWidth(line) >= 4 || unparseableListItem(line)
		trimmed := trimTrailingWhitespace(line)
		if verbatim {
			trimmed = line
		}

		if isLinkRefDef(line) || isTableSeparator(line) {
			sticky = true
		}

		if i == 0 {
			current.WriteString(trimmed)
			continue
		}

		// A line with a delimiter row after it is a table header, so it must
		// start its own line. canJoin sees only the two lines being joined and
		// cannot look ahead, so the check belongs here.
		header := i+1 < len(p.lines) && isTableSeparator(p.lines[i+1])

		if header || sticky || !canJoin(p.lines[i-1], line) {
			// Not a warranted join: emit what we have and start again,
			// leaving this line exactly as it arrived.
			result = append(result, current.String())
			current.Reset()
			current.WriteString(trimmed)
			continue
		}

		// Two lines that are each ordinary text can still form a block
		// construct once joined: "**" and "**" become "** **", a thematic
		// break. Check the result, not only the parts.
		joined := current.String() + " " + strings.TrimLeft(trimmed, " ")
		if isThematicBreak(joined) || isSetextUnderline(joined) {
			result = append(result, current.String())
			current.Reset()
			current.WriteString(trimmed)
			continue
		}

		// Continuation — join with space.
		current.Reset()
		current.WriteString(joined)
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	// A hard break on the final line is stripped by renderers anyway, but the
	// buffer is flushed whenever a paragraph might be ending, not only when it
	// is: a line that looks like a table row flushes it, and goes back into
	// the same paragraph if no delimiter row follows. Trimming here would then
	// remove a break that had content after it, so the line is left alone and
	// the two spaces are kept.

	p.lines = p.lines[:0]
	return result
}

// canJoin reports whether line may be joined onto prev as a soft-wrapped
// paragraph continuation.
//
// Joining is the only operation in this package that can change a document's
// meaning. Leaving two lines separate always renders identically, because a
// soft line break and a space are both just whitespace to a renderer, so the
// safe default is to leave a line alone. Every join therefore needs a positive
// warrant, and anything uncertain is passed through untouched — an unmodelled
// construct then costs compression rather than correctness.
//
// Add a new reason to decline here rather than a new reason to join, and test
// each signal against both lines: a join has two ends, and either of them can
// be the construct we failed to recognise.
func canJoin(prev, line string) bool {
	// Constructs we model, where line opens a block of its own.
	if startsNewBlock(line) || isSetextUnderline(line) {
		return false
	}

	// The same signals applied to the other end of the join. A list item is
	// the one construct that can take a continuation, and then only when its
	// own content can: an item holding a heading, or holding nothing at all,
	// has no open paragraph for the next line to continue. Asking the same
	// question of the item's content terminates, because the content is
	// always shorter than the line it came from.
	if startsNewBlock(prev) || isSetextUnderline(prev) {
		content, ok := listItemContent(prev)
		if !ok || content == "" || !canJoin(content, line) {
			return false
		}
	}

	// A hard break ends its line deliberately.
	if hasHardBreak(prev) {
		return false
	}

	// Four or more columns of indentation may open an indented code block, or
	// mark a nested list item whose marker cannot be recognised without
	// tracking container offsets. Neither is modelled yet, so decline.
	if indentWidth(prev) >= 4 || indentWidth(line) >= 4 {
		return false
	}

	// A pipe on either line may make them the header and delimiter rows of a
	// GFM table, which does not require a leading pipe and so is not caught
	// upstream by isTableRow.
	if strings.ContainsRune(line, '|') || strings.ContainsRune(prev, '|') {
		return false
	}

	// A single-column table needs no pipe at all: "0" over "-:" is a table
	// with one right-aligned column. Either end matters — a delimiter row
	// before the line means the line is a body row of that table.
	if isTableSeparator(line) || isTableSeparator(prev) {
		return false
	}

	// An HTML tag may span a line break. Joining would change what the tag
	// contains, and can complete one that was left open.
	if hasUnclosedTag(prev) {
		return false
	}

	return true
}

// unparseableListItem reports whether the line opens a list item that
// parseListItemPrefix declines to take over, which today means a marker
// followed by a tab rather than a space.
func unparseableListItem(line string) bool {
	if !isListMarker(line) {
		return false
	}
	_, ok := parseListItemPrefix(line)
	return !ok
}
