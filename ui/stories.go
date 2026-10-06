package ui

// stories.go — the /__components story gallery: one page showing the
// observable ui states as real template renderings against deterministic
// in-code fixtures. It is the ui module's visual test surface: later pixel
// tests screenshot each state container by id (#state-*) and every byte of
// markup is decided at compile time — no store, no api, no clock, no
// randomness. This file is test-support-shaped but production-compiled (it
// serves a real page), and it is stdlib-only like the rest of ui: the
// gallery never touches p.loadBoard or any other module. The todo-state
// sections retired with the todo list render; the full board gallery —
// every observable state (board, empty board, load failure, create band
// ready and rejected, edit band open, refusal at card, stale banner, drag
// mid-gesture) — landed here at KW7 (card ui/13). Pixel-snapshot baselines
// over these sections are a separate follow-up lane.

import (
	"bytes"
	"fmt"
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

// storySections builds every observable state of the board page (card
// ui/13): the populated board (with one stated-empty column and the done
// treatment derived in the last), the whole-board empty, the stated load
// failure, the create band ready and rejected, the inline edit band open,
// an edit refusal stated at its card, the stale-operation banner over the
// truth, and a drag frozen mid-gesture. Every section is a real template
// rendering against the deterministic fixtures below — the states that
// only exist mid-interaction (band open, drag chrome) are the same markup
// with the shell-script class hooks frozen, per the static-stand-in
// precedent the input example's .is-focus established.
func storySections() []storySection {
	return []storySection{
		{ID: "state-board", Caption: "board",
			HTML: fragmentHTML(boardTmpl, columnsOf(storyBoard()))},
		{ID: "state-board-empty", Caption: "empty board",
			HTML: fragmentHTML(boardTmpl, columnsOf(storyBoardEmpty()))},
		{ID: "state-load-failure", Caption: "load failure",
			HTML: fragmentHTML(failedTmpl, nil)},
		{ID: "state-create", Caption: "create band",
			HTML: fragmentHTML(createAreaTmpl, createAreaData{})},
		{ID: "state-create-error", Caption: "rejected create",
			HTML: fragmentHTML(createAreaTmpl, createAreaData{Value: "   ", Error: "text is required"})},
		{ID: "state-edit-band", Caption: "edit band open",
			HTML: addCardClass(fragmentHTML(boardTmpl, columnsOf(storyBoardFull())), 913, "editing")},
		{ID: "state-edit-error", Caption: "edit refused at card",
			HTML: storyEditError()},
		{ID: "state-stale", Caption: "stale operation",
			HTML: storyStaleFailure()},
		{ID: "state-drag", Caption: "drag mid-gesture",
			HTML: storyDrag()},
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
<p class="component-label">board cards (k6/k7: the controls paint in on row hover)</p>
<ul class="column__cards">
<li id="c-card" class="card"><span class="card__title">Draft the launch note</span><button type="button" class="btn btn--secondary card__edit">Edit</button><button type="button" class="btn btn--secondary card__delete">Delete</button></li>
<li id="c-card-done" class="card card--done"><span class="card__title">Ship v1.2</span><button type="button" class="btn btn--secondary card__edit">Edit</button><button type="button" class="btn btn--secondary card__delete">Delete</button></li>
<li id="c-source-slot" class="card card--source"><span class="card__title">Lifted while dragging</span></li>
</ul>
<p class="component-label">drag chrome (the shell-toggled classes, frozen static)</p>
<ul class="column__cards"><li id="c-drop-indicator" class="drop-indicator"></li></ul>
<div id="c-drag-chip" class="drag-chip">Fix login redirect</div>
<p class="component-label">stated failure banner</p>
<div id="c-banner" class="banner" role="alert">no such card</div>
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

// storyBoardFull is the every-column-populated fixture the interaction
// states need (edit band, refusal, drag all land on a card with neighbors):
// same contract shape, same derived done treatment, identifiers continuing
// past storyBoard's so a page holds both without collisions.
func storyBoardFull() boardResponse {
	return boardResponse{Columns: []boardColumnResponse{
		{Title: "To Do", Cards: []boardCardResponse{
			{ID: 911, Title: "Write the weekly report", Column: "todo", Position: 1},
			{ID: 912, Title: "Fix login redirect", Column: "todo", Position: 2},
		}},
		{Title: "In Progress", Cards: []boardCardResponse{
			{ID: 913, Title: "Refactor board module", Column: "in_progress", Position: 1},
			{ID: 914, Title: "Draft ADR-004", Column: "in_progress", Position: 2},
		}},
		{Title: "Done", Cards: []boardCardResponse{
			{ID: 915, Title: "Migrate todo list", Column: "done", Position: 1},
		}},
	}}
}

// storyBoardEmpty is the whole-board-empty fixture (journeys' derived page
// state "empty board"): every column present, every column stating its
// emptiness — visibly an empty board, not a blank page.
func storyBoardEmpty() boardResponse {
	return boardResponse{Columns: []boardColumnResponse{
		{Title: "To Do", Cards: []boardCardResponse{}},
		{Title: "In Progress", Cards: []boardCardResponse{}},
		{Title: "Done", Cards: []boardCardResponse{}},
	}}
}

// storyEditError renders the rejected-edit state through the live path:
// the truth columns with the contract's refusal attached to the edited
// card (attachEditError — the same function the PATCH handler uses), so
// the card keeps its original title and states the reason at itself.
func storyEditError() template.HTML {
	columns := columnsOf(storyBoardFull())
	attachEditError(columns, 912, "text is required")
	return fragmentHTML(boardTmpl, columns)
}

// storyStaleFailure renders the stale-operation surface exactly as
// writeStaleFailure answers it: the banner stating the contract's reason
// first, then the truth re-rendered without the card.
func storyStaleFailure() template.HTML {
	return fragmentHTML(missingCardTmpl, "no such card") +
		fragmentHTML(boardTmpl, columnsOf(storyBoardFull()))
}

// storyDrag freezes a between-columns drag mid-gesture: card 911 lifted
// from To Do renders as the dashed source slot, the insertion line sits in
// In Progress ahead of card 914 (the landing gap dropPosition counts),
// and the hovered column carries the drop-target ring. These are the exact
// classes the shell script toggles (render.go); the gallery has no script,
// so the frozen hooks are the static stand-in — the markup itself still
// comes from boardTmpl unchanged.
func storyDrag() template.HTML {
	board := addCardClass(fragmentHTML(boardTmpl, columnsOf(storyBoardFull())), 911, "card--source")
	board = template.HTML(strings.Replace(string(board),
		`<li id="card-914"`,
		`<li class="drop-indicator"></li><li id="card-914"`, 1))
	return template.HTML(strings.Replace(string(board),
		`id="column-in-progress" class="column"`,
		`id="column-in-progress" class="column column--drop-target"`, 1))
}

// addCardClass appends a rendering class to one card of an already-rendered
// board fragment — the gallery freezing a shell-script class hook (see
// storyDrag; the .is-focus input example is the same precedent). The card
// markup itself is untouched; a card that lost this attribute shape is a
// template change the gallery section tests then loudly notice.
func addCardClass(html template.HTML, id int64, class string) template.HTML {
	return template.HTML(strings.Replace(string(html),
		fmt.Sprintf(`<li id="card-%d" class="card"`, id),
		fmt.Sprintf(`<li id="card-%d" class="card %s"`, id, class),
		1))
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
