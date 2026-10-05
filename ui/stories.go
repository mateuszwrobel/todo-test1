package ui

// stories.go — the /__components story gallery: one page showing every
// observable ui state as a real template rendering against a deterministic
// in-code fixture. It is the ui module's visual test surface: later pixel
// tests screenshot each state container by id (#state-*) and every byte of
// markup is decided at compile time — no store, no api, no clock, no
// randomness. This file is test-support-shaped but production-compiled (it
// serves a real page), and it is stdlib-only like the rest of ui: the
// gallery never touches p.listTodos or any other module.

import (
	"bytes"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
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
<title>Todo UI — components</title>
{{/* Same contractual load order as the real page: token layer first. */}}
<link rel="stylesheet" href="/static/tokens.css">
<link rel="stylesheet" href="/static/style.css">
</head>
<body class="stories">
<main>
<h1 class="heading">Todo UI — components</h1>
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

// storySections builds every observable state, in the order of the ui
// workplan's state list. Id bases keep #todo-N unique across the document
// even though several sections render lists.
func storySections() []storySection {
	return []storySection{
		{ID: "state-list-populated", Caption: "list populated",
			HTML: fragmentHTML(listTmpl, rowsFor(storyRows(1)))},
		{ID: "state-empty", Caption: "empty",
			HTML: fragmentHTML(emptyTmpl, nil)},
		{ID: "state-create-error", Caption: "create error",
			HTML: fragmentHTML(createAreaTmpl, createAreaData{Error: "title is required"})},
		{ID: "state-edit-band", Caption: "edit band",
			HTML: fragmentHTML(listTmpl, storyEditingRows(11))},
		{ID: "state-load-failure", Caption: "load failure",
			HTML: fragmentHTML(failedTmpl, nil)},
		{ID: "state-missing-todo", Caption: "missing todo",
			// Exactly what writeMissingTodo sends: banner, then the
			// server-truth list under it.
			HTML: fragmentHTML(bannerTmpl, "no such todo") +
				fragmentHTML(listTmpl, rowsFor(storyRows(31)))},
		{ID: "state-in-flight-disabled", Caption: "in-flight disabled",
			HTML: storyInFlightHTML()},
	}
}

// componentExamplesHTML builds the #c-* block: one example per named
// component class, each carrying its exact #c-* id, above the state
// sections. Composite examples (rows, banner, empty state) are rendered
// through the REAL surface templates with the example id swapped in —
// the same fidelity rule as the state fixtures, so an example cannot
// drift from what the page renders. Atoms (buttons, fields, text states)
// are static markup carrying the same component classes the templates
// emit. The heading example is a div, not an h1: the gallery pins itself
// to one h1 (the frozen w9 e2e addresses it unambiguously), and .heading
// carries the identical computed style either way. The #c-tokens block
// shows one labeled chip per --color-* token parsed from tokens.css, so
// the gallery cannot hide a token rename from its own color inventory.
func componentExamplesHTML() template.HTML {
	var b bytes.Buffer
	b.WriteString(`<p class="component-label">buttons</p>
<button id="c-btn-primary" type="button" class="btn btn--primary">Primary</button>
<button id="c-btn-secondary" type="button" class="btn btn--secondary">Secondary</button>
<button id="c-btn-disabled" type="button" class="btn btn--secondary" disabled>Disabled</button>
<p class="component-label">input</p>
<input id="c-input-default" class="input" type="text" name="title" value="Walk the dog" placeholder="What needs doing?">
<input id="c-input-focus" class="input is-focus" type="text" name="title" value="Focused field">
<p class="component-label">checkbox</p>
<label class="done-toggle"><input id="c-checkbox-unchecked" type="checkbox" class="checkbox"><span>not done</span></label>
<label class="done-toggle"><input id="c-checkbox-checked" type="checkbox" class="checkbox" checked><span>done</span></label>
<label class="done-toggle"><input id="c-checkbox-disabled" type="checkbox" class="checkbox" disabled><span>not done</span></label>
<p class="component-label">rows</p>`)
	b.WriteString(renderRowExample("c-row-not-done", todo{ID: 901, Title: "Walk the dog", Done: false}))
	b.WriteString(renderRowExample("c-row-done", todo{ID: 902, Title: "Buy milk", Done: true}))
	b.WriteString(`<p class="component-label">stated text</p>
<p id="c-error-text" class="error-text">title is required</p>
`)
	b.WriteString(swapFragmentID(string(fragmentHTML(bannerTmpl, "no such todo")),
		"missing-todo-banner", "c-banner"))
	b.WriteString(`
<p id="c-hint" class="hint">Up to 500 characters</p>
<p class="component-label">surfaces</p>`)
	b.WriteString(swapFragmentID(string(fragmentHTML(emptyTmpl, nil)), "empty-state", "c-empty-state"))
	b.WriteString(`
<div id="c-panel" class="panel">Panel surface.</div>
<div id="c-heading" class="heading">My Todos</div>
<p class="component-label">color tokens</p>
<div id="c-tokens">`)
	for _, name := range componentColorTokens() {
		b.WriteString(`<span class="token-chip"><span class="token-swatch" style="background: ` +
			"var(" + name + `)"></span><code>` + name + `</code></span>`)
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

// renderRowExample renders ONE row through the real list template (so the
// example is the real row markup, controls and all) under the example id.
func renderRowExample(id string, t todo) string {
	return swapFragmentID(string(fragmentHTML(listTmpl, rowsFor([]todo{t}))),
		"todo-"+strconv.FormatInt(t.ID, 10), id)
}

// swapFragmentID renames one id inside a rendered fragment so the example
// carries the #c-* anchor while every other attribute stays as the real
// template emitted it.
func swapFragmentID(fragment, from, to string) string {
	return strings.Replace(fragment, `id="`+from+`"`, `id="`+to+`"`, 1)
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

// storyRows is the fixture list: mixed done states, fixed titles, ids from
// idBase. The same rows render in every list section (offset per section
// only to keep DOM ids unique).
func storyRows(idBase int64) []todo {
	return []todo{
		{ID: idBase, Title: "Buy milk", Done: true},
		{ID: idBase + 1, Title: "Pay electricity bill", Done: true},
		{ID: idBase + 2, Title: "Walk the dog", Done: false},
		{ID: idBase + 3, Title: "Read 20 pages", Done: false},
	}
}

// storyEditingRows is the edit-band fixture: the not-done row "Walk the
// dog" sits in edit mode with its band open around the prefilled text
// (mockup 04), so the band renders visible without any client script.
func storyEditingRows(idBase int64) []listRow {
	rows := rowsFor(storyRows(idBase))
	rows[2].Editing = true
	rows[2].Typed = "Walk the dog in the park"
	return rows
}

// storyInFlightHTML renders the create row and a list (with one band open)
// through the real templates, then marks every control htmx disables during
// flight (card ui/16's four controls: Add, checkbox, Save, Delete) with the
// plain disabled attribute — exactly what the page looks like while a
// request is in flight. The replacements target the templates' rendered
// control markup; the gallery's unit test pins that they still land. The
// create-area/create-form ids get a section-local suffix so the document
// keeps unique ids while both create sections stay structurally faithful.
func storyInFlightHTML() template.HTML {
	html := string(createAreaHTML(createAreaData{})) +
		string(fragmentHTML(listTmpl, storyEditingRows(21)))
	html = strings.ReplaceAll(html, `<button type="submit" class="btn btn--primary">Add</button>`,
		`<button type="submit" class="btn btn--primary" disabled>Add</button>`)
	html = strings.ReplaceAll(html, `<input type="checkbox" `,
		`<input type="checkbox" disabled `)
	html = strings.ReplaceAll(html, `<button type="submit" class="btn btn--primary save">`,
		`<button type="submit" class="btn btn--primary save" disabled>`)
	html = strings.ReplaceAll(html, `<button type="button" class="btn btn--secondary delete" `,
		`<button type="button" class="btn btn--secondary delete" disabled `)
	html = strings.Replace(html, `id="create-area"`, `id="create-area-in-flight"`, 1)
	html = strings.Replace(html, `id="create-form"`, `id="create-form-in-flight"`, 1)
	return template.HTML(html)
}

// fragmentHTML renders one surface template to a string for embedding in
// the gallery — the same templates, the same output the fragment endpoints
// send.
func fragmentHTML(t *template.Template, data any) template.HTML {
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return "" // static templates; nothing here can fail
	}
	return template.HTML(b.String())
}
