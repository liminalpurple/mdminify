package conformance

import (
	"strings"
	"testing"
)

// Every container bug found so far has eventually turned out to apply to the
// other container too, because a blockquote and a list item do the same job
// and were written separately. Finding each rule twice, once per container and
// usually a shipped bug apart, is the pattern this file exists to break.
//
// TestContainerParity takes the constructs that have caused trouble and runs
// each one through every container wrapping, so a body added here is checked
// against all of them at once and a rule that holds for one container is
// immediately tested against the rest.

// parityBodies are constructs whose meaning depends on whitespace or on
// context a single line does not show. Each has broken at least one container.
var parityBodies = map[string]string{
	"open-fence":          "```\n  \n",
	"open-fence-content":  "```\ncode\n",
	"closed-fence":        "```\ncode\n```\n",
	"fence-inner-blank":   "```\na\n\nb\n```\n",
	"hard-break":          "a  \nb\n",
	"hard-break-end":      "a  \n",
	"backslash-break":     "a\\\nb\n",
	"tab-indent":          "\t0\n",
	"tab-after-marker":    "* \t0\n",
	"indented-code":       "    code\n",
	"heading-then-list":   "# h\n\n* x\n",
	"heading-then-para":   "# h\n\ntext\n",
	"link-ref":            "[0]:\n0\n0\n",
	"link-ref-title":      "[a]: /u\n\"t\"\nmore\n",
	"table-pipeless":      "0\n-:\n0\n0\n",
	"table-padded":        "| a | b |\n| - | - |\n| 1 | 2 |\n",
	"table-blank-cell":    "|  | \n|-: \n",
	"setext":              "title\n===\n",
	"thematic-break":      "a\n\n---\n\nb\n",
	"blank-between":       "a\n\nb\n",
	"trailing-blank":      "a\n\n",
	"empty-marker":        "-\ntext\n",
	"loose-list":          "- a\n\n- b\n",
	"nested-list":         "- a\n  - b\n",
	"soft-wrap":           "one\ntwo\nthree\n",
	"html-block":          "<div>\na\n</div>\n",
	"unclosed-tag":        "<span\nclass=\"x\">a\n",
	"marker-tab-padding":  "*   \t0\n",
	"para-then-indented":  "text\n    still the same paragraph\n",
	"list-item-code":      "*     0 \n",
	"lazy-continuation":   "- item\ncontinued\n",
	"blockquote-inside":   "> quoted\n> more\n",
	"heading-in-item":     "* a\n  # h\n   \n  * b\n",
	"consecutive-markers": "* a\n* b\n* c\n",
}

// parityWrappers place a body inside each container, and inside combinations
// of them, since a rule has to survive nesting as well as each container
// alone.
var parityWrappers = map[string]func(string) string{
	"bare":            func(s string) string { return s },
	"quote":           func(s string) string { return wrapQuote(s, "> ", ">") },
	"quote-tight":     func(s string) string { return wrapQuote(s, ">", ">") },
	"quote-indented":  func(s string) string { return wrapQuote(s, "  > ", "  >") },
	"quote-nested":    func(s string) string { return wrapQuote(wrapQuote(s, "> ", ">"), "> ", ">") },
	"item-dash":       func(s string) string { return wrapItem(s, "- ", "  ") },
	"item-star":       func(s string) string { return wrapItem(s, "* ", "  ") },
	"item-ordered":    func(s string) string { return wrapItem(s, "1. ", "   ") },
	"item-wide":       func(s string) string { return wrapItem(s, "-   ", "    ") },
	"item-nested":     func(s string) string { return wrapItem(wrapItem(s, "- ", "  "), "- ", "  ") },
	"quote-in-item":   func(s string) string { return wrapItem(wrapQuote(s, "> ", ">"), "- ", "  ") },
	"item-in-quote":   func(s string) string { return wrapQuote(wrapItem(s, "- ", "  "), "> ", ">") },
	"item-after-text": func(s string) string { return "intro\n\n" + wrapItem(s, "- ", "  ") },
	"quote-after-text": func(s string) string {
		return "intro\n\n" + wrapQuote(s, "> ", ">")
	},
}

// wrapQuote puts every line of the body behind a blockquote marker, using the
// bare marker for a blank line so no trailing space is introduced.
func wrapQuote(body, prefix, blank string) string {
	var b strings.Builder
	for _, line := range splitBody(body) {
		if line == "" {
			b.WriteString(blank)
		} else {
			b.WriteString(prefix + line)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// wrapItem puts the body inside one list item: the marker opens the first
// line and the remaining lines are indented to the content offset. A blank
// line stays empty, since indenting it would only add trailing whitespace.
func wrapItem(body, marker, indent string) string {
	var b strings.Builder
	for i, line := range splitBody(body) {
		switch {
		case i == 0:
			b.WriteString(marker + line)
		case line == "":
		default:
			b.WriteString(indent + line)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// splitBody returns the body's lines without the empty element that trailing
// newline would otherwise produce.
func splitBody(body string) []string {
	return strings.Split(strings.TrimSuffix(body, "\n"), "\n")
}

// parityKnownBroken lists body/wrapper combinations that currently violate the
// contract, keyed "wrapper/body". Entries must name their cause; the test
// asserts they still fail, so a fix is reported rather than passing silently.
var parityKnownBroken = map[string]string{}

func TestContainerParity(t *testing.T) {
	for wname, wrap := range parityWrappers {
		for bname, body := range parityBodies {
			key := wname + "/" + bname
			t.Run(key, func(t *testing.T) {
				src := wrap(body)
				ok := checkEquivalent(t, src)
				if cause, broken := parityKnownBroken[key]; broken {
					if ok {
						t.Errorf("in parityKnownBroken but now passes — remove it from the map.\n"+
							"recorded cause: %s", cause)
					}
				} else if !ok {
					t.Errorf("container parity broken for %s", key)
				}
			})
		}
	}
}
