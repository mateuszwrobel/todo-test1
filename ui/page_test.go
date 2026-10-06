package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// fakeAPI serves a canned GET /board over real HTTP — the api contract
// stand-in for these tests. The ui module talks to it only via its base URL.
func fakeAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func uiServer(t *testing.T, apiBase string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(NewHandler(apiBase))
	t.Cleanup(srv.Close)
	return srv
}

func getPage(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, buf.String()
}

// boardJSON is the contract body with cards in all three columns; each
// column's array is in position order, as the contract states it.
const boardJSON = `{
	"columns": [
		{"title": "To Do", "cards": [
			{"id": 1, "title": "Draft the launch note", "column": "todo", "position": 1},
			{"id": 2, "title": "Rotate the API keys", "column": "todo", "position": 2}]},
		{"title": "In Progress", "cards": [
			{"id": 3, "title": "Wire the webhook", "column": "in_progress", "position": 1}]},
		{"title": "Done", "cards": [
			{"id": 4, "title": "Ship v1.2", "column": "done", "position": 1}]}
	]
}`

func boardAPI(t *testing.T) *httptest.Server {
	return fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/board" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(boardJSON))
	})
}

// columnHTML slices one column panel's markup out of the page.
func columnHTML(t *testing.T, page, anchor string) string {
	t.Helper()
	start := strings.Index(page, `<section id="column-`+anchor+`"`)
	if start < 0 {
		t.Fatalf("page has no column #%s:\n%s", anchor, page)
	}
	end := strings.Index(page[start:], `</section>`)
	if end < 0 {
		t.Fatalf("column #%s is unterminated", anchor)
	}
	return page[start : start+end]
}

// cardHTML slices one card's markup out of the page.
func cardHTML(t *testing.T, page string, id int64) string {
	t.Helper()
	start := strings.Index(page, `id="card-`+strconv.FormatInt(id, 10)+`"`)
	if start < 0 {
		t.Fatalf("page has no card %d:\n%s", id, page)
	}
	end := strings.Index(page[start:], `</li>`)
	if end < 0 {
		t.Fatalf("card %d is unterminated", id)
	}
	return page[start : start+end]
}

// Card ui/01 — Board renders three fixed columns.
// Given the server holds cards in all three columns
// When  the user opens the page
// Then  three column panels appear in the order "To Do", "In Progress", "Done"
//
//	And each panel lists its cards top-to-bottom exactly as the server's arrays order them
//	And cards in "Done" render with the done treatment while other cards render plain
//	And no done checkbox or toggle exists anywhere on the page
func TestBoardRendersThreeFixedColumns(t *testing.T) {
	api := boardAPI(t)
	status, page := getPage(t, uiServer(t, api.URL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	// Three column panels, in the fixed order.
	anchors := []string{"to-do", "in-progress", "done"}
	titles := []string{"To Do", "In Progress", "Done"}
	prev := -1
	for i, anchor := range anchors {
		at := strings.Index(page, `<section id="column-`+anchor+`"`)
		if at < 0 {
			t.Fatalf("page is missing the column panel %q:\n%s", titles[i], page)
		}
		if at < prev {
			t.Errorf("column %q does not follow the fixed order:\n%s", titles[i], page)
		}
		prev = at
		if got := strings.Index(columnHTML(t, page, anchor), titles[i]); got < 0 {
			t.Errorf("column #%s does not state its title %q", anchor, titles[i])
		}
	}

	// Each panel lists its cards top-to-bottom exactly as the server's
	// arrays order them.
	for _, c := range []struct {
		anchor string
		ids    []int64
	}{{"to-do", []int64{1, 2}}, {"in-progress", []int64{3}}, {"done", []int64{4}}} {
		col := columnHTML(t, page, c.anchor)
		prev := -1
		for _, id := range c.ids {
			at := strings.Index(col, `id="card-`+strconv.FormatInt(id, 10)+`"`)
			if at < 0 {
				t.Fatalf("column %s is missing card %d:\n%s", c.anchor, id, col)
			}
			if at < prev {
				t.Errorf("column %s does not list card %d in array order:\n%s", c.anchor, id, col)
			}
			prev = at
		}
	}

	// The done treatment renders on the Done column's cards; every other
	// card renders plain.
	if !strings.Contains(cardHTML(t, page, 4), `card--done`) {
		t.Errorf("card 4 (Done column) lacks the done treatment:\n%s", cardHTML(t, page, 4))
	}
	for _, id := range []int64{1, 2, 3} {
		if strings.Contains(cardHTML(t, page, id), `card--done`) {
			t.Errorf("card %d (not in Done) carries the done treatment:\n%s", id, cardHTML(t, page, id))
		}
	}

	// No done checkbox or toggle anywhere on the page.
	if strings.Contains(page, `type="checkbox"`) {
		t.Errorf("page carries a done checkbox/toggle:\n%s", page)
	}

	// Every card carries its edit and delete affordance hooks. The edit
	// band is live since KW3 (card ui/06 — every card, every column); the
	// delete control is live since KW4 (card ui/07) — hx-delete wiring,
	// pinned in full by delete_test.go.
	for _, id := range []int64{1, 2, 3, 4} {
		card := cardHTML(t, page, id)
		if !strings.Contains(card, `class="btn btn--secondary card__edit"`) {
			t.Errorf("card %d has no edit affordance element:\n%s", id, card)
		}
		if !strings.Contains(card, `hx-delete="/ui/cards/`) {
			t.Errorf("card %d has no delete affordance element:\n%s", id, card)
		}
	}
}

// htmx is vendored and served by this module (ADR-001).
func TestServesVendoredHTMX(t *testing.T) {
	uiSrv := uiServer(t, "http://127.0.0.1:1") // api base unused here
	resp, err := http.Get(uiSrv.URL + "/static/htmx.min.js")
	if err != nil {
		t.Fatalf("GET /static/htmx.min.js: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", ct)
	}
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(buf.String(), "htmx") {
		t.Error("served asset is not the htmx library")
	}
}
