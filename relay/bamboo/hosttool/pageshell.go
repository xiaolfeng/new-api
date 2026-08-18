package hosttool

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/net/html"
)

const (
	spaVisibleRunes = 80
	maxComments     = 8
	maxCommentRunes = 2000
	maxJSONBlocks   = 3
	maxJSONRunes    = 4000
	maxResources    = 40
	defaultFindMax  = 50
	hardFindMax     = 200
	defaultFindCtx  = 10
	hardFindCtx     = 20
)

type htmlShell struct {
	Title       string
	Description string
	Canonical   string
	SiteName    string
	MountIDs    []string
	Comments    []string
	Resources   []string
	JSONBlocks  []string
	Noscript    string
	HasModule   bool
}

func composeFetchedHTML(raw, format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "html" {
		return raw
	}
	converted := convertHTMLBody(raw, format)
	shell := extractHTMLShell(raw)
	if !shouldAttachShell(converted, shell) {
		return converted
	}
	return attachShell(shell, converted, format == "text")
}

func convertHTMLBody(raw, format string) string {
	if format == "text" {
		return extractVisibleText(raw)
	}
	return htmlToMarkdown(raw)
}

func shouldAttachShell(visible string, shell htmlShell) bool {
	if shell.isSPA(visible) {
		return true
	}
	return strings.TrimSpace(visible) == "" || sameVisibleTitle(visible, shell.Title)
}

func (s htmlShell) isSPA(visible string) bool {
	if !s.HasModule && len(s.MountIDs) == 0 {
		return false
	}
	if sameVisibleTitle(visible, s.Title) {
		return true
	}
	return thinText(visible, spaVisibleRunes)
}

func extractHTMLShell(raw string) htmlShell {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return htmlShell{}
	}
	var s htmlShell
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.CommentNode {
			if c := normalizeComment(n.Data); c != "" && len(s.Comments) < maxComments {
				s.Comments = append(s.Comments, c)
			}
			return
		}
		if n.Type != html.ElementNode {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			return
		}
		switch strings.ToLower(n.Data) {
		case "title":
			if s.Title == "" {
				s.Title = strings.TrimSpace(directText(n))
			}
			return
		case "meta":
			collectMeta(&s, n)
			return
		case "link":
			collectLink(&s, n)
			return
		case "script":
			collectScript(&s, n)
			return
		case "noscript":
			if s.Noscript == "" {
				s.Noscript = strings.TrimSpace(directText(n))
			}
			return
		case "div", "main", "app":
			if id := attr(n, "id"); isMountID(id) && !hasVisibleText(n) {
				s.MountIDs = appendUniq(s.MountIDs, id)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return s
}

func collectMeta(s *htmlShell, n *html.Node) {
	name := strings.ToLower(firstNonEmpty(attr(n, "name"), attr(n, "property"), attr(n, "itemprop")))
	content := strings.TrimSpace(attr(n, "content"))
	if name == "" || content == "" {
		return
	}
	switch name {
	case "description", "og:description", "twitter:description":
		if s.Description == "" {
			s.Description = content
		}
	case "og:title", "twitter:title":
		if s.Title == "" {
			s.Title = content
		}
	case "og:url":
		if s.Canonical == "" {
			s.Canonical = content
		}
	case "og:site_name", "application-name":
		if s.SiteName == "" {
			s.SiteName = content
		}
	}
}

func collectLink(s *htmlShell, n *html.Node) {
	href := strings.TrimSpace(attr(n, "href"))
	if href == "" || strings.HasPrefix(strings.ToLower(href), "javascript:") {
		return
	}
	rel := strings.ToLower(attr(n, "rel"))
	if strings.Contains(rel, "canonical") && s.Canonical == "" {
		s.Canonical = href
	}
	if !interestingRel(rel) {
		return
	}
	if len(s.Resources) < maxResources {
		s.Resources = appendUniq(s.Resources, href)
	}
}

func collectScript(s *htmlShell, n *html.Node) {
	typ := strings.ToLower(strings.TrimSpace(attr(n, "type")))
	src := strings.TrimSpace(attr(n, "src"))
	if typ == "module" {
		s.HasModule = true
	}
	if src != "" && !strings.HasPrefix(strings.ToLower(src), "javascript:") {
		if len(s.Resources) < maxResources {
			s.Resources = appendUniq(s.Resources, src)
		}
	}
	if typ != "application/ld+json" && typ != "application/json" {
		return
	}
	if len(s.JSONBlocks) >= maxJSONBlocks {
		return
	}
	if block := compactJSONText(directText(n)); block != "" {
		s.JSONBlocks = append(s.JSONBlocks, block)
	}
}

func interestingRel(rel string) bool {
	for _, key := range []string{"icon", "manifest", "canonical", "alternate", "modulepreload", "stylesheet"} {
		if strings.Contains(rel, key) {
			return true
		}
	}
	return false
}

func isMountID(id string) bool {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "app", "root", "__next", "__nuxt", "app-root":
		return true
	default:
		return false
	}
}

func attachShell(shell htmlShell, visible string, plain bool) string {
	var b strings.Builder
	title := strings.TrimSpace(shell.Title)
	if title != "" {
		if plain {
			fmt.Fprintf(&b, "title: %s\n", title)
		} else {
			fmt.Fprintf(&b, "# %s\n\n", title)
		}
	}
	if d := strings.TrimSpace(shell.Description); d != "" {
		if plain {
			fmt.Fprintf(&b, "description: %s\n", d)
		} else {
			fmt.Fprintf(&b, "%s\n", d)
		}
	}
	if name := strings.TrimSpace(shell.SiteName); name != "" {
		fmt.Fprintf(&b, "site: %s\n", name)
	}
	if canon := strings.TrimSpace(shell.Canonical); canon != "" {
		fmt.Fprintf(&b, "canonical: %s\n", canon)
	}
	if shell.isSPA(visible) {
		note := "client-rendered SPA shell; HTTP HTML has no visible article, navigation, or buttons"
		if len(shell.MountIDs) > 0 {
			note += "; empty mount #" + strings.Join(shell.MountIDs, ", #")
		}
		if plain {
			fmt.Fprintf(&b, "note: %s\n", note)
		} else {
			fmt.Fprintf(&b, "\n> %s\n", note)
		}
	}
	writeShellBlock(&b, "HTML comments", shell.Comments, plain)
	if ns := strings.TrimSpace(shell.Noscript); ns != "" {
		writeShellBlock(&b, "noscript", []string{ns}, plain)
	}
	if len(shell.Resources) > 0 {
		if plain {
			b.WriteString("resources:\n")
			for _, r := range shell.Resources {
				fmt.Fprintf(&b, "- %s\n", r)
			}
		} else {
			b.WriteString("\n## Resources\n\n")
			for _, r := range shell.Resources {
				fmt.Fprintf(&b, "- %s\n", r)
			}
		}
	}
	writeShellBlock(&b, "Embedded JSON", shell.JSONBlocks, plain)
	body := strings.TrimSpace(visible)
	if sameVisibleTitle(body, title) {
		body = ""
	}
	if body != "" {
		b.WriteByte('\n')
		b.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

func writeShellBlock(b *strings.Builder, heading string, items []string, plain bool) {
	if len(items) == 0 {
		return
	}
	if plain {
		fmt.Fprintf(b, "%s:\n", strings.ToLower(heading))
		for _, item := range items {
			fmt.Fprintf(b, "%s\n", item)
		}
		return
	}
	fmt.Fprintf(b, "\n## %s\n\n```\n%s\n```\n", heading, strings.Join(items, "\n\n"))
}

func applyFetchView(body string, startLine int, pattern string, maxMatches, contextLines int) string {
	if strings.TrimSpace(pattern) != "" {
		return applyPatternFind(body, pattern, maxMatches, contextLines)
	}
	return applyStartLine(body, startLine)
}

func applyStartLine(body string, startLine int) string {
	if startLine <= 1 || body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	if startLine > len(lines) {
		return ""
	}
	return strings.Join(lines[startLine-1:], "\n")
}

func applyPatternFind(body, pattern string, maxMatches, contextLines int) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return body
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return "pattern_error: invalid regex\n" + body
	}
	if maxMatches <= 0 {
		maxMatches = defaultFindMax
	}
	if maxMatches > hardFindMax {
		maxMatches = hardFindMax
	}
	if contextLines <= 0 {
		contextLines = defaultFindCtx
	}
	if contextLines > hardFindCtx {
		contextLines = hardFindCtx
	}
	lines := strings.Split(body, "\n")
	type window struct{ lo, hi int }
	var wins []window
	matched := 0
	for i, line := range lines {
		if !re.MatchString(line) {
			continue
		}
		matched++
		if matched > maxMatches {
			break
		}
		lo := i - contextLines
		if lo < 0 {
			lo = 0
		}
		hi := i + contextLines
		if hi >= len(lines) {
			hi = len(lines) - 1
		}
		if n := len(wins); n > 0 && lo <= wins[n-1].hi+1 {
			if hi > wins[n-1].hi {
				wins[n-1].hi = hi
			}
			continue
		}
		wins = append(wins, window{lo: lo, hi: hi})
	}
	if len(wins) == 0 {
		return "0 matches"
	}
	var b strings.Builder
	for i, w := range wins {
		if i > 0 {
			b.WriteString("\n---\n")
		}
		for ln := w.lo; ln <= w.hi; ln++ {
			fmt.Fprintf(&b, "%d: %s\n", ln+1, lines[ln])
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func normalizeComment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "[if") {
		return ""
	}
	runes := []rune(s)
	if len(runes) > maxCommentRunes {
		return string(runes[:maxCommentRunes]) + "..."
	}
	return s
}

func compactJSONText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var v any
	if err := common.Unmarshal([]byte(s), &v); err != nil {
		return clipRunes(s, maxJSONRunes)
	}
	raw, err := common.Marshal(v)
	if err != nil {
		return clipRunes(s, maxJSONRunes)
	}
	return clipRunes(string(raw), maxJSONRunes)
}

func clipRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func directText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func hasVisibleText(n *html.Node) bool {
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found || n == nil {
			return
		}
		if n.Type == html.ElementNode && skipTags[strings.ToLower(n.Data)] {
			return
		}
		if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
			found = true
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	return found
}

func sameVisibleTitle(visible, title string) bool {
	v := strings.Join(strings.Fields(visible), " ")
	t := strings.Join(strings.Fields(title), " ")
	return t != "" && strings.EqualFold(v, t)
}

func thinText(s string, limit int) bool {
	return utf8.RuneCountInString(strings.TrimSpace(s)) < limit
}

func appendUniq(dst []string, v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return dst
	}
	for _, item := range dst {
		if item == v {
			return dst
		}
	}
	return append(dst, v)
}
