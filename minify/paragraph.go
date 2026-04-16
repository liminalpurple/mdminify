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
		trimmed := trimTrailingWhitespace(line)

		if i == 0 {
			current.WriteString(trimmed)
			continue
		}

		// Does this line start a new logical block?
		if startsNewBlock(line) || isSetextUnderline(line) {
			// Flush current accumulator.
			result = append(result, current.String())
			current.Reset()
			current.WriteString(trimmed)
			continue
		}

		// Check if previous line had a hard break.
		prev := p.lines[i-1]
		if hasHardBreak(prev) {
			result = append(result, current.String())
			current.Reset()
			current.WriteString(trimmed)
			continue
		}

		// Continuation — join with space.
		current.WriteByte(' ')
		current.WriteString(strings.TrimLeft(trimmed, " "))
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	// A hard break on the very last line is meaningless (nothing follows),
	// so strip trailing whitespace from the final output line.
	if len(result) > 0 {
		last := result[len(result)-1]
		result[len(result)-1] = strings.TrimRight(last, " \t")
	}

	p.lines = p.lines[:0]
	return result
}
