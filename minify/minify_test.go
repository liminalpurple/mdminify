package minify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenFiles(t *testing.T) {
	matches, err := filepath.Glob("../testdata/*.expected.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no golden files found in testdata/")
	}

	for _, expectedPath := range matches {
		base := strings.TrimSuffix(filepath.Base(expectedPath), ".expected.md")
		inputPath := filepath.Join(filepath.Dir(expectedPath), base+".md")

		t.Run(base, func(t *testing.T) {
			input, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("reading input: %v", err)
			}
			expected, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("reading expected: %v", err)
			}

			var buf strings.Builder
			if err := Minify(strings.NewReader(string(input)), &buf); err != nil {
				t.Fatalf("Minify error: %v", err)
			}
			got := buf.String()
			want := string(expected)

			// Normalise: ensure both end with a single newline for comparison.
			got = strings.TrimRight(got, "\n") + "\n"
			want = strings.TrimRight(want, "\n") + "\n"

			if got != want {
				t.Errorf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", base, got, want)
			}
		})
	}
}
