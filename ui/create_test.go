package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"todo/api"
	"todo/board"
)

// realChain wires the whole path these tests exercise: a real board store
// (SQLite in a temp dir) under a real api handler over real HTTP, with the
// page pointed at that address — the composition the binary itself assembles.
// A canned fake could not prove the contract's refusal strings flow through
// unchanged, so the create chain is tested for real end to end (same real-
// store style the api lane's create tests use). The todo store port is nil:
// the surviving todo endpoints play no part here, and a nil port fails loudly
// if a test ever reaches them.
func realChain(t *testing.T, seeds ...string) (*board.Store, *httptest.Server) {
	t.Helper()
	store, err := board.Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatalf("board.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, text := range seeds {
		if _, err := store.Create(text); err != nil {
			t.Fatalf("seed card %q: %v", text, err)
		}
	}
	apiSrv := fakeAPI(t, api.NewHandler(nil, store).ServeHTTP)
	return store, uiServer(t, apiSrv.URL)
}

// postCreate submits the create form the way htmx does: a form POST to the
// ui fragment endpoint.
func postCreate(t *testing.T, uiURL, title string) (int, string) {
	t.Helper()
	resp, err := http.PostForm(uiURL+"/ui/cards", url.Values{"title": {title}})
	if err != nil {
		t.Fatalf("POST /ui/cards: %v", err)
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, buf.String()
}

// toDoCards returns the created cards' titles top-to-bottom from the store's
// own truth (what the next board read will render).
func toDoCards(t *testing.T, store *board.Store) []string {
	t.Helper()
	columns, err := store.List()
	if err != nil {
		t.Fatalf("board List: %v", err)
	}
	for _, col := range columns {
		if col.Name != board.Todo {
			continue
		}
		titles := make([]string, 0, len(col.Cards))
		for _, c := range col.Cards {
			titles = append(titles, c.Title)
		}
		return titles
	}
	t.Fatalf("board has no To Do column: %+v", columns)
	return nil
}

// Card ui/04 — Create appends without reload.
// Given the page shows the board
// When  the user types text into the create input and submits
// Then  the new card appears at the bottom of "To Do" without a page reload
//
//	And the input and Add button return to ready for the next card
//
// The fragment shape below is what makes "without reload" possible: a swap-
// targeted fragment, not a whole document. The no-navigation guarantee
// itself is htmx swapping the response in place; the e2e lane pins that
// browser behavior.
func TestCreateAppendsWithoutReload(t *testing.T) {
	store, uiSrv := realChain(t, "Write weekly report", "Fix login redirect")

	// Given the page shows the board, wired for htmx create-in-place.
	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	for _, want := range []string{`hx-post="/ui/cards"`, `hx-target="#board-area"`,
		`id="board-area"`, `id="create-area"`, "Write weekly report", "Fix login redirect"} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %s:\n%s", want, page)
		}
	}

	// When the create form submits.
	status, frag := postCreate(t, uiSrv.URL, "Buy milk")
	if status != http.StatusCreated {
		t.Fatalf("POST /ui/cards status = %d, want 201 (body %s)", status, frag)
	}

	// Then: a fragment (no document → nothing to reload).
	if strings.Contains(strings.ToLower(frag), "<!doctype") || strings.Contains(frag, "<html") {
		t.Errorf("create answered a full document, not a swap fragment:\n%s", frag)
	}

	// The new card sits at the bottom of the To Do column — after both
	// existing cards — and nowhere else.
	todo := columnHTML(t, frag, "to-do")
	prev := -1
	for _, title := range []string{"Write weekly report", "Fix login redirect", "Buy milk"} {
		at := strings.Index(todo, title)
		if at < 0 {
			t.Fatalf("To Do is missing %q:\n%s", title, todo)
		}
		if at < prev {
			t.Errorf("To Do does not list %q at the bottom:\n%s", title, todo)
		}
		prev = at
	}
	if !strings.Contains(todo, `id="card-`) {
		t.Errorf("the appended card has no card element:\n%s", todo)
	}
	for _, anchor := range []string{"in-progress", "done"} {
		if strings.Contains(columnHTML(t, frag, anchor), `id="card-`) {
			t.Errorf("create put a card into the %s column:\n%s", anchor, frag)
		}
	}

	// The contract's truth: three To Do cards, the new one last.
	if got := toDoCards(t, store); len(got) != 3 || got[2] != "Buy milk" {
		t.Errorf("store To Do column = %q, want the created card appended last", got)
	}

	// The create area comes back reset for the next card (out-of-band swap):
	// empty input, an Add button, ready state.
	if !strings.Contains(frag, `id="create-area"`) || !strings.Contains(frag, "hx-swap-oob") {
		t.Errorf("fragment carries no out-of-band create-area reset:\n%s", frag)
	}
	if !strings.Contains(frag, `name="title" value=""`) || !strings.Contains(frag, ">Add<") {
		t.Errorf("create control not ready for the next card:\n%s", frag)
	}
}

// Card ui/04 (trim arm) — the appended card lands at the bottom as the
// server stored it: the contract trims the title, so the swap content shows
// the trimmed text, not the typed padding.
func TestCreateTrimmedTitleLandsAtBottom(t *testing.T) {
	store, uiSrv := realChain(t, "Existing card")

	_, frag := postCreate(t, uiSrv.URL, "  Padded title  ")
	if !strings.Contains(frag, `<span class="card__title">Padded title</span>`) {
		t.Fatalf("trimmed title not rendered at the swap content:\n%s", frag)
	}
	todo := columnHTML(t, frag, "to-do")
	if strings.Index(todo, "Existing card") > strings.Index(todo, "Padded title") {
		t.Errorf("trimmed title did not land at the bottom:\n%s", todo)
	}
	if got := toDoCards(t, store); len(got) != 2 || got[1] != "Padded title" {
		t.Errorf("store To Do column = %q, want the trimmed card appended last", got)
	}
}
