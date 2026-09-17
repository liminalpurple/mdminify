package minify

import (
	"bytes"
	"strings"
)

// processBlockquote takes a slice of blockquote lines (with > prefix still
// present), strips one level of prefix, recursively minifies the inner
// content, then re-adds the > prefix.
func processBlockquote(lines []string, depth int) ([]string, error) {
	// Strip one level of > prefix.
	// A tab's width depends on the column it lands in, so rewriting any marker
	// ahead of one changes how much indentation it represents: ">> \t0" is a
	// paragraph, while the normalised "> > \t0" pushes the tab past column 4
	// and makes it an indented code block. Expanding the tab would need the
	// absolute column, which is not tracked inside a container, so a quote
	// whose prefix contains a tab is passed through untouched instead. This is
	// checked across the whole block before anything is rewritten, because the
	// outer level moves the column just as the inner ones do.
	for _, line := range lines {
		if hasTabInPrefix(line) {
			return lines, nil
		}
	}

	var inner []string
	stripped := false
	for _, line := range lines {
		s, ok := stripBlockquotePrefix(line)
		if ok {
			stripped = true
			inner = append(inner, s)
		} else {
			// Lazy continuation — line without > that continues a blockquote paragraph.
			inner = append(inner, line)
		}
	}

	// Recursing without having removed anything would not terminate. Callers
	// should only pass blocks with at least one prefix, so this is a guard
	// against a future mismatch rather than an expected path.
	if !stripped {
		return lines, nil
	}

	// Recursively minify the inner content.
	innerText := strings.Join(inner, "\n") + "\n"
	var buf bytes.Buffer
	if err := minifyDepth(strings.NewReader(innerText), &buf, depth+1); err != nil {
		return nil, err
	}

	// Re-add > prefix to each output line.
	output := buf.String()
	// Remove trailing newline added by Minify for clean splitting.
	output = strings.TrimSuffix(output, "\n")
	outLines := strings.Split(output, "\n")

	// A blank line ending the quote closes its paragraph. Minify drops a
	// trailing blank, so it is restored here: without it a following
	// unprefixed line would be read as a lazy continuation of the quote.
	if n := len(inner); n > 0 && strings.TrimSpace(inner[n-1]) == "" &&
		len(outLines) > 0 && outLines[len(outLines)-1] != "" {
		outLines = append(outLines, "")
	}

	// Re-apply the block's original indentation. Up to three leading spaces
	// are significant: they can be what places the quote inside a list item,
	// and without container tracking there is no way to tell that case from a
	// top-level quote whose indent could safely be dropped.
	indent := strings.Repeat(" ", countLeadingSpaces(lines[0]))

	var result []string
	for _, ol := range outLines {
		if ol == "" {
			result = append(result, indent+">")
		} else {
			result = append(result, indent+"> "+ol)
		}
	}
	return result, nil
}
