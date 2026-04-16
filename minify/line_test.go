package minify

import (
	"testing"
)

func TestIsBlank(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"", true},
		{"   ", true},
		{"\t", true},
		{"hello", false},
		{"  x", false},
	}
	for _, tt := range tests {
		if got := isBlank(tt.line); got != tt.want {
			t.Errorf("isBlank(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestIsOpeningFence(t *testing.T) {
	tests := []struct {
		line string
		want bool
		char byte
		n    int
	}{
		{"```", true, '`', 3},
		{"````", true, '`', 4},
		{"~~~", true, '~', 3},
		{"  ```", true, '`', 3},
		{"   ~~~python", true, '~', 3},
		{"```python", true, '`', 3},
		{"```py`thon", false, 0, 0}, // backtick in info string
		{"``", false, 0, 0},         // too few
		{"    ```", false, 0, 0},    // 4 spaces = indented code
		{"~~~js", true, '~', 3},
		{"hello", false, 0, 0},
	}
	for _, tt := range tests {
		fi, ok := isOpeningFence(tt.line)
		if ok != tt.want {
			t.Errorf("isOpeningFence(%q) ok = %v, want %v", tt.line, ok, tt.want)
			continue
		}
		if ok {
			if fi.char != tt.char {
				t.Errorf("isOpeningFence(%q) char = %c, want %c", tt.line, fi.char, tt.char)
			}
			if fi.count != tt.n {
				t.Errorf("isOpeningFence(%q) count = %d, want %d", tt.line, fi.count, tt.n)
			}
		}
	}
}

func TestIsClosingFence(t *testing.T) {
	fi := fenceInfo{char: '`', count: 3, indent: 0}
	tests := []struct {
		line string
		fi   fenceInfo
		want bool
	}{
		{"```", fi, true},
		{"````", fi, true},
		{"  ```", fi, true},
		{"```  ", fi, true},
		{"~~~", fi, false},   // wrong char
		{"``", fi, false},    // too few
		{"``` x", fi, false}, // trailing non-space
		{"~~~", fenceInfo{char: '~', count: 3}, true},
		{"~~~~", fenceInfo{char: '~', count: 4}, true},
		{"~~~", fenceInfo{char: '~', count: 4}, false}, // too few
	}
	for _, tt := range tests {
		if got := isClosingFence(tt.line, tt.fi); got != tt.want {
			t.Errorf("isClosingFence(%q, %+v) = %v, want %v", tt.line, tt.fi, got, tt.want)
		}
	}
}

func TestIsATXHeading(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"# Hello", true},
		{"## Hello", true},
		{"###### Hello", true},
		{"####### Hello", false}, // 7 hashes
		{"#Hello", false},        // no space
		{"#", true},              // empty heading
		{" # Hello", true},       // leading space
		{"hello", false},
	}
	for _, tt := range tests {
		if got := isATXHeading(tt.line); got != tt.want {
			t.Errorf("isATXHeading(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestIsSetextUnderline(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"===", true},
		{"---", true},
		{"=", true},
		{"-", true},
		{"  ===  ", true},
		{"=-=", false},
		{"", false},
		{"abc", false},
	}
	for _, tt := range tests {
		if got := isSetextUnderline(tt.line); got != tt.want {
			t.Errorf("isSetextUnderline(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestIsThematicBreak(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"---", true},
		{"***", true},
		{"___", true},
		{"- - -", true},
		{"* * *", true},
		{" ---", true},
		{"--", false},
		{"abc", false},
	}
	for _, tt := range tests {
		if got := isThematicBreak(tt.line); got != tt.want {
			t.Errorf("isThematicBreak(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestIsTableRow(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"| a | b |", true},
		{"|a|b|", true},
		{"  | a | b |", true},
		{"a | b", false}, // no leading |
		{"", false},
	}
	for _, tt := range tests {
		if got := isTableRow(tt.line); got != tt.want {
			t.Errorf("isTableRow(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestIsTableSeparator(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"|---|---|", true},
		{"| --- | --- |", true},
		{"|:---|---:|", true},
		{"|:-:|:-:|", true},
		{"|-|", true},
		{"| a | b |", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isTableSeparator(tt.line); got != tt.want {
			t.Errorf("isTableSeparator(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestSplitTableCells(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"| a | b | c |", []string{" a ", " b ", " c "}},
		{"|a|b|", []string{"a", "b"}},
		{`| a \| b | c |`, []string{` a \| b `, ` c `}}, // escaped pipe — cell whitespace preserved, caller trims
	}
	for _, tt := range tests {
		got := splitTableCells(tt.line)
		if len(got) != len(tt.want) {
			t.Errorf("splitTableCells(%q) = %v, want %v", tt.line, got, tt.want)
			continue
		}
		for i := range got {
			// Trim leading space from split for comparison
			if got[i] != tt.want[i] {
				t.Errorf("splitTableCells(%q)[%d] = %q, want %q", tt.line, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsListMarker(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"- item", true},
		{"* item", true},
		{"+ item", true},
		{"1. item", true},
		{"99. item", true},
		{"1) item", true},
		{"  - item", true},
		{"   - item", true},
		{"    - item", false}, // 4 spaces = indented code
		{"-item", false},      // no space after marker
		{"hello", false},
	}
	for _, tt := range tests {
		if got := isListMarker(tt.line); got != tt.want {
			t.Errorf("isListMarker(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestIsBlockquotePrefix(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"> hello", true},
		{">hello", true},
		{"  > hello", true},
		{"hello", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isBlockquotePrefix(tt.line); got != tt.want {
			t.Errorf("isBlockquotePrefix(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestHasHardBreak(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"hello  ", true},
		{"hello\\", true},
		{"hello", false},
		{"hello ", false}, // only one space
		{"", false},
	}
	for _, tt := range tests {
		if got := hasHardBreak(tt.line); got != tt.want {
			t.Errorf("hasHardBreak(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestTrimTrailingWhitespace(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"hello ", "hello"},        // single trailing space — stripped
		{"hello  ", "hello  "},     // hard break (2 spaces) — preserved
		{"hello   ", "hello  "},    // hard break (3 spaces) — normalised to 2
		{"hello\\", "hello\\"},     // backslash hard break
		{"hello\t", "hello"},       // trailing tab — stripped
		{"  hello  ", "  hello  "}, // hard break preserved, leading untouched
	}
	for _, tt := range tests {
		if got := trimTrailingWhitespace(tt.line); got != tt.want {
			t.Errorf("trimTrailingWhitespace(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestIsBoldAsHeading(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"**Bold heading**", true},
		{"__Bold heading__", true},
		{"**Bold heading:**", true},
		{"**Bold heading**:", true},
		{"**Bold heading**;", true},
		{"  **Bold heading**  ", true},
		{"**x**", true},
		{"**Not bold** and more text.", false},
		{"Some **bold** text.", false},
		{"****", false}, // empty inner
		{"**", false},
		{"plain text", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isBoldAsHeading(tt.line); got != tt.want {
			t.Errorf("isBoldAsHeading(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestIsLinkRefDef(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{`[id]: http://example.com "Title"`, true},
		{`[id]: http://example.com`, true},
		{`  [id]: url`, true},
		{`[no colon]`, false},
		{`hello`, false},
	}
	for _, tt := range tests {
		if got := isLinkRefDef(tt.line); got != tt.want {
			t.Errorf("isLinkRefDef(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}
