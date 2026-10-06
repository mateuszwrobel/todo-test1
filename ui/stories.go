package ui

// stories.go — the /__components story gallery: one page showing the
// observable ui states as real template renderings against deterministic
// in-code fixtures. It is the ui module's visual test surface: later pixel
// tests screenshot each state container by id (#state-*) and every byte of
// markup is decided at compile time — no store, no api, no clock, no
// randomness. This file is test-support-shaped but production-compiled (it
// serves a real page), and it is stdlib-only like the rest of ui: the
// gallery never touches p.loadBoard or any other module. The todo-state
// sections retired with the todo list render; the full board gallery with
// every kanban state (create error, edit band, drag placeholder, in-flight,
// each stated error) lands with card ui/13 at KW7 — the sections here are
// the placeholder inventory of surfaces that exist now.

import (
	"bytes"
	"html/template"
	"net/http"
	"regexp"
	"strings"
)

// storiesTmpl is the gallery shell. No htmx script: the fragments render
// identically without it (every visible detail is CSS), and its absence
// keeps the gallery inert — no control on this page can reach a live
// endpoint. The components block sits above the state sections: it shows
// each named primitive in its static states, then the state sections show
// the same primitives composed into whole page surfaces. Each state
// section is labelled by the caption directly above it.
var storiesTmpl = template.Must(template.New("stories").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Board UI — components</title>
{{/* Same contractual load order as the real page: token layer first. */}}
<link rel="stylesheet" href="/static/tokens.css">
<link rel="stylesheet" href="/static/style.css">
</head>
<body class="stories">
<main>
<h1 class="heading">Board UI — components</h1>
<p class="state-caption">components</p>
<section id="components" class="state-section">{{.Components}}</section>
{{range .States}}
<p class="state-caption">state: {{.Caption}}</p>
<section id="{{.ID}}">{{.HTML}}</section>
{{end}}
</main>
</body>
</html>
`))

type storySection struct {
	ID      string // stable id — the pixel tests' screenshot anchor
	Caption string // rendered as "state: <caption>"
	HTML    template.HTML
}

type storyPage struct {
	Components template.HTML  // the #c-* component examples block
	States     []storySection // the observable-state sections below it
}

// handleStories answers GET /__components. It reads no store state: every
// section is rendered from the same templates the real page uses, fed by
// the fixtures below.
func (p *page) handleStories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = storiesTmpl.Execute(w, storyPage{
		Components: componentExamplesHTML(),
		States:     storySections(),
	})
}

// storySections builds every observable state that exists after the todo
// list render retired: the board (cards in the fixed columns, one empty
// column, the done treatment in the last) and the stated load failure.
func storySections() []storySection {
	return []storySection{
		{ID: "state-board", Caption: "board",
			HTML: fragmentHTML(boardTmpl, columnsOf(storyBoard()))},
		{ID: "state-load-failure", Caption: "load failure",
			HTML: fragmentHTML(failedTmpl, nil)},
	}
}

// componentExamplesHTML builds the #c-* block: one example per named
// component class, each carrying its exact #c-* id, above the state
// sections. Atoms (buttons, fields, text states) are static markup
// carrying the same component classes the templates emit; composite
// examples are rendered through the REAL surface templates so an example
// cannot drift from what the page renders. The heading example is a div,
// not an h1: the gallery pins itself to one h1, and .heading carries the
// identical computed style either way. The #c-tokens block shows one
// labeled chip per --color-* token parsed from tokens.css: the chips
// mirror the declaration inventory, so the gallery cannot hide a
// declaration change. Renames are pinned on the consumer side — a unit
// test requires every var() reference in style.css to name a declared
// token; the chips never see the references.
func componentExamplesHTML() template.HTML {
	var b bytes.Buffer
	b.WriteString(`<p class="component-label">buttons</p>
<button id="c-btn-primary" type="button" class="btn btn--primary">Primary</button>
<button id="c-btn-secondary" type="button" class="btn btn--secondary">Secondary</button>
<button id="c-btn-disabled" type="button" class="btn btn--secondary" disabled>Disabled</button>
<p class="component-label">input</p>
<input id="c-input-default" class="input" type="text" name="title" value="Draft the launch note" placeholder="What needs doing?">
<input id="c-input-focus" class="input is-focus" type="text" name="title" value="Focused field">
<p class="component-label">stated text</p>
<p id="c-error-text" class="error-text">text is required</p>
<p class="component-label">surfaces</p>
<div id="c-panel" class="panel">Panel surface.</div>
<div id="c-heading" class="heading">Board</div>
<p class="component-label">color tokens</p>
<div id="c-tokens">`)
	for _, name := range componentColorTokens() {
		b.WriteString(`<span class="token-chip"><span class="token-swatch" style="background: ` +
			"var(" + name + `)"></span><code>` + name + `</code></span>`)
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

// customPropRe names every custom property declaration in tokens.css; the
// :root-only file layout means one scan finds the whole inventory.
var customPropRe = regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+)\s*:`)

// componentColorTokens lists the --color-* token names in tokens.css, in
// declaration order — the chip inventory. A parse miss (malformed file)
// shows up as missing chips in the gallery's unit test.
func componentColorTokens() []string {
	src, err := staticFS.ReadFile("static/tokens.css")
	if err != nil {
		return nil // static asset; absence would already fail the page tests
	}
	var names []string
	for _, m := range customPropRe.FindAllStringSubmatch(string(src), -1) {
		if strings.HasPrefix(m[1], "--color-") {
			names = append(names, m[1])
		}
	}
	return names
}

// storyBoard is the board fixture in contract shape: cards in the fixed
// column order, one column empty, the Done column's cards riding through
// the SAME mapping the live page uses — so the done treatment shown here
// is derived from column membership like everywhere else, never faked.
func storyBoard() boardResponse {
	return boardResponse{Columns: []boardColumnResponse{
		{Title: "To Do", Cards: []boardCardResponse{
			{ID: 901, Title: "Draft the launch note", Column: "todo", Position: 1},
			{ID: 902, Title: "Rotate the API keys", Column: "todo", Position: 2},
		}},
		{Title: "In Progress", Cards: []boardCardResponse{}},
		{Title: "Done", Cards: []boardCardResponse{
			{ID: 903, Title: "Ship v1.2", Column: "done", Position: 1},
		}},
	}}
}

// fragmentHTML renders one surface template to a string for embedding in
// the gallery — the same templates, the same output the page sends.
func fragmentHTML(t *template.Template, data any) template.HTML {
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return "" // static templates; nothing here can fail
	}
	return template.HTML(b.String())
}
