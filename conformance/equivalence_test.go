// Package conformance verifies mdminify's core contract: that minified output
// renders to byte-identical HTML.
//
// It lives in its own module so that the goldmark renderer it depends on never
// enters the mdminify module, which is deliberately zero-dependency.
//
// Run it explicitly, from this directory:
//
//	go test ./...
//	go test -run Fuzz -fuzz FuzzHTMLEquivalence
package conformance

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/liminalpurple/mdminify/minify"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// md renders with GFM enabled, because mdminify handles GFM tables.
var md = goldmark.New(goldmark.WithExtensions(extension.GFM))

// render converts markdown source to normalised HTML.
func render(t *testing.T, src string) string {
	t.Helper()
	var buf strings.Builder
	if err := md.Convert([]byte(src), &buf); err != nil {
		t.Fatalf("rendering markdown: %v", err)
	}
	out, err := normaliseHTML(buf.String())
	if err != nil {
		t.Fatalf("normalising HTML: %v", err)
	}
	return out
}

// minified runs src through mdminify.
func minified(t *testing.T, src string) string {
	t.Helper()
	var buf strings.Builder
	if err := minify.Minify(strings.NewReader(src), &buf); err != nil {
		t.Fatalf("Minify error: %v", err)
	}
	return buf.String()
}

// equalHTML reports whether two markdown sources render to the same
// normalised HTML.
func equalHTML(t *testing.T, a, b string) bool {
	t.Helper()
	return render(t, a) == render(t, b)
}

// checkEquivalent asserts that minifying src does not change its rendered HTML.
// It returns true if the contract held.
func checkEquivalent(t *testing.T, src string) bool {
	t.Helper()
	got := minified(t, src)
	wantHTML := render(t, src)
	gotHTML := render(t, got)
	if gotHTML == wantHTML {
		return true
	}
	t.Logf("--- input ---\n%s\n--- minified ---\n%s\n--- want HTML ---\n%s\n--- got HTML ---\n%s",
		src, got, wantHTML, gotHTML)
	return false
}

// TestTestdataEquivalence checks every testdata input, and each committed
// golden file, against the HTML-equivalence contract.
func TestTestdataEquivalence(t *testing.T) {
	inputs, err := filepath.Glob("../testdata/*.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no testdata files found")
	}

	for _, path := range inputs {
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			if !checkEquivalent(t, string(src)) {
				t.Errorf("minifying %s changed the rendered HTML", filepath.Base(path))
			}
		})
	}
}

// cases covers constructs that golden files do not currently reach. Keep the
// names stable: knownBroken refers to them.
var cases = map[string]string{
	// A tab in a blockquote prefix: its width depends on the column it lands
	// in, so every marker rewritten ahead of it moves that column.
	"bq-tab-nested":        ">>\t0\n",
	"bq-tab-nested-space":  ">> \t0\n",
	"bq-tab-nested-2space": ">>  \t0\n",
	"bq-tab-single":        ">\t0\n",
	"bq-tab-double":        ">>\t\t0\n",
	"bq-tab-indented":      "  >>\t0\n",
	"bq-tab-multiline":     ">>\tx\n>>\ty\n",
	"bq-tab-list":          ">\t- a\n",
	"bq-tab-heading":       ">>\t# h\n",
	"bq-tab-fence":         ">>\t```\n>>\tc\n",
	"bq-tab-quote":         ">>\t> q\n",
	// A hard break at the end of a list item, lazily continued from outside it.
	"list-item-hard-break": "* 0  \n0\n",
	// A tight blockquote marker changes goldmark's inline parsing, so its
	// spelling is preserved rather than normalised.
	"quote-tight-emphasis": ">*0\n>*\n",
	"quote-tight-text":     ">quoted\n>more\n",
	"quote-tight-nested":   ">>*0\n>>*\n",
	"quote-mixed-spelling": "> *0\n>*\n",
	"quote-tight-indented": ">- a\n>  - b\n",
	// A tab after a list marker is still indentation, so rewriting an enclosing
	// quote's prefix moves it.
	"quote-marker-tab":      ">*   \t00\n",
	"quote-marker-tab-dash": ">- \tx\n",
	"quote-marker-tab-num":  ">1. \tx\n",
	"quote-marker-tab-deep": ">>* \tx\n",
	"quote-emphasis-tab":    "> *emphasis* \tx\n",
	// A quote ending inside an unclosed fence: the trailing whitespace line is
	// code content, not a blank line closing the quote.
	"quote-open-fence-blank": ">>```\n>>  \n",
	"quote-open-fence-outer": "> ```\n>   \n",
	"quote-open-fence-tilde": ">>~~~\n>>  \n",
	"quote-closed-fence":     "> ```\n> c\n> ```\n>  \n",
	"quote-fence-mid-blank":  "> ```\n> c\n>  \n> ```\n",
	// A tab in a list item's indentation: dedenting and re-indenting the item
	// would move the tab to a different column.
	"list-tab-content":      "* \t\n\t\t0\n",
	"list-tab-marker":       "*\t0\n",
	"list-tab-nested":       "- a\n\t- b\n",
	"list-tab-nested-deep":  "- a\n\t\t- b\n",
	"list-tab-continuation": "* a\n\tcont\n",
	"list-tab-para":         "* a\n\n\tpara\n",
	"list-tab-pad":          "-  \tx\n",
	"list-tab-fence":        "* a\n\t```\n\tc\n\t```\n",
	"list-tab-in-quote":     "> * a\n> \t- b\n",
	"list-tab-inline":       "* a\tb\n",
	// A blank line inside a list item makes the containing list loose.
	"list-item-loose-blank": "* a\n  # 0\n   \n  * 0\n",
	// A whitespace-only header cell carries alignment an empty one does not.
	"table-blank-header-cell": "|  | \n|-: \n",
	"table-empty-header-cell": "||\n|-:|\n",
	// A list item holding indented code, where trailing space is content.
	"list-item-code-space": ">\n*     0 \n",
	// A pipeless single-column table whose body rows must stay separate.
	"table-pipeless-body": "0\n-:\n0\n0\n",
	// A link reference definition whose destination is on the next line.
	"linkref-split":        "[0]:\n0\n0\n",
	"nested-list-4-space":  "- top level\n    - nested item\n        - deeper item\n- second top\n",
	"nested-list-2-space":  "- top level\n  - nested item\n- second top\n",
	"nested-list-ordered":  "1. ordered\n    1. nested under ordered\n2. second\n",
	"indented-code":        "Some text.\n\n    code line one\n    code line two\n\nAfter.\n",
	"indented-code-first":  "    code line one\n    code line two\n",
	"lazy-continuation":    "Some text.\n    still the same paragraph\n",
	"loose-list":           "- item one\n\n- item two\n",
	"empty-list-item":      "-\ntext\n",
	"empty-list-ordered":   "0)\n0000\n",
	"empty-list-trailing":  "- \ntext\n",
	"list-lazy-continue":   "- item\ntext\n",
	"empty-item-after":     "* 0\n0)\n",
	"indented-code-trail":  "    code line  \n",
	"code-then-para":       "    code\ntext\n",
	"setext-then-text":     "Title\n=====\ntext\n",
	"linkref-then-text":    "[0]: http://e.com\ntext\n",
	"thematic-then-text":   "---\ntext\n",
	"indented-blockquote":  "*\n  >0\n",
	"code-block-with-gt":   "    > x\n",
	"code-gt-after-para":   "text\n\n    > x\n\nmore\n",
	"tab-indented-code":    "\t0\n00\n",
	"tab-code-after-para":  "text\n\n\tcode\n\nmore\n",
	"literal-backslash":    "a \\\n",
	"backslash-hardbreak":  "a \\\nb\n",
	"item-holding-head":    "* #\n0\n",
	"nested-marker-head":   "- - #\ntext\n",
	"item-then-continue":   "- item one\n  wraps\n",
	"unclosed-fence":       "```0\n",
	"fence-in-list-item":   "* ```\n0\n",
	"html-type7-block":     "<A>\n0\n",
	"inline-html-joins":    "some <em>text</em> that\nwraps here\n",
	"table-cell-mismatch":  "||0\n|-\n",
	"table-aligned":        "| a | b |\n| --- | :---: |\n| 1 | 2 |\n",
	"loose-empty-first":    "-\n\n- 0\n",
	"setext-then-list":     "Title\n=====\n\n- item\n",
	"blank-in-code-block":  "    0\n\n\n    00\n",
	"code-trailing-blanks": "    code\n\n\ntext\n",
	"code-then-code":       "    a\n\n    b\n",
	"fence-on-marker-line": "* ```\n \n",
	"fence-closed-in-item": "- ```\n  code\n  ```\n",
	"bold-then-empty-item": "**0**\n\n*\n",
	"bold-then-item":       "**B**\n\n- item\n",
	"bold-then-ordered-2":  "**B**\n\n2. x\n",
	"nested-list-wrapped":  "- top spans\n  two lines\n    - nested that\n      wraps\n- second\n",
	"loose-item-blank":     "* \n\n  0\n",
	"table-body-dashes":    "| syntax | meaning |\n| --- | --- |\n| --- | break |\n| :-: | centre |\n",
	"table-after-para":     "0\n-|-|-\n|||0\n|-|-|--\n",
	"tab-after-marker":     "*\t#\n0\n",
	"pseudo-setext":        "0\n#\n=\n\n*\n",
	"join-makes-hr":        "**\n**\n",
	"join-makes-setext":    "text\n-\n-\n",
	"empty-item-nested":    "*\n  +\n",
	"empty-item-text":      "*\n  a\n",
	"quote-blank-closes":   ">0\n>\n0\n",
	"html-comment-blank":   "<!-- Foo\n\nbar\n   baz -->\nokay\n",
	"html-pre-blank":       "<pre>\n\ncode\n</pre>\nokay\n",
	"html-cdata-blank":     "<![CDATA[\n\nx\n]]>\nokay\n",
	"html-multiline-tag":   "<img src=\"a.png\"\n     alt=\"a picture\">\n",
	"linkref-title-next":   "[foo]: /url\n  \"title\"\n\n[foo]\n",
	"tab-marker-code":      "*\t\t0 \n",
	"hardbreak-then-pipe":  "0  \n|\n",
	"pipeless-table":       "0\n-:\n",
	"para-then-pipeless":   "0\n0\n-:\n",
	"space-after-slash":    "|\\ \n0\n",
	"pipeless-table-body":  "0\n-:\n0\n",
	"tight-list":           "- item one\n- item two\n",
	"list-para-continued":  "- item one spans\n  multiple lines.\n- item two.\n",
	"bold-as-heading":      "**Section**\n\n- item\n",
	"triple-bold-heading":  "***Section***\n\n- item\n",
	"italic-not-heading":   "*emphasis*\n\n- item\n",
	"setext-heading":       "Title\n=====\n\nBody text.\n",
	"thematic-break":       "Above.\n\n***\n\nBelow.\n",
	"fenced-code-indent":   "- item\n\n  ```go\n  x := 1\n  ```\n",
	"blockquote-nested":    "> outer\n> > inner wrapping\n> > across lines\n",
	"hard-break-spaces":    "line one  \nline two\n",
	"hard-break-slash":     "line one\\\nline two\n",
	"table-basic":          "| a | b |\n| --- | --- |\n| 1 | 2 |\n",
	"table-no-lead-pipe":   "a | b\n--- | ---\n1 | 2\n",
	"table-bare-minimal":   "a\n-|\n",
	"html-block":           "<div>\n  <p>raw</p>\n</div>\n",
	"link-ref-def":         "[a]: http://example.com\n\nSee [a].\n",
}

// knownBroken lists cases that currently violate the contract. Entries must
// name their cause; the test asserts they still fail, so a fix is reported
// rather than passing silently.
var knownBroken = map[string]string{}

func TestCaseEquivalence(t *testing.T) {
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			ok := checkEquivalent(t, src)
			reason, broken := knownBroken[name]

			switch {
			case ok && broken:
				t.Errorf("case is in knownBroken but now passes — remove it from the map.\nrecorded cause: %s", reason)
			case !ok && broken:
				t.Logf("known failure: %s", reason)
			case !ok:
				t.Errorf("minifying this input changed the rendered HTML")
			}
		})
	}
}

// TestKnownBrokenNamesExist guards against a case being renamed or deleted
// while leaving a stale entry in knownBroken.
func TestKnownBrokenNamesExist(t *testing.T) {
	for name := range knownBroken {
		if _, ok := cases[name]; !ok {
			t.Errorf("knownBroken refers to %q, which is not in cases", name)
		}
	}
}

// TestIdempotent checks that minifying twice equals minifying once. A transform
// that keeps changing the document cannot be safe to run in a pre-commit hook.
func TestIdempotent(t *testing.T) {
	inputs, err := filepath.Glob("../testdata/*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range inputs {
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			once := minified(t, string(src))
			twice := minified(t, once)
			if once != twice {
				t.Errorf("not idempotent\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
			}
		})
	}
}

// FuzzHTMLEquivalence explores the same contract over generated input.
// goldmarkQuoteQuirk reports whether goldmark renders src differently from the
// same document with every blockquote marker spelled "> " instead of ">".
//
// CommonMark makes the space after > optional, so the two are one document,
// but goldmark parses emphasis across a line break differently between them:
// ">>*0" over ">>*!" renders as <em>0</em>!, while "> > *0" over "> > *!",
// ">>*0 *!" and the unquoted "*0" over "*!" all render the literal text. The
// spec agrees with the majority — a * preceded by a newline is not
// right-flanking and cannot close emphasis — so the tight-marker reading is
// the one that is wrong.
//
// Where goldmark contradicts itself it cannot be the oracle for this
// contract, so the fuzz target skips such inputs rather than reporting a
// violation mdminify did not commit. The rewrite is not applied inside fenced
// code, where a leading > is content, so a fence holding one is skipped too:
// losing a little coverage is the safe direction for an oracle problem.
func goldmarkQuoteQuirk(t *testing.T, src string) bool {
	t.Helper()
	spaced := spaceQuoteMarkers(src)
	if spaced == src {
		return false
	}
	return render(t, src) != render(t, spaced)
}

// spaceQuoteMarkers rewrites each line's leading run of blockquote markers so
// every > is followed by a space, leaving the rest of the line alone.
func spaceQuoteMarkers(src string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		j := 0
		for j < len(line) && line[j] == ' ' {
			j++
		}
		var b strings.Builder
		b.WriteString(line[:j])
		changed := false
		for j < len(line) && line[j] == '>' {
			b.WriteByte('>')
			j++
			if j < len(line) && line[j] != ' ' {
				b.WriteByte(' ')
				changed = true
			}
		}
		if changed {
			lines[i] = b.String() + line[j:]
		}
	}
	return strings.Join(lines, "\n")
}

// TestGoldmarkQuoteQuirkStillPresent pins the upstream inconsistency that
// goldmarkQuoteQuirk exists to work around, so a goldmark fix or a dependency
// bump reports that the skip can go rather than passing silently.
func TestGoldmarkQuoteQuirkStillPresent(t *testing.T) {
	const src = ">>*0\n>>*!\n"
	if !goldmarkQuoteQuirk(t, src) {
		t.Errorf("goldmark now renders %q consistently with its spaced-marker "+
			"form — remove goldmarkQuoteQuirk and its use in the fuzz target", src)
	}
}

func FuzzHTMLEquivalence(f *testing.F) {
	// Seed only with cases that currently hold. Entries removed from
	// knownBroken rejoin the corpus automatically.
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, broken := knownBroken[name]; !broken {
			f.Add(cases[name])
		}
	}
	inputs, _ := filepath.Glob("../testdata/*.md")
	for _, path := range inputs {
		if src, err := os.ReadFile(path); err == nil {
			f.Add(string(src))
		}
	}

	f.Fuzz(func(t *testing.T, src string) {
		// Control characters other than newline and tab are not meaningful
		// markdown and goldmark may normalise them inconsistently.
		if strings.ContainsFunc(src, func(r rune) bool { return r < 32 && r != '\n' && r != '\t' }) {
			t.Skip()
		}
		// mdminify guarantees a single trailing newline, so compare against
		// input that has one. goldmark parses an unterminated final line
		// differently from a terminated one — a fence's info string is
		// dropped, for instance — which would otherwise report that
		// guarantee as a contract violation.
		if !strings.HasSuffix(src, "\n") {
			src += "\n"
		}
		// goldmark cannot be the oracle where it contradicts itself.
		if goldmarkQuoteQuirk(t, src) {
			t.Skip()
		}
		var buf strings.Builder
		if err := minify.Minify(strings.NewReader(src), &buf); err != nil {
			t.Fatalf("Minify error: %v", err)
		}
		if !equalHTML(t, src, buf.String()) {
			t.Errorf("HTML changed\n--- input ---\n%q\n--- minified ---\n%q", src, buf.String())
		}
	})
}
