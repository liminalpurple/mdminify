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
	return minifyDepth(r, w, 0)
}

// maxContainerDepth bounds how far blockquotes and list items may nest before
// their contents are passed through untouched. Each level of recursion strips
// at least one column, so deeply nested input terminates on its own, but only
// after a stack frame per level. The limit keeps pathological input, such as a
// line of thousands of repeated list markers, from exhausting the stack.
const maxContainerDepth = 64

func minifyDepth(r io.Reader, w io.Writer, depth int) error {
	scanner := bufio.NewScanner(r)
	m := &minifier{w: w, depth: depth}
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
	stateIndentedCode
)

type minifier struct {
	w     io.Writer
	state state
	depth int // container nesting depth, for the recursion guard

	fence fenceInfo // current fenced code block info
	para  paragraphBuffer

	// Table handling: we buffer potential table rows until we can confirm
	// we're actually in a table (by seeing a separator row).
	tableBuf     []string // buffered rows before separator confirmed
	inTable      bool     // true once separator has been seen
	tableStarted bool     // true if we've started buffering table rows

	// The table began directly after paragraph text, with no blank line. A
	// renderer may have started the table earlier than we did, which would
	// make the row we read as the delimiter one of its body rows. Padding is
	// trimmed either way, but the delimiter is not reduced, since reducing a
	// body cell reading "---" would change what the table says.
	tableAfterParagraph bool

	// Blockquote handling: accumulate consecutive blockquote lines.
	bqBuf []string

	// Indented code handling: the block is buffered so that trailing blank
	// lines, which belong after the block rather than inside it, can be
	// separated from interior ones, which are content.
	codeBuf []string

	// List handling: the lines of the current item, which are minified as a
	// unit so that indentation inside it is measured from the item's content
	// offset rather than from column zero.
	listBuf    []string
	listOffset int

	// Track consecutive blank lines.
	lastWasBlank bool

	// A blank line is pending — deferred so we can suppress it if the next
	// content is a block element following a heading-like line.
	pendingBlank bool

	// The last emitted block was heading-like (ATX heading, setext heading,
	// or bold-as-heading paragraph).
	lastWasHeadingLike bool

	// That heading-like block was a bold-as-heading paragraph rather than a
	// real heading. Removing the blank line after a real heading is always
	// safe; after a paragraph it is only safe if the next block is one that
	// may interrupt a paragraph, since otherwise it would be absorbed.
	lastHeadingWasParagraph bool

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

	// --- INDENTED CODE BLOCK (continuation) ---
	if m.state == stateIndentedCode {
		// Blank lines and indented lines stay in the block. A blank might be
		// trailing, so it is buffered rather than emitted, and dropped on exit
		// if nothing indented follows.
		if isBlank(line) || indentWidth(line) >= 4 {
			m.codeBuf = append(m.codeBuf, line)
			return nil
		}
		if err := m.flushIndentedCode(); err != nil {
			return err
		}
		// Fall through to process this line normally.
	}

	// --- LIST ITEM ---
	// Containers are handled before leaf blocks, because a line's meaning
	// inside an item is relative to that item's content offset. The item is
	// buffered and minified as a unit, which is what lets nested structure be
	// measured correctly rather than from column zero.
	if len(m.listBuf) > 0 {
		if isBlank(line) || indentWidth(line) >= m.listOffset {
			m.listBuf = append(m.listBuf, line)
			return nil
		}
		if err := m.flushList(); err != nil {
			return err
		}
		// Fall through: the line may open the next item, or end the list.
	}
	if prefix, ok := parseListItemPrefix(line); ok && m.canStartList() {
		// A list may only interrupt an open paragraph under narrow conditions.
		// Where it may not, the line is left to the paragraph buffer, which
		// passes it through rather than guessing.
		if len(m.para.lines) == 0 || prefix.interruptsParagraph(line) {
			if err := m.flushParagraph(); err != nil {
				return err
			}
			m.listOffset = prefix.contentOffset()
			m.listBuf = append(m.listBuf, line)
			return nil
		}
	}

	// --- INDENTED CODE BLOCK (start) ---
	if indentWidth(line) >= 4 && m.canStartIndentedCode() {
		m.state = stateIndentedCode
		m.codeBuf = append(m.codeBuf, line)
		return nil
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
			return m.emitBlock(squashTableRow(line, false))
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
						if err := m.emitBlock(squashTableRow(r, false)); err != nil {
							return err
						}
					} else {
						if err := m.emit(squashTableRow(r, false)); err != nil {
							return err
						}
					}
				}
				m.tableBuf = nil
				return m.emit(squashTableRow(line, !m.tableAfterParagraph))
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
		m.tableAfterParagraph = len(m.para.lines) > 0
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
	if err := m.flushIndentedCode(); err != nil {
		return err
	}
	if err := m.flushList(); err != nil {
		return err
	}
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
		// A setext underline only underlines paragraph text. After a heading,
		// or after another underline, the same characters are a paragraph of
		// their own, and treating them as a heading would wrongly suppress the
		// blank line that follows.
		setext := i > 0 && isSetextUnderline(l) &&
			!startsNewBlock(lines[i-1]) && !isSetextUnderline(lines[i-1])

		switch {
		case isATXHeading(l) || setext:
			m.lastWasHeadingLike = true
			m.lastHeadingWasParagraph = false
		case isBoldAsHeading(l):
			m.lastWasHeadingLike = true
			m.lastHeadingWasParagraph = true
		}
	}
	return nil
}

// canStartIndentedCode reports whether an indented line at this point opens an
// indented code block. It cannot interrupt a paragraph, and anything already
// buffered means some other block is still open.
func (m *minifier) canStartIndentedCode() bool {
	return m.state == stateNormal &&
		len(m.para.lines) == 0 &&
		len(m.bqBuf) == 0 &&
		len(m.listBuf) == 0 &&
		len(m.tableBuf) == 0 &&
		!m.inTable
}

// canStartList reports whether a list marker at this point opens an item we
// can safely take over. Beyond the depth limit the item is left alone, as are
// markers appearing while another container is being buffered.
func (m *minifier) canStartList() bool {
	return m.state == stateNormal &&
		m.depth < maxContainerDepth &&
		len(m.bqBuf) == 0 &&
		len(m.tableBuf) == 0 &&
		!m.inTable
}

// flushList minifies the buffered list item and emits it. Trailing blank lines
// fall outside the item, where they mark the list as loose.
func (m *minifier) flushList() error {
	if len(m.listBuf) == 0 {
		return nil
	}

	lines, looseBlank, err := processListItem(m.listBuf, m.depth)
	m.listBuf = nil
	if err != nil {
		return err
	}

	for i, l := range lines {
		if i == 0 {
			if err := m.emitBlock(l); err != nil {
				return err
			}
		} else if err := m.emit(l); err != nil {
			return err
		}
	}

	if looseBlank {
		return m.emitBlank()
	}
	return nil
}

// flushIndentedCode emits a buffered indented code block verbatim. Interior
// blank lines are content and are written as they are; trailing blank lines
// fall outside the block and are handed to the usual blank handling.
func (m *minifier) flushIndentedCode() error {
	if len(m.codeBuf) == 0 {
		m.state = stateNormal
		return nil
	}

	end := len(m.codeBuf)
	for end > 0 && isBlank(m.codeBuf[end-1]) {
		end--
	}
	trailing := len(m.codeBuf) - end
	lines := m.codeBuf[:end]

	for _, l := range lines {
		if err := m.emit(l); err != nil {
			return err
		}
	}

	m.codeBuf = nil
	m.state = stateNormal

	if trailing > 0 {
		return m.emitBlank()
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
	lines, err := processBlockquote(m.bqBuf, m.depth)
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
		suppress := m.lastWasHeadingLike && followsHeading &&
			(!m.lastHeadingWasParagraph || canInterruptParagraph(line))
		if !suppress {
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
	m.lastHeadingWasParagraph = false
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
