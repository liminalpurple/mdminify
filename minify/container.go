package minify

import (
	"bytes"
	"strings"
)

// This file holds the rules shared by every block container — today a
// blockquote and a list item. Both are minified the same way: strip the
// container's prefix, minify the content one level deeper, put the prefix
// back. They differ only in the prefix itself, and processBlockquote and
// processListItem keep that part to themselves.
//
// Everything else belongs here, because it is not really about blockquotes or
// list items but about the lines inside any container, and a rule discovered
// through one container has always turned out to hold for the other too. Each
// of the helpers below was first written inline in one of them and needed in
// the other later, once a bug had been shipped.
//
// The contract a container must keep:
//
//  1. Decline rather than guess. Returning the input unchanged always renders
//     correctly, so any doubt is answered by passing the block through. Every
//     guard here reports that a container is unsafe to rewrite, never that it
//     is safe.
//
//  2. Make progress before recursing. A container that strips nothing and
//     recurses on its own input does not terminate.
//
//  3. Never trim a line while putting the prefix back. Trailing whitespace is
//     a hard break, and the line after a container's last line may lazily
//     continue the paragraph inside it, so the break is load-bearing even at
//     the very end of the block.
//
//  4. Measure indentation, never assume it. A tab's width depends on the
//     column it lands in, which no container tracks; see containerTabsUnsafe.

// containerTabsUnsafe reports whether the block cannot be rewritten because a
// tab appears in indentation that rewriting would move.
//
// A tab advances to the next four-column stop, so its width depends on where
// it starts. Rewriting a prefix ahead of one — normalising ">>" to "> > ", or
// dedenting a list item to its content offset and re-indenting it — moves the
// tab to a different column, where it stands for a different amount of
// indentation. ">> \t0" is a paragraph; the normalised "> > \t0" pushes the
// tab past column 4 and makes it an indented code block.
//
// Expanding the tab correctly would need its absolute column, which is not
// tracked inside a container, so a block with a tab in its indentation is
// passed through untouched.
//
// firstFrom is the byte offset on the opening line at which to start looking,
// letting a caller skip a marker that is not itself indentation. The whole
// block is checked before anything is rewritten, because an outer level moves
// the column just as an inner one does.
func containerTabsUnsafe(lines []string, firstFrom int) bool {
	if len(lines) == 0 {
		return false
	}
	if firstFrom <= len(lines[0]) && hasTabInPrefix(lines[0][firstFrom:]) {
		return true
	}
	for _, line := range lines[1:] {
		if hasTabInPrefix(line) {
			return true
		}
	}
	return false
}

// containerEndsInBlank reports whether the container's stripped content ends
// in a blank line that belongs outside the container.
//
// The distinction matters because such a line is significant in both
// directions and in different ways: it closes a blockquote's paragraph, so
// processBlockquote restores one that Minify dropped, and it falls outside a
// list item, where it marks the list loose.
//
// Inside an unclosed fence the same line is code content, which Minify keeps.
// Treating it as a blank then corrupts the block — by appending a line that
// was never in the input, or by moving one out of the code and making a list
// loose on its account.
func containerEndsInBlank(inner []string) bool {
	return len(inner) > 0 && !endsInOpenFence(inner) &&
		strings.TrimSpace(inner[len(inner)-1]) == ""
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

// minifyInner minifies a container's stripped content one level deeper and
// returns the output as lines.
//
// Minify emits exactly one trailing newline, so only that one is removed.
// Trimming every trailing newline would discard a final blank line, which is
// content when the content ends inside an unclosed fence.
func minifyInner(inner []string, depth int, inListItem bool) ([]string, error) {
	var buf bytes.Buffer
	if err := minifyDepth(strings.NewReader(strings.Join(inner, "\n")+"\n"), &buf, depth+1, inListItem); err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n"), nil
}
