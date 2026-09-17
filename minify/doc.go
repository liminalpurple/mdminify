// Package minify transforms markdown documents into a more compact form
// without changing their rendered meaning.
//
// The contract is that minified output renders to the same HTML as the input.
// Everything else follows from it: leaving two lines alone always renders
// correctly, because a soft line break and a space are both just whitespace to
// a renderer, so rewriting is the only operation that can change a document.
// Every rewrite therefore needs a positive warrant, and anything uncertain is
// passed through untouched — an unmodelled construct costs compression rather
// than correctness.
//
// The package performs several transforms, each declining where it cannot be
// sure:
//
//   - Paragraph unwrapping: joins soft-wrapped lines within paragraphs, list
//     items, and blockquotes into single lines. Hard breaks (trailing backslash
//     or double space) are preserved, as are joins that would form a table,
//     complete a link reference definition, or close an HTML tag left open
//     across the break.
//   - Table squashing: strips cell padding, and reduces the delimiter row to
//     its minimal form (e.g. "|---|" becomes "|-|", "|:---:|" becomes "|:-:|").
//     Only that row: the same text in a header or body cell is content. A cell
//     holding only whitespace keeps one space, since an empty cell can lose the
//     column's alignment.
//   - Heading-to-block collapse: removes blank lines between headings (ATX,
//     setext, or bold-as-heading) and the block element that follows. Not
//     inside a list item, where a blank line marks the list loose.
//   - Blank line collapsing: consecutive blank lines become a single blank
//     line, except within a code block, where they are content.
//   - Trailing whitespace removal: stripped from all lines except intentional
//     hard line breaks.
//
// Content inside fenced code blocks, indented code blocks, YAML front matter,
// and HTML blocks passes through unmodified. So does any container whose
// indentation uses tabs, whose width depends on the column the tab lands in,
// and any blockquote mixing "> " and ">" markers, whose spelling is preserved
// rather than normalised.
//
// The entry point is [Minify], which reads from an [io.Reader] and writes to an
// [io.Writer]:
//
//	var buf strings.Builder
//	if err := minify.Minify(input, &buf); err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Print(buf.String())
package minify
