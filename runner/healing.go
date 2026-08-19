package runner

import (
	"strings"

	"golang.org/x/net/html"
)

// healThreshold is the minimum score a candidate element must reach before we
// trust it as a healed replacement for a missing selector.
const healThreshold = 3

// selectorParts is the original selector broken into matchable pieces.
type selectorParts struct {
	tag     string
	id      string
	classes []string
}

// parseSelector splits a CSS selector like "button#buy.cta.lg" into its tag,
// id, and class tokens. Tag/id/class are all optional.
func parseSelector(sel string) selectorParts {
	p := selectorParts{}
	var cur strings.Builder
	kind := "tag"
	flush := func() {
		s := cur.String()
		cur.Reset()
		switch kind {
		case "tag":
			if p.tag == "" {
				p.tag = s
			}
		case "id":
			p.id = s
		case "class":
			if s != "" {
				p.classes = append(p.classes, s)
			}
		}
	}
	for _, r := range sel {
		switch r {
		case '#':
			flush()
			kind = "id"
		case '.':
			flush()
			kind = "class"
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return p
}

// candidate is a DOM element scored as a possible replacement.
type candidate struct {
	tag     string
	id      string
	classes []string
	text    string
	score   int
}

// scoreCandidates walks the parsed DOM and returns the highest-scoring element
// against the original selector, plus whether anything was considered.
func scoreCandidates(doc *html.Node, sp selectorParts) (candidate, bool) {
	best := candidate{}
	found := false
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			c := candidate{tag: n.Data}
			for _, a := range n.Attr {
				switch a.Key {
				case "id":
					c.id = a.Val
				case "class":
					c.classes = strings.Fields(a.Val)
				}
			}
			c.text = strings.ToLower(visibleText(n))
			c.score = scoreOne(c, sp)
			if c.score > best.score {
				best = c
				found = true
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return best, found
}

// scoreOne applies the heuristic: +2 for matching tag, +3 for a shared class
// substring, +5 if the element's visible text contains a class token, +3 for a
// shared id substring, +5 if the text contains the id token.
func scoreOne(c candidate, sp selectorParts) int {
	score := 0
	if sp.tag != "" && sp.tag != "*" && c.tag == sp.tag {
		score += 2
	}
	for _, sc := range sp.classes {
		scL := strings.ToLower(sc)
		matched := false
		for _, cc := range c.classes {
			if strings.Contains(strings.ToLower(cc), scL) || strings.Contains(scL, strings.ToLower(cc)) {
				score += 3
				matched = true
				break
			}
		}
		if !matched && c.text != "" && strings.Contains(c.text, scL) {
			score += 5
		}
	}
	if sp.id != "" {
		idL := strings.ToLower(sp.id)
		if c.id != "" && (strings.Contains(strings.ToLower(c.id), idL) || strings.Contains(idL, strings.ToLower(c.id))) {
			score += 3
		}
		if c.text != "" && strings.Contains(c.text, idL) {
			score += 5
		}
	}
	return score
}

// candidateSelector builds a CSS selector for the chosen element. When the
// original selector named classes, we prefer a candidate class that shares a
// substring with one of them (that's the "renamed class" case); otherwise we
// prefer the id, then the first class, then the tag.
func candidateSelector(c candidate, sp selectorParts) string {
	if len(sp.classes) > 0 {
		for _, sc := range sp.classes {
			scL := strings.ToLower(sc)
			for _, cc := range c.classes {
				if strings.Contains(strings.ToLower(cc), scL) || strings.Contains(scL, strings.ToLower(cc)) {
					return "." + cc
				}
			}
		}
	}
	if c.id != "" {
		return "#" + c.id
	}
	if len(c.classes) > 0 {
		return "." + c.classes[0]
	}
	return c.tag
}

// visibleText recursively collects an element's text content, trimmed and
// collapsed to single spaces.
func visibleText(n *html.Node) string {
	var sb strings.Builder
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return strings.Join(strings.Fields(sb.String()), " ")
}
