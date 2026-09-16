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

	for i, line := range p.lines {
		// A line indented four or more columns may be indented code, where
		// trailing whitespace is content rather than noise. Such a line is
		// never joined (see canJoin), so it is emitted exactly as it arrived.
		verbatim := indentWidth(line) >= 4
		trimmed := trimTrailingWhitespace(line)
		if verbatim {
			trimmed = line
		}

		if i == 0 {
			current.WriteString(trimmed)
			continue
		}

		if !canJoin(p.lines[i-1], line) {
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

	// A hard break on the very last line is meaningless (nothing follows), so
	// strip trailing whitespace from the final output line — unless it is
	// indented enough to be code, where that whitespace is content.
	if len(result) > 0 {
		last := result[len(result)-1]
		if indentWidth(last) < 4 {
			result[len(result)-1] = strings.TrimRight(last, " \t")
		}
	}

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

	return true
}
