package ui

import (
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
)

// W9 — the /__components story gallery. One observable state per section,
// each section a real template rendering of a deterministic fixture, each
// addressed by its stable #state-* id for later pixel tests. The todo-state
// sections retired with the todo list render; the full board gallery — the
// whole observable-state inventory (card ui/13, KW7) — stands here now.

const storyGalleryPath = "/__components"

func galleryPage(t *testing.T) (int, string) {
	t.Helper()
	// The gallery reads no live state, so an unroutable api base is fine.
	return getPage(t, uiServer(t, "http://127.0.0.1:1").URL+storyGalleryPath)
}

// sectionHTML slices one section's markup out of the gallery page. Depth-
// aware: the board section nests its column <section> elements, so the
// slice ends at the closing tag that matches the section's own opening.
func sectionHTML(t *testing.T, page, id string) string {
	t.Helper()
	start := strings.Index(page, `<section id="`+id+`">`)
	if start < 0 {
		t.Fatalf("gallery page has no section #%s:\n%s", id, page)
	}
	rest := page[start:]
	depth, at := 0, 0
	for {
		open := strings.Index(rest[at:], `<section`)
		close := strings.Index(rest[at:], `</section>`)
		if close < 0 {
			t.Fatalf("section #%s is unterminated", id)
		}
		if open >= 0 && open < close {
			depth++
			at += open + len("<section")
			continue
		}
		depth--
		if depth == 0 {
			return page[start : start+at+close+len("</section>")]
		}
		at += close + len("</section>")
	}
}

func TestStoryGalleryRendersEveryState(t *testing.T) {
	status, page := galleryPage(t)
	if status != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", storyGalleryPath, status)
	}
	if !strings.Contains(page, "<title>Board UI — components</title>") {
		t.Errorf("gallery page missing its title:\n%s", page)
	}
	if !strings.Contains(page, `href="/static/style.css"`) {
		t.Error("gallery page does not link the stylesheet")
	}

	states := []struct{ id, caption string }{
		{"state-board", "state: board"},
		{"state-board-empty", "state: empty board"},
		{"state-load-failure", "state: load failure"},
		{"state-create", "state: create band"},
		{"state-create-error", "state: rejected create"},
		{"state-edit-band", "state: edit band open"},
		{"state-edit-error", "state: edit refused at card"},
		{"state-stale", "state: stale operation"},
		{"state-drag", "state: drag mid-gesture"},
		{"state-assigned", "state: assignment states"},
		{"state-filtered", "state: filtered column"},
	}
	for _, s := range states {
		sectionHTML(t, page, s.id) // fails loudly when the container is absent
		if !strings.Contains(page, s.caption) {
			t.Errorf("gallery is missing the caption %q", s.caption)
		}
	}
}

func TestStoryGalleryNeverTouchesTheContract(t *testing.T) {
	// The gallery renders from fixtures only: an unreachable api base must
	// not slow it down or change its output, and no request may reach the
	// api side at all.
	apiHit := false
	api := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		apiHit = true
		http.Error(w, "the gallery must not call the api", http.StatusInternalServerError)
	})
	srv := uiServer(t, api.URL)
	status, page := getPage(t, srv.URL+storyGalleryPath)
	if status != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", storyGalleryPath, status)
	}
	if apiHit {
		t.Error("the story gallery reached the api — fixtures must be in-code only")
	}
	if !strings.Contains(page, `<section id="state-board">`) {
		t.Error("gallery content missing with a live-but-untouchable api")
	}
}

func TestStoryGalleryFixturesAreContractTrue(t *testing.T) {
	_, page := galleryPage(t)

	checks := []struct {
		section string
		want    []string
	}{
		{"state-board", []string{
			`id="board"`, `id="column-to-do"`, `id="column-in-progress"`, `id="column-done"`,
			"Draft the launch note", "Rotate the API keys", "Ship v1.2",
			`id="card-903" class="card card--done"`,                   // done treatment derived from column membership
			`hx-delete="/ui/cards/903"`,                               // delete stays on the done card (the freeze takes the edit only)
			`<p class="column__empty" data-empty="true">No cards</p>`, // the empty column's stated treatment
		}},
		{"state-board-empty", []string{
			`id="board"`,
			`id="column-to-do"`, `id="column-in-progress"`, `id="column-done"`,
			// every column states its emptiness; no card, no failure surface.
		}},
		{"state-load-failure", []string{`id="load-error"`, "Could not load board.", `id="retry"`}},
		{"state-create", []string{`id="create-area"`, `id="create-form"`, `name="title"`, ">Add<"}},
		{"state-create-error", []string{`id="create-error"`, `class="error-text"`, "text is required"}},
		{"state-edit-band", []string{
			// the band markup is the template's own; .editing is the frozen
			// reveal hook (static stand-in for the shell script's class toggle).
			`id="card-913" class="card editing"`,
			`<input class="input" type="text" name="title" value="Refactor board module">`,
			`class="btn btn--primary save"`, `card__edit`,
		}},
		{"state-edit-error", []string{
			`id="edit-error-912"`, `class="error-text"`, "text is required",
			`<span class="card__title">Fix login redirect</span>`, // original text stands, refusal stated at the card
		}},
		{"state-stale", []string{
			`id="missing-card"`, `class="banner"`, "no such card",
			`id="board"`, // the truth under the banner — and not the stale card: 911–915 are all still board truth here
		}},
		{"state-drag", []string{
			`id="card-911" class="card card--source"`, // dashed source slot
			`<li class="drop-indicator"></li>`,        // insertion line at the landing gap
			`class="column column--drop-target"`,      // the hovered column's ring
		}},
	}
	for _, c := range checks {
		section := sectionHTML(t, page, c.section)
		for _, want := range c.want {
			if !strings.Contains(section, want) {
				t.Errorf("section #%s missing %q\nsection: %s", c.section, want, section)
			}
		}
	}
	// The done freeze (2026-10-07, parent scenario 15): the board
	// fixture's Done card 903 renders no edit affordance, while the cards
	// outside it keep their bands.
	if s := sectionHTML(t, page, "state-board"); strings.Contains(s, `hx-patch="/ui/cards/903"`) {
		t.Errorf("state-board renders an edit affordance on the done card:\n%s", s)
	}
	if s := sectionHTML(t, page, "state-board"); !strings.Contains(s, `hx-patch="/ui/cards/901"`) ||
		!strings.Contains(s, `hx-patch="/ui/cards/902"`) {
		t.Errorf("state-board lost an edit band on a card outside Done:\n%s", s)
	}
	// The whole-board-empty section states emptiness three times and never
	// fakes cards or failure.
	empty := sectionHTML(t, page, "state-board-empty")
	if got := strings.Count(empty, `class="column__empty"`); got != 3 {
		t.Errorf("empty board states emptiness %d times, want 3 per column\nsection: %s", got, empty)
	}
	if strings.Contains(empty, `id="card-`) || strings.Contains(empty, `id="load-error"`) {
		t.Errorf("empty board section renders cards or a failure surface:\n%s", empty)
	}
	// The board fixture states its fixed columns in order.
	section := sectionHTML(t, page, "state-board")
	for _, title := range []string{"To Do", "In Progress", "Done"} {
		if !strings.Contains(section, title) {
			t.Errorf("section #state-board missing column title %q\nsection: %s", title, section)
		}
	}
}

func TestGalleryComponentExamplesAnchorEveryPrimitive(t *testing.T) {
	// W10 part 3 — one example per named component, each at its exact
	// #c-* id, with the components block above the state sections and
	// the page keeping its single-h1 shape. The checkbox and row examples
	// retired with the todo list surface; board component examples join
	// with the gallery pass at KW7 (card ui/13).
	_, page := galleryPage(t)

	ids := []string{
		"c-btn-primary", "c-btn-secondary", "c-btn-disabled",
		"c-input-default", "c-input-focus",
		"c-error-text", "c-panel", "c-heading", "c-tokens",
		// KW7 board examples (card ui/13): the card component in its plain
		// and done treatments, the drag chrome classes, and the failure banner.
		"c-card", "c-card-done", "c-source-slot", "c-drop-indicator", "c-drag-chip", "c-banner",
	}
	for _, id := range ids {
		if got := strings.Count(page, `id="`+id+`"`); got != 1 {
			t.Errorf("example #%s appears %d times, want exactly 1", id, got)
		}
	}
	if strings.Count(page, "<h1") != 1 {
		t.Errorf("gallery no longer carries exactly one h1:\n%s", page)
	}
	componentsAt := strings.Index(page, `<section id="components"`)
	statesAt := strings.Index(page, `<section id="state-board"`)
	if componentsAt < 0 || statesAt < 0 || componentsAt > statesAt {
		t.Errorf("components block is not above the state sections (components at %d, states at %d)",
			componentsAt, statesAt)
	}
}

func TestGalleryComponentExamplesCarryFrozenStates(t *testing.T) {
	_, page := galleryPage(t)
	for _, want := range []string{
		`id="c-btn-disabled" type="button" class="btn btn--secondary" disabled`,
		`id="c-input-focus" class="input is-focus"`, // static focus stand-in
	} {
		if !strings.Contains(page, want) {
			t.Errorf("component example missing %q", want)
		}
	}
	// The done card example carries delete only: the done freeze of
	// 2026-10-07 leaves no edit affordance on a done card (the plain
	// example above keeps Edit — the pair shows both arms).
	at := strings.Index(page, `id="c-card-done"`)
	if at < 0 {
		t.Fatal(`gallery has no #c-card-done example`)
	}
	doneExample := page[at : at+strings.Index(page[at:], `</li>`)]
	if strings.Contains(doneExample, `card__edit`) || strings.Contains(doneExample, `edit-form`) {
		t.Errorf("done card example carries a frozen edit affordance:\n%s", doneExample)
	}
	if !strings.Contains(doneExample, `card__delete`) {
		t.Errorf("done card example lost its delete control:\n%s", doneExample)
	}
}

func TestGalleryTokenChipsMatchTokensFile(t *testing.T) {
	// Drift gate between the design system and its gallery: one labeled
	// chip per --color-* token, names parsed straight from
	// static/tokens.css — a token added there without a gallery chip
	// (or a chip naming a token that no longer exists) fails here. This
	// gate tracks declarations only; the consumer side (every var()
	// reference in style.css naming a declared token) is pinned by the
	// token-layer test.
	src, err := os.ReadFile("static/tokens.css")
	if err != nil {
		t.Fatalf("read tokens.css: %v", err)
	}
	var declared []string
	for _, m := range regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+)\s*:`).FindAllStringSubmatch(string(src), -1) {
		if strings.HasPrefix(m[1], "--color-") {
			declared = append(declared, m[1])
		}
	}
	if len(declared) == 0 {
		t.Fatal("tokens.css declares no --color-* tokens")
	}

	_, page := galleryPage(t)
	labeled := map[string]bool{}
	for _, m := range regexp.MustCompile(`<code>(--[a-z0-9-]+)</code>`).FindAllStringSubmatch(page, -1) {
		labeled[m[1]] = true
	}
	for _, name := range declared {
		if !labeled[name] {
			t.Errorf("token %s declared in tokens.css has no gallery chip", name)
		}
	}
	for name := range labeled {
		found := false
		for _, d := range declared {
			if d == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("gallery chip %s names no token declared in tokens.css", name)
		}
	}
}

func TestStoriesFileIsStdlibOnly(t *testing.T) {
	// The ui module's boundary is stdlib-only; the stories file must not
	// reach the store or the api in-process (archspec gates the whole
	// package — this pins the stories file specifically).
	file, err := parser.ParseFile(token.NewFileSet(), "stories.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse stories.go: %v", err)
	}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.HasPrefix(path, "todo/") || strings.Contains(strings.SplitN(path, "/", 2)[0], ".") {
			t.Errorf("stories.go imports non-stdlib package %q", path)
		}
	}
}
