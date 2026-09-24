package normalize

import (
	"strings"

	"golang.org/x/net/html"
)

// blocks are the elements that start a new line in plain text.
var blocks = map[string]bool{
	"p": true, "br": true, "div": true, "li": true, "ul": true, "ol": true, "tr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

// Text renders an HTML fragment as plain text: tags dropped, entities
// decoded, a line break at each block element, whitespace within a line
// collapsed, and blank lines removed.
func Text(fragment string) string {
	z := html.NewTokenizer(strings.NewReader(fragment))
	var b strings.Builder
	for {
		switch z.Next() {
		case html.ErrorToken:
			return tidy(b.String())
		case html.TextToken:
			b.WriteString(strings.ReplaceAll(string(z.Text()), "\n", " "))
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			if name, _ := z.TagName(); blocks[string(name)] {
				b.WriteByte('\n')
			}
		}
	}
}

// tidy collapses whitespace within each line and drops empty lines.
func tidy(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if f := strings.Join(strings.Fields(line), " "); f != "" {
			lines = append(lines, f)
		}
	}
	return strings.Join(lines, "\n")
}
