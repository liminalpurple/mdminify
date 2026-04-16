// Package minify transforms markdown documents into a more compact form
// without changing their rendered meaning.
//
// The package performs several transforms:
//
//   - Paragraph unwrapping: joins soft-wrapped lines within paragraphs, list
//     items, and blockquotes into single lines. Hard breaks (trailing backslash
//     or double space) are preserved.
//   - Table squashing: strips cell padding and reduces separator rows to their
//     minimal form (e.g. "|---|" becomes "|-|", "|:---:|" becomes "|:-:|").
//   - Heading-to-block collapse: removes blank lines between headings (ATX,
//     setext, or bold-as-heading) and the block element that follows.
//   - Blank line collapsing: consecutive blank lines become a single blank line.
//   - Trailing whitespace removal: stripped from all lines except intentional
//     hard line breaks.
//
// Content inside fenced code blocks, indented code blocks, YAML front matter,
// and HTML blocks passes through unmodified.
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
