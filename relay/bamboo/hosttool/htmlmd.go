package hosttool

import (
	"strings"

	"golang.org/x/net/html"
)

var skipTags = map[string]bool{
	"script": true, "style": true, "iframe": true, "object": true,
	"embed": true, "link": true, "meta": true, "noscript": true,
}

func htmlToMarkdown(raw string) string {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return extractVisibleText(raw)
	}
	var b strings.Builder
	walkHTML(&b, doc)
	return strings.TrimSpace(b.String())
}

func extractVisibleText(raw string) string {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return raw
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(b.String())
}

func walkHTML(b *strings.Builder, n *html.Node) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	if n.Type != html.ElementNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkHTML(b, c)
		}
		return
	}
	tag := strings.ToLower(n.Data)
	if skipTags[tag] {
		return
	}
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(tag[1] - '0')
		b.WriteString("\n")
		b.WriteString(strings.Repeat("#", level))
		b.WriteByte(' ')
		writeChildren(b, n)
		b.WriteString("\n")
	case "p":
		b.WriteString("\n")
		writeChildren(b, n)
		b.WriteString("\n")
	case "br":
		b.WriteString("\n")
	case "ul":
		b.WriteString("\n")
		writeChildren(b, n)
	case "ol":
		b.WriteString("\n")
		writeChildren(b, n)
	case "li":
		b.WriteString("- ")
		writeChildren(b, n)
		b.WriteString("\n")
	case "a":
		href := attr(n, "href")
		if href != "" && !strings.HasPrefix(strings.ToLower(href), "javascript:") {
			b.WriteByte('[')
			writeChildren(b, n)
			b.WriteString("](")
			b.WriteString(href)
			b.WriteByte(')')
		} else {
			writeChildren(b, n)
		}
	case "code":
		b.WriteByte('`')
		writeChildren(b, n)
		b.WriteByte('`')
	case "pre":
		b.WriteString("\n```\n")
		writeChildren(b, n)
		b.WriteString("\n```\n")
	case "blockquote":
		b.WriteString("\n> ")
		writeChildren(b, n)
		b.WriteString("\n")
	case "em", "i":
		b.WriteByte('*')
		writeChildren(b, n)
		b.WriteByte('*')
	case "strong", "b":
		b.WriteString("**")
		writeChildren(b, n)
		b.WriteString("**")
	case "table", "thead", "tbody":
		writeChildren(b, n)
		b.WriteByte('\n')
	case "tr":
		b.WriteByte('|')
		writeChildren(b, n)
		b.WriteString("\n")
	case "th", "td":
		b.WriteByte(' ')
		writeChildren(b, n)
		b.WriteString(" |")
	default:
		writeChildren(b, n)
	}
}

func writeChildren(b *strings.Builder, n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkHTML(b, c)
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}
