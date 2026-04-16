package minify

import "testing"

func TestSquashTableRow(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"| Name    | Age | City      |", "|Name|Age|City|"},
		{"|Name|Age|", "|Name|Age|"},
		{"| --- | --- | --- |", "|-|-|-|"},
		{"|:---|---:|:---:|", "|:-|-:|:-:|"},
		{"|:---------|----:|:--------:|", "|:-|-:|:-:|"},
		{"| a | b |", "|a|b|"},
		{`| a \| b | c |`, `|a \| b|c|`},
		{"| | |", "|||"},
	}
	for _, tt := range tests {
		got := squashTableRow(tt.input)
		if got != tt.want {
			t.Errorf("squashTableRow(%q) = %q, want %q", tt.input, got, tt.want)
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
