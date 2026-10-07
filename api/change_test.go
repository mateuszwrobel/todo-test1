package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todo/board"
)

// patchCard sends a PATCH /cards/{id} with the given raw JSON body and
// returns the response plus its body, already read and closed.
func patchCard(t *testing.T, url string, id int64, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/cards/%d", url, id), strings.NewReader(body))
	if err != nil {
		t.Fatalf("new PATCH request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /cards/%d: %v", id, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp, string(b)
}

// createCardThroughAPI creates a card over POST /cards and returns the
// decoded card — tests set up state the way a client does.
func createCardThroughAPI(t *testing.T, srvURL, title string) map[string]interface{} {
	t.Helper()
	resp, err := http.Post(srvURL+"/cards", "application/json",
		strings.NewReader(fmt.Sprintf(`{"title": %q}`, title)))
	if err != nil {
		t.Fatalf("POST /cards (%q): %v", title, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /cards (%q) status = %d, want 201", title, resp.StatusCode)
	}
	var card map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("create body is not card JSON: %v", err)
	}
	return card
}

// assertCardKeys pins that a body is the full Card JSON — exactly the four
// contract fields, nothing more, nothing less.
func assertCardKeys(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var card map[string]interface{}
	if err := json.Unmarshal([]byte(body), &card); err != nil {
		t.Fatalf("body is not card JSON: %v (body %s)", err, body)
	}
	if got := mapKeys(card); !equalKeys(got, []string{"column", "id", "position", "title"}) {
		t.Fatalf("card keys = %v, want exactly the four contract fields id, title, column, position (body %s)", got, body)
	}
	return card
}

// boardSnapshot reads the whole board for before/after unchanged probes.
func boardSnapshot(t *testing.T, store *board.Store) []board.ColumnCards {
	t.Helper()
	list, err := store.List()
	if err != nil {
		t.Fatalf("board store.List: %v", err)
	}
	return list
}

// assertBoardUnchanged is the unchanged-card probe of the error cards: after
// a rejection the List read must show every column exactly as it was.
func assertBoardUnchanged(t *testing.T, store *board.Store, before []board.ColumnCards) {
	t.Helper()
	after := boardSnapshot(t, store)
	if len(after) != len(before) {
		t.Fatalf("board columns changed shape: before %+v after %+v", before, after)
	}
	for i, col := range after {
		if col.Name != before[i].Name {
			t.Fatalf("column %d name changed: %q after, %q before", i, col.Name, before[i].Name)
		}
		if len(col.Cards) != len(before[i].Cards) {
			t.Errorf("column %q holds %d cards after the rejection, want %d (unchanged)",
				col.Name, len(col.Cards), len(before[i].Cards))
			continue
		}
		for j, c := range col.Cards {
			if c != before[i].Cards[j] {
				t.Errorf("column %q position %d = %+v after the rejection, want %+v (unchanged)",
					col.Name, j, c, before[i].Cards[j])
			}
		}
	}
}

// Card api/05 — Patch title returns the card.
// Given a card exists with identifier N
// When  a client patches {"title": "new text"} to /cards/N
// Then  the response is 200
//
//	And the body is the card with the new title, same column, same position,
//	same id
//
// Tested against the real board store: what the wire shows is what the board
// holds. Place and identity survive a rename — the List probe re-reads them.
func TestPatchCardTitleReturnsCard(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	first := createCardThroughAPI(t, srv.URL, "first card")
	second := createCardThroughAPI(t, srv.URL, "second card")
	id := int64(second["id"].(float64))

	resp, body := patchCard(t, srv.URL, id, `{"title": "new text"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	card := assertCardKeys(t, body)
	if card["title"] != "new text" {
		t.Errorf("title = %v, want %q", card["title"], "new text")
	}
	if card["column"] != "todo" {
		t.Errorf("column = %v, want %q — a title change keeps the place", card["column"], "todo")
	}
	if card["position"] != float64(1) {
		t.Errorf("position = %v, want 1 — a title change keeps the position", card["position"])
	}
	if card["id"] != second["id"] {
		t.Errorf("id = %v, want %v — a title change keeps the identifier", card["id"], second["id"])
	}

	// The board itself agrees: both cards in the todo column in position
	// order, the renamed one holding its slot.
	list := boardSnapshot(t, store)
	if len(list) != 3 || list[0].Name != "todo" {
		t.Fatalf("board list = %+v, want the three fixed columns todo-first", list)
	}
	want := []struct {
		title    string
		position int
	}{
		{"first card", 0},
		{"new text", 1},
	}
	if len(list[0].Cards) != len(want) {
		t.Fatalf("todo column holds %+v, want the two cards", list[0].Cards)
	}
	for i, c := range list[0].Cards {
		if c.Title != want[i].title || c.Position != want[i].position {
			t.Errorf("todo position %d = %+v, want %q at position %d", i, c, want[i].title, want[i].position)
		}
	}

	// The untouched card's identifier survived too — the patch touched only
	// the named card.
	if list[0].Cards[0].ID != int64(first["id"].(float64)) {
		t.Errorf("first card id = %d, want %v (unchanged)", list[0].Cards[0].ID, first["id"])
	}
}

// Card api/05 — Amendment 2026-10-07 (done freeze: title edits refused on
// cards in Done; edit affordance absent on Done cards — user decision). This
// pin flips honestly from its earlier claim ("a card in the done column
// stays fully editable through the same endpoint", which asserted 200 +
// rename-in-place for a done card's title): the freeze is contract-level, so
// the wire answers a stated refusal. The class choice mirrors the retired
// todo app's frozen-todo convention — 422 with one stated message, there
// "cannot edit a done todo" (api/change.go at 8195fde), here restated for
// cards and single-sourced through the one board.ErrDoneFrozen mapping:
//
// Given a card in the "Done" column
// When  a client patches a title for it
// Then  the response is the stated 422 refusal and the board is unchanged
//
//	And after it is moved out of Done the title patch succeeds again
func TestPatchCardTitleOnDoneCardRefused(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	card := createCardThroughAPI(t, srv.URL, "reviewed work")
	id := int64(card["id"].(float64))

	// Move it to done first (the column direction the same handler carries —
	// a move is not an edit, so it still answers 200).
	if resp, body := patchCard(t, srv.URL, id, `{"column": "done"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("move to done: status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	before := boardSnapshot(t, store)

	// Title present — even alongside a column that would carry it out of
	// done — the frozen refusal answers (the todo app's leg shape, kept: the
	// freeze reads the card's CURRENT column, so the combined request too).
	for _, body := range []string{`{"title": "reviewed work (final)"}`, `{"title": "reviewed work (final)", "column": "todo"}`} {
		resp, got := patchCard(t, srv.URL, id, body)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("PATCH %s status = %d, want 422 (body %s)", body, resp.StatusCode, got)
		}
		assertCreateError(t, []byte(got), "cannot edit a done card")
	}

	// Board-unchanged probe: the refusals touched nothing, cell for cell.
	assertBoardUnchanged(t, store, before)

	// "And the card becomes editable after it is moved out of Done": the
	// column-only patch answers 200, and the title direction then succeeds —
	// the rename lands in place in To Do.
	if resp, body := patchCard(t, srv.URL, id, `{"column": "todo"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("move out of done: status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	resp, body := patchCard(t, srv.URL, id, `{"title": "reviewed work (final)"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — the move-out of Done unlocked the title direction (body %s)", resp.StatusCode, body)
	}
	updated := assertCardKeys(t, body)
	if updated["title"] != "reviewed work (final)" || updated["column"] != "todo" {
		t.Errorf("unlocked patch returned %+v, want the renamed card in todo", updated)
	}
	list := boardSnapshot(t, store)
	if len(list[0].Cards) != 1 || list[0].Cards[0].Title != "reviewed work (final)" {
		t.Errorf("todo column holds %+v, want the one renamed card", list[0].Cards)
	}
}

// The contract lets one request carry title and column together: a single
// PATCH applies both directions — the new text and the move to the bottom
// of the target column — and answers the card as it now stands.
func TestPatchCardTitleAndColumnTogether(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	moved := createCardThroughAPI(t, srv.URL, "old title")
	kept := createCardThroughAPI(t, srv.URL, "stays put")
	id := int64(moved["id"].(float64))

	resp, body := patchCard(t, srv.URL, id, `{"title": "new title", "column": "in_progress"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	card := assertCardKeys(t, body)
	if card["title"] != "new title" {
		t.Errorf("title = %v, want %q", card["title"], "new title")
	}
	if card["column"] != "in_progress" {
		t.Errorf("column = %v, want in_progress", card["column"])
	}
	if card["position"] != float64(0) {
		t.Errorf("position = %v, want 0 — the bottom of the empty in_progress column", card["position"])
	}

	// Both directions landed in the store, and the source column closed
	// its gap: the kept card now sits at todo position 0.
	list := boardSnapshot(t, store)
	if len(list[1].Cards) != 1 || list[1].Cards[0].Title != "new title" || list[1].Cards[0].Position != 0 {
		t.Errorf("in_progress column holds %+v, want the moved card at position 0", list[1].Cards)
	}
	if len(list[0].Cards) != 1 || list[0].Cards[0].ID != int64(kept["id"].(float64)) || list[0].Cards[0].Position != 0 {
		t.Errorf("todo column holds %+v, want only the kept card at position 0", list[0].Cards)
	}
}

// The title direction of PATCH runs into board's text rule — the same rule
// create hits — so the refusal bodies must read identically across verbs:
// single wording per error class. Blank → "title is required" (create's
// api/03 wording), over-limit → the limit statement formatted from
// board.MaxTextLen (create's api/04 wording). Both pinned byte-for-byte
// against the POST /cards refusal bodies, and the board left unchanged.
func TestPatchCardTitleRefusalsWordedLikeCreate(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	card := createCardThroughAPI(t, srv.URL, "solid card")
	id := int64(card["id"].(float64))
	before := boardSnapshot(t, store)

	// Blank title → the required refusal, create's exact wording.
	for _, title := range []string{"", " \t\n "} {
		resp, body := patchCard(t, srv.URL, id, fmt.Sprintf(`{"title": %q}`, title))
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("blank title %q: status = %d, want 422 (body %s)", title, resp.StatusCode, body)
		}
		assertCreateError(t, []byte(body), "title is required")
	}

	// Over-limit title → the limit statement, word-for-word what create
	// answers for the same input.
	overLimit := strings.Repeat("x", board.MaxTextLen+1)
	_, createBody := func() (*http.Response, string) {
		resp, err := http.Post(srv.URL+"/cards", "application/json",
			strings.NewReader(fmt.Sprintf(`{"title": %q}`, overLimit)))
		if err != nil {
			t.Fatalf("POST /cards: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("create over-limit status = %d, want 422", resp.StatusCode)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, string(b)
	}()
	resp, patchBody := patchCard(t, srv.URL, id, fmt.Sprintf(`{"title": %q}`, overLimit))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("over-limit: status = %d, want 422 (body %s)", resp.StatusCode, patchBody)
	}
	assertCreateError(t, []byte(patchBody), fmt.Sprintf("title exceeds the %d character limit", board.MaxTextLen))
	if patchBody != createBody {
		t.Errorf("over-limit bodies differ: PATCH %s vs POST %s — one wording per error class across verbs", patchBody, createBody)
	}

	assertBoardUnchanged(t, store, before)
}

// Card api/07 — Patch unknown id is a stated 404.
// Given no card exists with identifier Z
// When  a client patches any fields to /cards/Z
// Then  the response is 404 with the error "no such card"
//
// Any fields — title alone, column alone, both: the identifier check is the
// board transaction's first read, so every variant reaches the same stated
// outcome and the board is exactly as it was (List probe).
func TestPatchCardUnknownIDIsStated404(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	createCardThroughAPI(t, srv.URL, "the only card")
	before := boardSnapshot(t, store)

	const missingID = 999
	for _, body := range []string{
		`{"title": "renamed away"}`,
		`{"column": "done"}`,
		`{"title": "both", "column": "in_progress"}`,
	} {
		resp, got := patchCard(t, srv.URL, missingID, body)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d for body %s, want 404 (body %s)", resp.StatusCode, body, got)
		}
		assertCreateError(t, []byte(got), "no such card")
	}

	assertBoardUnchanged(t, store, before)
}

// Card api/08 — Patch invalid column is a stated 422.
// Given a card exists with identifier N
// When  a client patches {"column": "someday"} to /cards/N
// Then  the response is 422 with the error "invalid column"
//
//	And the card is unchanged
//
// The enum check is board's (api validates shape only), and it runs before
// any write — the List probe shows the card exactly where it was. A string
// of the right type but outside the enum is the rule-refusal class; a
// non-string column value was a shape violation (400, pinned apart).
func TestPatchCardInvalidColumnIsStated422(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	card := createCardThroughAPI(t, srv.URL, "steady card")
	id := int64(card["id"].(float64))
	before := boardSnapshot(t, store)

	for _, column := range []string{"someday", "BACKLOG", ""} {
		resp, got := patchCard(t, srv.URL, id, fmt.Sprintf(`{"column": %q}`, column))
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("column %q: status = %d, want 422 (body %s)", column, resp.StatusCode, got)
		}
		assertCreateError(t, []byte(got), "invalid column")
	}

	assertBoardUnchanged(t, store, before)
	if list := boardSnapshot(t, store); len(list[0].Cards) != 1 || list[0].Cards[0].ID != id || list[0].Cards[0].Position != 0 {
		t.Errorf("card changed after the refusal: %+v, want it alone in todo at position 0", list[0].Cards)
	}
}

// Card api/09 — Patch empty body is a stated 422.
// Given a card exists with identifier N
// When  a client patches an empty JSON object to /cards/N
// Then  the response is 422 stating that at least one field is required
//
//	And the card is unchanged
//
// The same refusal class covers every body that supplies no field: absent
// keys, keys whose value is JSON null, no body at all, and well-formed JSON
// that is not an object — all read as "field absent", so the at-least-one-
// field clause is what fails. Nothing reaches the store; the List probe
// shows the card exactly as it was.
func TestPatchCardEmptyBodyIsStated422(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	card := createCardThroughAPI(t, srv.URL, "card standing pat")
	id := int64(card["id"].(float64))
	before := boardSnapshot(t, store)

	for _, body := range []string{
		`{}`,                              // the card's empty JSON object
		``,                                // no body at all
		`{"title": null, "column": null}`, // keys present but null — no field supplied
		`null`,                            // well-formed JSON, not an object
		`"just a string"`,                 // same: carries no fields
	} {
		resp, got := patchCard(t, srv.URL, id, body)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d for body %q, want 422 (body %s)", resp.StatusCode, body, got)
		}
		assertCreateError(t, []byte(got), "at least one field is required")
	}

	assertBoardUnchanged(t, store, before)
	if list := boardSnapshot(t, store); len(list[0].Cards) != 1 || list[0].Cards[0].Title != "card standing pat" {
		t.Errorf("card changed after the empty-body refusal: %+v", list[0].Cards)
	}
}

// Shape violations are transport errors, not rule refusals — the module's
// convention kept from create ("Malformed JSON is a transport error: 400
// invalid request, distinct from the 422 rule refusals"; a field present but
// not a string is the same class there). A non-numeric id is unparseable
// input: the contract's blanket 400 rule, kept from the todo handlers.
func TestPatchCardMalformedBodyIs400(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	card := createCardThroughAPI(t, srv.URL, "untouched card")
	id := int64(card["id"].(float64))
	before := boardSnapshot(t, store)

	for _, body := range []string{
		`{"title": "unterminated`, // broken JSON
		`{"title": [1,2]}`,        // title present but not a string
		`{"column": true}`,        // column present but not a string
	} {
		resp, got := patchCard(t, srv.URL, id, body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d for body %s, want 400 (body %s)", resp.StatusCode, body, got)
		}
		assertCreateError(t, []byte(got), "invalid request")
	}

	// Non-numeric id: the same transport class.
	resp, got := func() (*http.Response, string) {
		r, err := http.NewRequest(http.MethodPatch, srv.URL+"/cards/not-a-number", strings.NewReader(`{"title": "x"}`))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		rp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("PATCH /cards/not-a-number: %v", err)
		}
		defer rp.Body.Close()
		b, err := io.ReadAll(rp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return rp, string(b)
	}()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-numeric id: status = %d, want 400 (body %s)", resp.StatusCode, got)
	}
	assertCreateError(t, []byte(got), "invalid request")

	assertBoardUnchanged(t, store, before)
}
