package minify

import "strings"

// htmlBlockTerminators returns the strings that close the HTML block opened by
// this line, and whether it opens one of the kinds that this function covers.
//
// CommonMark defines seven kinds of HTML block. Kinds 1 to 5 run until a
// closing string appears, passing through blank lines on the way, which is why
// they need tracking: a comment or a CDATA section may contain one. Kinds 6
// and 7 end at a blank line instead, so the ordinary line-by-line path already
// handles them and they are not reported here.
func htmlBlockTerminators(line string) ([]string, bool) {
	s := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(s, "<") {
		return nil, false
	}

	lower := strings.ToLower(s)
	for _, tag := range []string{"<pre", "<script", "<style", "<textarea"} {
		if !strings.HasPrefix(lower, tag) {
			continue
		}
		if rest := lower[len(tag):]; rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '>' {
			return []string{"</pre>", "</script>", "</style>", "</textarea>"}, true
		}
	}

	switch {
	case strings.HasPrefix(s, "<!--"):
		return []string{"-->"}, true
	case strings.HasPrefix(s, "<?"):
		return []string{"?>"}, true
	case strings.HasPrefix(s, "<![CDATA["):
		return []string{"]]>"}, true
	case len(s) > 2 && s[1] == '!' && isASCIILetter(s[2]):
		return []string{">"}, true
	}
	return nil, false
}

// isASCIILetter reports whether c is an unaccented Latin letter.
func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// containsAny reports whether s contains any of the given substrings, matched
// without regard to case so that closing tags such as </PRE> are recognised.
func containsAny(s string, subs []string) bool {
	lower := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// hasUnclosedTag reports whether the line opens an HTML tag that it does not
// close. A tag may span a line break, so joining such a line onto the next
// changes what the tag contains, and can complete a tag that was incomplete.
//
// Only a "<" that begins a tag counts, so ordinary prose comparing values with
// "a < b" still joins.
func hasUnclosedTag(line string) bool {
	for i := 0; i < len(line); i++ {
		if line[i] != '<' {
			continue
		}
		rest := line[i+1:]
		if rest == "" {
			continue
		}
		if !isASCIILetter(rest[0]) && rest[0] != '/' && rest[0] != '!' && rest[0] != '?' {
			continue
		}
		if !strings.ContainsRune(rest, '>') {
			return true
		}
	}
	return false
}
