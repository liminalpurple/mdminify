package minify

import "strings"

// processBlockquote takes a slice of blockquote lines (with > prefix still
// present), strips one level of prefix, recursively minifies the inner
// content, then re-adds the > prefix.
func processBlockquote(lines []string, depth int, inListItem bool) ([]string, error) {
	if containerTabsUnsafe(lines, 0) {
		return lines, nil
	}

	// The marker's spelling is preserved rather than normalised. ">x" and
	// "> x" hold the same content, because CommonMark makes the space after >
	// optional, but goldmark parses them differently once a second line is
	// involved: ">*0" over ">*" renders as emphasis while "> *0" over "> *"
	// renders as literal text. Rewriting the marker can therefore change the
	// rendering, and normalising ">x" to "> x" also added a byte to every
	// line, so the spelling the input used is kept.
	//
	// A block mixing the two spellings has no single spelling to keep, so it
	// is passed through rather than having one chosen for it.
	tight, spaced := false, false
	for _, line := range lines {
		t, s := blockquoteMarkerStyle(line)
		tight = tight || t
		spaced = spaced || s
	}
	if tight && spaced {
		return lines, nil
	}

	// Strip one level of > prefix.
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

	// Rule 2: recursing without having removed anything would not terminate.
	// Callers should only pass blocks with at least one prefix, so this is a
	// guard against a future mismatch rather than an expected path.
	if !stripped {
		return lines, nil
	}

	outLines, err := minifyInner(inner, depth, inListItem)
	if err != nil {
		return nil, err
	}

	// A blank line ending the quote closes its paragraph. Minify drops a
	// trailing blank, so it is restored here: without it a following
	// unprefixed line would be read as a lazy continuation of the quote.
	if containerEndsInBlank(inner) && len(outLines) > 0 && outLines[len(outLines)-1] != "" {
		outLines = append(outLines, "")
	}

	// Re-apply the block's original indentation. Up to three leading spaces
	// are significant: they can be what places the quote inside a list item,
	// and without container tracking there is no way to tell that case from a
	// top-level quote whose indent could safely be dropped.
	indent := strings.Repeat(" ", countLeadingSpaces(lines[0]))

	// A tight marker cannot carry content that begins with a space, because
	// re-parsing would consume that space as the marker's own: ">" plus
	// "  - b" reads back as " - b". Such a block is passed through.
	marker := "> "
	if tight {
		for _, ol := range outLines {
			if strings.HasPrefix(ol, " ") {
				return lines, nil
			}
		}
		marker = ">"
	}

	// Rule 3: the prefix goes on in front of each line and nothing is trimmed.
	var result []string
	for _, ol := range outLines {
		if ol == "" {
			result = append(result, indent+">")
		} else {
			result = append(result, indent+marker+ol)
		}
	}
	return result, nil
}
