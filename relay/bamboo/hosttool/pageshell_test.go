package hosttool

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const xlfSPAShell = `<!-- --------------------------------------------------------------------------------
Copyright (c) 2016-NOW(至今) 筱锋
Author: 筱锋「xiao_lfeng」(https://www.x-lf.com)
-------------------------------------------------------------------------------- -->
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="theme-color" content="#000000" />
    <meta name="description" content="Web site created using create-tsrouter-app" />
    <link rel="icon" href="/favicon.ico" />
    <link rel="manifest" href="/manifest.json" />
    <!-- 竹林水墨 · 衬线字体（Noto Serif SC 中文 + Source Serif 4 西文） -->
    <title>Create TanStack App - bamboo-main-frontend</title>
    <script type="module" crossorigin src="/assets/index-CezjbG19.js"></script>
    <link rel="modulepreload" crossorigin href="/assets/useStore-DLYuMt2P.js">
    <link rel="stylesheet" crossorigin href="/assets/index-DlaH9jWV.css">
  </head>
  <body>
    <div id="app"></div>
  </body>
</html>`

func TestComposeFetchedHTMLSPAShell(t *testing.T) {
	t.Parallel()
	got := composeFetchedHTML(xlfSPAShell, "markdown")
	assert.Contains(t, got, "# Create TanStack App - bamboo-main-frontend")
	assert.Contains(t, got, "Web site created using create-tsrouter-app")
	assert.Contains(t, got, "client-rendered SPA shell")
	assert.Contains(t, got, "empty mount #app")
	assert.Contains(t, got, "筱锋")
	assert.Contains(t, got, "https://www.x-lf.com")
	assert.Contains(t, got, "竹林水墨")
	assert.Contains(t, got, "/manifest.json")
	assert.Contains(t, got, "/assets/index-CezjbG19.js")
	assert.NotContains(t, got, "javascript:")
}

func TestComposeFetchedHTMLKeepsArticle(t *testing.T) {
	t.Parallel()
	raw := `<!doctype html><html><head><title>Docs</title></head>
<body><h1>Install</h1><p>Run bun install, then start the development server and open the dashboard.</p>
<p>More setup notes for operators who already have a token.</p></body></html>`
	got := composeFetchedHTML(raw, "markdown")
	assert.Contains(t, got, "# Install")
	assert.Contains(t, got, "bun install")
	assert.NotContains(t, got, "SPA shell")
	assert.NotContains(t, got, "## Resources")
}

func TestComposeFetchedHTMLKeepsShortArticle(t *testing.T) {
	t.Parallel()
	raw := `<html><head><title>Hi</title></head><body><h1>Hi</h1><p>Hello</p></body></html>`
	got := composeFetchedHTML(raw, "markdown")
	assert.Contains(t, got, "Hello")
	assert.NotContains(t, got, "SPA shell")
}

func TestComposeFetchedHTMLTextFormat(t *testing.T) {
	t.Parallel()
	got := composeFetchedHTML(xlfSPAShell, "text")
	assert.Contains(t, got, "title: Create TanStack App - bamboo-main-frontend")
	assert.Contains(t, got, "description: Web site created using create-tsrouter-app")
	assert.Contains(t, got, "note: client-rendered SPA shell")
}

func TestComposeFetchedHTMLRawPassthrough(t *testing.T) {
	t.Parallel()
	assert.Equal(t, xlfSPAShell, composeFetchedHTML(xlfSPAShell, "html"))
}

func TestComposeFetchedHTMLJSONLD(t *testing.T) {
	t.Parallel()
	raw := `<html><head><title>Shop</title>
<script type="application/ld+json">{"@type":"WebSite","name":"Shop"}</script>
</head><body><div id="root"></div></body></html>`
	got := composeFetchedHTML(raw, "markdown")
	assert.Contains(t, got, `"@type":"WebSite"`)
	assert.Contains(t, got, `"name":"Shop"`)
}

func TestApplyStartLine(t *testing.T) {
	t.Parallel()
	body := "a\nb\nc"
	assert.Equal(t, body, applyStartLine(body, 0))
	assert.Equal(t, body, applyStartLine(body, 1))
	assert.Equal(t, "b\nc", applyStartLine(body, 2))
	assert.Equal(t, "", applyStartLine(body, 9))
}

func TestApplyPatternFind(t *testing.T) {
	t.Parallel()
	body := strings.Join([]string{"alpha", "bravo", "charlie", "delta", "echo"}, "\n")
	got := applyPatternFind(body, "charlie", 10, 1)
	assert.Contains(t, got, "2: bravo")
	assert.Contains(t, got, "3: charlie")
	assert.Contains(t, got, "4: delta")
	assert.NotContains(t, got, "alpha")
	assert.Equal(t, "0 matches", applyPatternFind(body, "zzz", 10, 1))
	assert.True(t, strings.HasPrefix(applyPatternFind(body, "(", 10, 1), "pattern_error:"))
}

func TestApplyFetchViewPrefersPattern(t *testing.T) {
	t.Parallel()
	body := "one\ntwo\nthree"
	got := applyFetchView(body, 3, "one", 10, 0)
	assert.Contains(t, got, "1: one")
	assert.NotEqual(t, "three", got)
}

func TestExtractHTMLShellIgnoresIEComments(t *testing.T) {
	t.Parallel()
	raw := `<!--[if IE]>oldie<![endif]--><html><head><title>T</title></head><body></body></html>`
	shell := extractHTMLShell(raw)
	assert.Equal(t, "T", shell.Title)
	assert.Empty(t, shell.Comments)
}

func TestFirstPositiveInt(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 4, firstPositiveInt(0, "x", 4.0))
	assert.Equal(t, 0, firstPositiveInt(-1, 0))
}

func TestInspectFetchSchemaMentionsPattern(t *testing.T) {
	t.Parallel()
	require.Contains(t, string(fetchSchemaJSON), `"pattern"`)
	require.Contains(t, string(fetchSchemaJSON), `"start_line"`)
	assert.Contains(t, fetchDescription, "SPA")
}
