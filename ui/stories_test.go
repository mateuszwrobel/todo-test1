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
// sections retired with the todo list render; the full board gallery is
// card ui/13 (KW7). What stands here is the placeholder inventory of the
// surfaces that exist now: the board and the stated load failure.

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
		{"state-load-failure", "state: load failure"},
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
			`<p class="column__empty" data-empty="true">No cards</p>`, // the empty column's stated treatment
		}},
		{"state-load-failure", []string{`id="load-error"`, "Could not load board.", `id="retry"`}},
	}
	for _, c := range checks {
		section := sectionHTML(t, page, c.section)
		for _, want := range c.want {
			if !strings.Contains(section, want) {
				t.Errorf("section #%s missing %q\nsection: %s", c.section, want, section)
			}
		}
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
