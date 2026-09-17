# mdminify

[![Go Reference](https://pkg.go.dev/badge/github.com/liminalpurple/mdminify.svg)](https://pkg.go.dev/github.com/liminalpurple/mdminify)
[![CI](https://github.com/liminalpurple/mdminify/actions/workflows/ci.yml/badge.svg)](https://github.com/liminalpurple/mdminify/actions/workflows/ci.yml)

A small, zero-dependency Go tool that minifies markdown files without changing how they render.
It unwraps soft-wrapped paragraphs, strips padding from tables, and removes blank lines that carry
no meaning.

The guarantee is a specific one: **minified output renders to the same HTML as the input.** Where
`mdminify` cannot be certain a rewrite is safe, it leaves the text exactly as it found it — so a
construct it does not recognise costs you some compression, never correctness.

## What it does

Given markdown like this:

```markdown
## Heading

| Name    | Age | City      |
|---------|-----|-----------|
| Alice   | 30  | London    |

This paragraph wraps
across multiple lines
for readability.
```

`mdminify` produces:

```markdown
## Heading
|Name|Age|City|
|-|-|-|
|Alice|30|London|

This paragraph wraps across multiple lines for readability.
```

The rendered output is identical. The file is just smaller.

### Transforms

- **Paragraph unwrapping** — joins soft-wrapped lines within a paragraph, list item, or blockquote
  into a single line. A join is made only where it is provably safe; hard breaks (trailing `\` or
  double space), table rows, link reference definitions, HTML tags spanning a line break, and
  indented content all stop one.
- **Table squashing** — strips cell padding. Alignment markers in the delimiter row reduce to their
  minimal form (`-`, `:-`, `-:`, `:-:`); the same text in a header or body cell is content and is
  left alone, so a table documenting markdown syntax survives. A cell holding only whitespace keeps
  one space, because emptying it can drop the column's alignment.
- **Heading-to-block collapse** — removes the blank line between a heading and the block that
  follows it (lists, blockquotes, tables), including bold-as-heading lines like
  `**Section title:**`. Not applied inside a list item, where that blank line makes the list *loose*
  and so changes how every item renders.
- **Blank line collapsing** — consecutive blank lines become a single blank line, except inside a
  code block, where they are content.
- **Trailing whitespace** — stripped, unless it forms a hard break.

### What it won't touch

Code blocks (fenced and indented), YAML front matter, and HTML blocks pass through unmodified, as
do setext heading underlines, link reference definitions, and hard line breaks.

Beyond those, `mdminify` declines to rewrite anything it cannot measure confidently:

- a container indented with tabs, since a tab's width depends on the column it lands in, and moving
  a marker in front of one changes how much indentation it represents
- a blockquote mixing `>` and `> ` markers — either spelling is preserved as written, never
  normalised to the other
- a list marker whose content offset it cannot determine

Each of these costs bytes rather than correctness, which is the trade the tool makes everywhere.

## Install

```sh
go install github.com/liminalpurple/mdminify@latest
```

Or build from source:

```sh
git clone https://github.com/liminalpurple/mdminify.git
cd mdminify
go build -o mdminify .
```

## Usage

```
mdminify [flags] [path...]
```

With no arguments, it reads from stdin and writes to stdout:

```sh
cat README.md | mdminify
```

Pass one or more files to minify them:

```sh
mdminify docs/guide.md
```

Pass a directory to process all markdown files recursively:

```sh
mdminify -w ./docs/
```

### Flags

| Flag | Description |
|-|-|
| `-w` | Write changes back to the source files (only writes if content changed) |
| `-check` | Exit with code 1 if any file would be modified — useful for CI or pre-commit hooks |
| `-ext` | File extensions to match in directory mode (default `.md,.markdown`) |
| `-v` | Print version |

Both `-w` and `-check` compare the minified output against the original before doing anything —
files that are already minimal are left untouched. This makes `mdminify` safe to use in pre-commit
hooks or CI pipelines without triggering spurious file modifications.

### As a library

The `minify` package is importable if you want to use it programmatically:

```go
import "github.com/liminalpurple/mdminify/minify"

var buf strings.Builder
err := minify.Minify(reader, &buf)
```

## Testing

`go test ./...` covers the minifier itself. The contract that minified output renders to identical
HTML is checked separately, by the `conformance` package:

```bash
cd conformance && go test ./...
```

That suite renders both the input and the minified output through a CommonMark parser and compares
the results, so it catches whole classes of breakage rather than the specific cases someone thought
to write down. It lives in its own Go module so that its renderer dependency stays out of
`mdminify`, which has none. Both suites run from the pre-commit hooks and in CI.

It holds three things beyond the hand-written cases:

- the **CommonMark spec suite** — every example from the specification, run through the same
  equivalence check
- a **container parity matrix**, which crosses the constructs that have caused bugs with every
  container nesting, so a rule found for a blockquote is tested against a list item and vice versa
- a **fuzz target** (below)

Cases that are known to fail are recorded in `knownBroken` maps with their cause. Those assert the
case *still* fails, so fixing one is reported rather than passing silently.

It also carries a fuzz target, which is worth running when changing how lines are classified or
joined:

```bash
cd conformance && go test -fuzz FuzzHTMLEquivalence
```

## Licence

Apache 2.0 — see [LICENSE](LICENSE).
