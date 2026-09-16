# mdminify

[![Go Reference](https://pkg.go.dev/badge/github.com/liminalpurple/mdminify.svg)](https://pkg.go.dev/github.com/liminalpurple/mdminify)
[![CI](https://github.com/liminalpurple/mdminify/actions/workflows/ci.yml/badge.svg)](https://github.com/liminalpurple/mdminify/actions/workflows/ci.yml)

A small, zero-dependency Go tool that minifies markdown files without changing their rendered
meaning. It unwraps soft-wrapped paragraphs into single lines, squashes padded tables down to
minimal width, and collapses unnecessary blank lines.

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
  into a single line. Hard breaks (trailing `\` or double space) are preserved.
- **Table squashing** — strips cell padding and reduces separator rows to their minimal form (`-`,
  `:-`, `-:`, `:-:`).
- **Heading-to-block collapse** — removes blank lines between headings and the block element that
  follows (lists, blockquotes, tables). Also recognises bold-as-heading lines like
  `**Section title:**`.
- **Blank line collapsing** — consecutive blank lines become a single blank line.
- **Trailing whitespace** — stripped from all lines except intentional hard breaks.

### What it won't touch

Code blocks (fenced and indented), YAML front matter, and HTML blocks pass through unmodified.
Setext heading underlines, link reference definitions, and hard line breaks are all preserved.

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

It also carries a fuzz target, which is worth running when changing how lines are classified or
joined:

```bash
cd conformance && go test -fuzz FuzzHTMLEquivalence
```

## Licence

Apache 2.0 — see [LICENSE](LICENSE).
