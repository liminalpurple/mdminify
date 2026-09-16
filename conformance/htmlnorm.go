package conformance

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// normaliseHTML parses an HTML fragment and re-serialises it with whitespace
// in text nodes collapsed to single spaces, except inside <pre>, where
// whitespace is significant.
//
// This is the correct notion of equality for mdminify. Unwrapping a
// soft-wrapped paragraph replaces newlines with spaces inside a text node,
// which changes the HTML bytes but not the rendered document, because HTML
// collapses whitespace runs. Comparing raw bytes would reject the tool's
// primary transform; comparing normalised trees rejects only real changes.
func normaliseHTML(fragment string) (string, error) {
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(fragment), ctx)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, n := range nodes {
		collapse(n, false)
		if err := html.Render(&sb, n); err != nil {
			return "", err
		}
	}
	return sb.String(), nil
}

// collapse walks the tree collapsing whitespace runs in text nodes. inPre
// suppresses collapsing for the subtree beneath a <pre> element.
func collapse(n *html.Node, inPre bool) {
	if n.Type == html.TextNode && !inPre {
		n.Data = strings.Join(strings.Fields(n.Data), " ")
		return
	}
	if n.Type == html.ElementNode && n.Data == "pre" {
		inPre = true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collapse(c, inPre)
	}
}
