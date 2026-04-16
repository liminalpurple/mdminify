package minify

import (
	"strings"
	"testing"
)

func TestParagraphFlush(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{
			name:  "single line",
			lines: []string{"hello world"},
			want:  []string{"hello world"},
		},
		{
			name:  "soft wrapped paragraph",
			lines: []string{"hello", "world", "foo"},
			want:  []string{"hello world foo"},
		},
		{
			name:  "hard break with trailing spaces",
			lines: []string{"hello  ", "world"},
			want:  []string{"hello  ", "world"},
		},
		{
			name:  "hard break with backslash",
			lines: []string{"hello\\", "world"},
			want:  []string{"hello\\", "world"},
		},
		{
			name:  "list items stay separate",
			lines: []string{"- item one", "- item two"},
			want:  []string{"- item one", "- item two"},
		},
		{
			name:  "list item with continuation",
			lines: []string{"- item one", "continues here", "- item two"},
			want:  []string{"- item one continues here", "- item two"},
		},
		{
			name:  "heading interrupts",
			lines: []string{"para text", "# heading"},
			want:  []string{"para text", "# heading"},
		},
		{
			name:  "setext underline stays separate",
			lines: []string{"Heading", "==="},
			want:  []string{"Heading", "==="},
		},
		{
			name:  "blockquote starts new line",
			lines: []string{"text", "> quote"},
			want:  []string{"text", "> quote"},
		},
		{
			name:  "trailing whitespace stripped",
			lines: []string{"hello   "},
			want:  []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pb paragraphBuffer
			for _, l := range tt.lines {
				pb.add(l)
			}
			got := pb.flush()
			if len(got) != len(tt.want) {
				t.Fatalf("flush() returned %d lines, want %d\ngot:  %s\nwant: %s",
					len(got), len(tt.want), strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
