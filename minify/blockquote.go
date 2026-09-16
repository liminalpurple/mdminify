package minify

import (
	"bytes"
	"strings"
)

// processBlockquote takes a slice of blockquote lines (with > prefix still
// present), strips one level of prefix, recursively minifies the inner
// content, then re-adds the > prefix.
func processBlockquote(lines []string) ([]string, error) {
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

	// Recursing without having removed anything would not terminate. Callers
	// should only pass blocks with at least one prefix, so this is a guard
	// against a future mismatch rather than an expected path.
	if !stripped {
		return lines, nil
	}

	// Recursively minify the inner content.
	innerText := strings.Join(inner, "\n") + "\n"
	var buf bytes.Buffer
	if err := Minify(strings.NewReader(innerText), &buf); err != nil {
		return nil, err
	}

	// Re-add > prefix to each output line.
	output := buf.String()
	// Remove trailing newline added by Minify for clean splitting.
	output = strings.TrimRight(output, "\n")
	outLines := strings.Split(output, "\n")

	var result []string
	for _, ol := range outLines {
		if ol == "" {
			result = append(result, ">")
		} else {
			result = append(result, "> "+ol)
		}
	}
	return result, nil
}
