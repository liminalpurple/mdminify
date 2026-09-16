package minify

import "testing"

func TestSquashTableRow(t *testing.T) {
	tests := []struct {
		input     string
		separator bool
		want      string
	}{
		{"| Name    | Age | City      |", false, "|Name|Age|City|"},
		{"|Name|Age|", false, "|Name|Age|"},
		{"| --- | --- | --- |", true, "|-|-|-|"},
		{"|:---|---:|:---:|", true, "|:-|-:|:-:|"},
		{"|:---------|----:|:--------:|", true, "|:-|-:|:-:|"},
		{"| a | b |", false, "|a|b|"},
		{`| a \| b | c |`, false, `|a \| b|c|`},
		{"| | |", false, "|||"},
		// Separator-shaped text in a body cell is content, not alignment.
		{"| --- | thematic break |", false, "|---|thematic break|"},
		{"| :-: | centred |", false, "|:-:|centred|"},
	}
	for _, tt := range tests {
		got := squashTableRow(tt.input, tt.separator)
		if got != tt.want {
			t.Errorf("squashTableRow(%q, %v) = %q, want %q", tt.input, tt.separator, got, tt.want)
		}
	}
}

func TestMinimalSeparator(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"---", "-"},
		{":---", ":-"},
		{"---:", "-:"},
		{":---:", ":-:"},
		{"-", "-"},
		{":-:", ":-:"},
	}
	for _, tt := range tests {
		got := minimalSeparator(tt.input)
		if got != tt.want {
			t.Errorf("minimalSeparator(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
