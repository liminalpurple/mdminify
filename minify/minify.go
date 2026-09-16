package minify

import (
	"bufio"
	"io"
	"strings"
)

// Minify reads markdown from r, applies minification transforms, and writes
// the result to w. The rendered meaning of the markdown is preserved — the
// output should produce identical HTML when passed through a CommonMark or
// GFM renderer.
//
// The output always ends with a single newline. CRLF line endings in the
// input are normalised to LF.
//
// Minify processes the input as a stream and does not buffer the entire
// document in memory, though blockquote content is buffered per-block for
// recursive processing.
func Minify(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	m := &minifier{w: w}
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// Normalise CRLF.
		line = strings.TrimRight(line, "\r")

		if err := m.processLine(line, lineNum); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return m.finish()
}

type state int

const (
	stateNormal state = iota
	stateFrontMatter
	stateFencedCode
)

type minifier struct {
	w     io.Writer
	state state

	fence fenceInfo // current fenced code block info
	para  paragraphBuffer

	// Table handling: we buffer potential table rows until we can confirm
	// we're actually in a table (by seeing a separator row).
	tableBuf     []string // buffered rows before separator confirmed
	inTable      bool     // true once separator has been seen
	tableStarted bool     // true if we've started buffering table rows

	// Blockquote handling: accumulate consecutive blockquote lines.
	bqBuf []string

	// Track consecutive blank lines.
	lastWasBlank bool

	// A blank line is pending — deferred so we can suppress it if the next
	// content is a block element following a heading-like line.
	pendingBlank bool

	// The last emitted block was heading-like (ATX heading, setext heading,
	// or bold-as-heading paragraph).
	lastWasHeadingLike bool

	// Whether we've emitted any output yet.
	started bool
}

func (m *minifier) processLine(line string, lineNum int) error {
	// --- FRONT MATTER (only at document start) ---
	if lineNum == 1 && line == "---" {
		m.state = stateFrontMatter
		return m.emit(line)
	}
	if m.state == stateFrontMatter {
		if line == "---" || line == "..." {
			m.state = stateNormal
		}
		return m.emit(line)
	}

	// --- FENCED CODE BLOCK ---
	if m.state == stateFencedCode {
		if isClosingFence(line, m.fence) {
			m.state = stateNormal
		}
		return m.emit(line)
	}
	if fi, ok := isOpeningFence(line); ok {
		if err := m.flushAll(); err != nil {
			return err
		}
		m.fence = fi
		m.state = stateFencedCode
		return m.emit(line)
	}

	// --- BLOCKQUOTE ---
	if isBlockquotePrefix(line) {
		// If we were accumulating non-blockquote content, flush it first.
		if len(m.bqBuf) == 0 {
			if err := m.flushTable(); err != nil {
				return err
			}
			if err := m.flushParagraph(); err != nil {
				return err
			}
		}
		m.bqBuf = append(m.bqBuf, line)
		return nil
	}
	// A non-blockquote, non-blank line after blockquote lines could be
	// lazy continuation. But for safety, flush the blockquote first.
	if len(m.bqBuf) > 0 {
		if isBlank(line) {
			if err := m.flushBlockquote(); err != nil {
				return err
			}
			return m.emitBlank()
		}
		// Non-blank, non-blockquote line — could be lazy continuation
		// if the last bq line was paragraph text. For simplicity, flush bq.
		if err := m.flushBlockquote(); err != nil {
			return err
		}
		// Fall through to process this line normally.
	}

	// --- TABLE ---
	if isTableRow(line) {
		if m.inTable {
			// Already confirmed in a table.
			return m.emitBlock(squashTableRow(line))
		}
		if m.tableStarted {
			// We have a buffered header row. Is this the separator? GFM
			// requires the delimiter row to have the same number of cells as
			// the header; if it does not, this is not a table at all.
			if isTableSeparator(line) && len(m.tableBuf) > 0 &&
				len(splitTableCells(line)) == len(splitTableCells(m.tableBuf[0])) {
				m.inTable = true
				// Flush the paragraph buffer first (it shouldn't have anything
				// but just in case).
				if err := m.flushParagraph(); err != nil {
					return err
				}
				// Emit the buffered header row(s).
				for i, r := range m.tableBuf {
					if i == 0 {
						if err := m.emitBlock(squashTableRow(r)); err != nil {
							return err
						}
					} else {
						if err := m.emit(squashTableRow(r)); err != nil {
							return err
						}
					}
				}
				m.tableBuf = nil
				return m.emit(squashTableRow(line))
			}
			// Not a separator — these were just lines starting with |.
			// Move them into the paragraph buffer.
			for _, r := range m.tableBuf {
				m.para.add(r)
			}
			m.tableBuf = nil
			m.tableStarted = false
			m.para.add(line)
			return nil
		}
		// Start buffering a potential table.
		if err := m.flushParagraph(); err != nil {
			return err
		}
		m.tableStarted = true
		m.tableBuf = append(m.tableBuf, line)
		return nil
	}
	// Non-table-row line — flush any table state.
	if err := m.flushTable(); err != nil {
		return err
	}

	// --- BLANK LINE ---
	if isBlank(line) {
		if err := m.flushParagraph(); err != nil {
			return err
		}
		return m.emitBlank()
	}

	// --- HTML BLOCK ---
	if isHTMLBlockStart(line) {
		if err := m.flushParagraph(); err != nil {
			return err
		}
		return m.emit(trimTrailingWhitespace(line))
	}

	// --- PARAGRAPH ACCUMULATION ---
	m.para.add(line)
	return nil
}

func (m *minifier) finish() error {
	if err := m.flushAll(); err != nil {
		return err
	}
	// Emit a final newline — conventional for text files.
	if m.started {
		_, err := io.WriteString(m.w, "\n")
		return err
	}
	return nil
}

func (m *minifier) flushAll() error {
	if err := m.flushBlockquote(); err != nil {
		return err
	}
	if err := m.flushTable(); err != nil {
		return err
	}
	return m.flushParagraph()
}

func (m *minifier) flushParagraph() error {
	lines := m.para.flush()
	for i, l := range lines {
		// First line of a list should use emitBlock so blank-line
		// suppression after headings works.
		if i == 0 && isListMarker(l) {
			if err := m.emitBlock(l); err != nil {
				return err
			}
		} else {
			if err := m.emit(l); err != nil {
				return err
			}
		}
		// Check if this emitted line makes the last block heading-like. A
		// setext underline only counts when something precedes it in the same
		// run: a lone "-" opens an empty list item instead, and treating it as
		// a heading would suppress a blank line that makes a list loose.
		if isATXHeading(l) || (i > 0 && isSetextUnderline(l)) || isBoldAsHeading(l) {
			m.lastWasHeadingLike = true
		}
	}
	return nil
}

func (m *minifier) flushTable() error {
	if m.inTable {
		m.inTable = false
		m.tableStarted = false
		return nil
	}
	if m.tableStarted {
		// Buffered rows that never got a separator — treat as paragraph lines.
		for _, r := range m.tableBuf {
			m.para.add(r)
		}
		m.tableBuf = nil
		m.tableStarted = false
	}
	return nil
}

func (m *minifier) flushBlockquote() error {
	if len(m.bqBuf) == 0 {
		return nil
	}
	lines, err := processBlockquote(m.bqBuf)
	m.bqBuf = nil
	if err != nil {
		return err
	}
	for i, l := range lines {
		if i == 0 {
			if err := m.emitBlock(l); err != nil {
				return err
			}
		} else {
			if err := m.emit(l); err != nil {
				return err
			}
		}
	}
	return nil
}

// emitContent writes a content line, flushing any pending blank line first.
// The followsHeading parameter controls whether a pending blank line is
// suppressed (true = this is a list/blockquote/table following a heading).
func (m *minifier) emitContent(line string, followsHeading bool) error {
	// Flush pending blank unless suppressed.
	if m.pendingBlank {
		m.pendingBlank = false
		if !m.lastWasHeadingLike || !followsHeading {
			if _, err := io.WriteString(m.w, "\n"); err != nil {
				return err
			}
		}
	}
	if m.started {
		if _, err := io.WriteString(m.w, "\n"); err != nil {
			return err
		}
	}
	m.started = true
	m.lastWasBlank = false
	m.lastWasHeadingLike = false
	_, err := io.WriteString(m.w, line)
	return err
}

// emit writes a line that is not a block-start (list/blockquote/table).
func (m *minifier) emit(line string) error {
	return m.emitContent(line, false)
}

// emitBlock writes a line that starts a block element (list/blockquote/table),
// which may suppress a preceding blank line after a heading.
func (m *minifier) emitBlock(line string) error {
	return m.emitContent(line, true)
}

func (m *minifier) emitBlank() error {
	// Collapse consecutive blank lines.
	if m.lastWasBlank || m.pendingBlank {
		return nil
	}
	// Don't emit a blank line at the very start.
	if !m.started {
		return nil
	}
	m.lastWasBlank = true
	m.pendingBlank = true
	return nil
}
