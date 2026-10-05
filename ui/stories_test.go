package ui

import (
	"go/parser"
	"go/token"
	"net/http"
	"strings"
	"testing"
)

// W9 — the /__components story gallery. One observable state per section,
// each section a real template rendering of a deterministic fixture, each
// addressed by its stable #state-* id for later pixel tests.

const storyGalleryPath = "/__components"

func galleryPage(t *testing.T) (int, string) {
	t.Helper()
	// The gallery reads no live state, so an unroutable api base is fine.
	return getPage(t, uiServer(t, "http://127.0.0.1:1").URL+storyGalleryPath)
}

// sectionHTML slices one section's markup out of the gallery page.
func sectionHTML(t *testing.T, page, id string) string {
	t.Helper()
	start := strings.Index(page, `<section id="`+id+`">`)
	if start < 0 {
		t.Fatalf("gallery page has no section #%s:\n%s", id, page)
	}
	end := strings.Index(page[start:], `</section>`)
	if end < 0 {
		t.Fatalf("section #%s is unterminated", id)
	}
	return page[start : start+end]
}

func TestStoryGalleryRendersEveryState(t *testing.T) {
	status, page := galleryPage(t)
	if status != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", storyGalleryPath, status)
	}
	if !strings.Contains(page, "<title>Todo UI — components</title>") {
		t.Errorf("gallery page missing its title:\n%s", page)
	}
	if !strings.Contains(page, `href="/static/style.css"`) {
		t.Error("gallery page does not link the stylesheet")
	}

	states := []struct{ id, caption string }{
		{"state-list-populated", "state: list populated"},
		{"state-empty", "state: empty"},
		{"state-create-error", "state: create error"},
		{"state-edit-band", "state: edit band"},
		{"state-load-failure", "state: load failure"},
		{"state-missing-todo", "state: missing todo"},
		{"state-in-flight-disabled", "state: in-flight disabled"},
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
	if !strings.Contains(page, `<section id="state-list-populated">`) {
		t.Error("gallery content missing with a live-but-untouchable api")
	}
}

func TestStoryGalleryFixturesAreContractTrue(t *testing.T) {
	_, page := galleryPage(t)

	checks := []struct {
		section string
		want    []string
	}{
		{"state-list-populated", []string{
			`id="todo-list"`, "Buy milk", "Pay electricity bill", "Walk the dog", "Read 20 pages",
			`data-state="done"`, `data-state="not-done"`,
		}},
		{"state-empty", []string{`id="empty-state"`, "No todos yet."}},
		{"state-create-error", []string{`id="create-error"`, "title is required"}},
		{"state-edit-band", []string{`class="editing"`, "Walk the dog in the park"}},
		{"state-load-failure", []string{`id="load-error"`, "Could not load todos.", `id="retry"`}},
		{"state-missing-todo", []string{`id="missing-todo-banner"`, "no such todo", `id="todo-list"`}},
	}
	for _, c := range checks {
		section := sectionHTML(t, page, c.section)
		for _, want := range c.want {
			if !strings.Contains(section, want) {
				t.Errorf("section #%s missing %q\nsection: %s", c.section, want, section)
			}
		}
	}
}

func TestStoryGalleryInFlightControlsCarryDisabled(t *testing.T) {
	_, page := galleryPage(t)
	section := sectionHTML(t, page, "state-in-flight-disabled")

	// The four controls card ui/16 blocks while in flight: create Add,
	// row checkbox, edit Save, row Delete.
	for _, want := range []string{
		`<button type="submit" disabled>Add</button>`,
		`<input type="checkbox" disabled `,
		`<button type="submit" class="save" disabled>`,
		`<button type="button" class="delete" disabled `,
	} {
		if !strings.Contains(section, want) {
			t.Errorf("in-flight section missing disabled control %q\nsection: %s", want, section)
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
