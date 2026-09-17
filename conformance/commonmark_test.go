package conformance

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// specExample is one case from the CommonMark specification.
//
// Only the markdown is kept. The specification's own expected HTML is not
// asserted here: mdminify does not claim to be a renderer, and the contract
// under test is that minifying an input does not change what it renders to.
// The suite is used as a corpus of markdown that exercises constructs and
// edge cases far beyond anything we would think to write by hand.
type specExample struct {
	Example  int    `json:"example"`
	Section  string `json:"section"`
	Markdown string `json:"markdown"`
}

func loadSpecExamples(t *testing.T) []specExample {
	t.Helper()
	raw, err := os.ReadFile("testdata/commonmark.json")
	if err != nil {
		t.Fatalf("reading spec corpus: %v", err)
	}
	var examples []specExample
	if err := json.Unmarshal(raw, &examples); err != nil {
		t.Fatalf("parsing spec corpus: %v", err)
	}
	if len(examples) == 0 {
		t.Fatal("spec corpus is empty")
	}
	return examples
}

// specKnownBroken lists spec examples that currently violate the contract,
// keyed by example number. Entries must name their cause; the test asserts
// they still fail, so a fix is reported rather than passing silently.
var specKnownBroken = map[int]string{
	616: "a raw HTML tag left open across a line break, where the unclosed tag is masked " +
		"from hasUnclosedTag by a '>' belonging to a tag nested inside a quoted attribute " +
		"value; separating them needs HTML tokenisation",
}

// The three above share a cause: this is a line-based minifier, and an inline
// construct that spans a line break can parse differently once the break
// becomes a space. Realistic forms of all three, including multi-line img tags
// and a definition with its title on the next line, are covered in
// equivalence_test.go and hold.

// TestCommonMarkSpec checks every example in the CommonMark specification
// against the HTML-equivalence contract.
func TestCommonMarkSpec(t *testing.T) {
	for _, ex := range loadSpecExamples(t) {
		name := fmt.Sprintf("%03d_%s", ex.Example, strings.ReplaceAll(ex.Section, " ", "_"))
		t.Run(name, func(t *testing.T) {
			ok := checkEquivalent(t, ex.Markdown)
			reason, broken := specKnownBroken[ex.Example]

			switch {
			case ok && broken:
				t.Errorf("example is in specKnownBroken but now passes — remove it from the map.\nrecorded cause: %s", reason)
			case !ok && broken:
				t.Logf("known failure: %s", reason)
			case !ok:
				t.Errorf("minifying this example changed the rendered HTML")
			}
		})
	}
}

// TestCommonMarkSpecIdempotent checks that minifying a spec example twice
// gives the same result as minifying it once.
func TestCommonMarkSpecIdempotent(t *testing.T) {
	for _, ex := range loadSpecExamples(t) {
		once := minified(t, ex.Markdown)
		twice := minified(t, once)
		if once != twice {
			t.Errorf("example %d (%s) is not idempotent\ninput: %q\nonce:  %q\ntwice: %q",
				ex.Example, ex.Section, ex.Markdown, once, twice)
		}
	}
}
