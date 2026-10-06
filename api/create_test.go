package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todo/board"
)

// Card api/02 — Create returns the card.
// Given an open board
// When  a client posts {"title": "Buy milk"} to /cards
// Then  the response is 201
//
//	And the body is the created card with column "todo", the bottom
//	position, and a fresh id
//
// Tested against the real board store: persistence is proven, not faked.
func TestPostCardsCreates(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(openStore(t), store))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/cards", "application/json",
		strings.NewReader(`{"title": "Buy milk"}`))
	if err != nil {
		t.Fatalf("POST /cards: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	t.Logf("POST /cards body: %s", body)

	// Decode generically so the JSON field names themselves are asserted,
	// not just a struct that happens to tolerate extra keys.
	var card map[string]interface{}
	if err := json.Unmarshal(body, &card); err != nil {
		t.Fatalf("body is not card JSON: %v (body %s)", err, body)
	}
	if got := mapKeys(card); !equalKeys(got, []string{"column", "id", "position", "title"}) {
		t.Fatalf("card keys = %v, want exactly the four contract fields id, title, column, position (body %s)", got, body)
	}
	if got := card["title"]; got != "Buy milk" {
		t.Errorf("title = %v, want %q", got, "Buy milk")
	}
	if got := card["column"]; got != "todo" {
		t.Errorf("column = %v, want %q (created at the bottom of To Do)", got, "todo")
	}
	if got := card["position"]; got != float64(0) {
		t.Errorf("position = %v, want 0 (bottom of the empty todo column)", got)
	}
	firstID, ok := card["id"].(float64)
	if !ok || firstID < 1 {
		t.Fatalf("id = %v, want a fresh positive identifier", card["id"])
	}

	// The bottom position once more: the next created card sits under this
	// one (position 1), with a fresh — different, store-assigned — id.
	resp2, err := http.Post(srv.URL+"/cards", "application/json",
		strings.NewReader(`{"title": "Buy bread"}`))
	if err != nil {
		t.Fatalf("POST /cards (second): %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("second create status = %d, want 201", resp2.StatusCode)
	}
	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var second map[string]interface{}
	if err := json.Unmarshal(body2, &second); err != nil {
		t.Fatalf("second body is not card JSON: %v (body %s)", err, body2)
	}
	if got := second["position"]; got != float64(1) {
		t.Errorf("second card position = %v, want 1 (the new bottom of the column)", got)
	}
	secondID, ok := second["id"].(float64)
	if !ok || secondID < 1 || secondID == firstID {
		t.Errorf("second card id = %v, want a fresh identifier distinct from the first (%v)", second["id"], firstID)
	}

	// Persisted through the real store: the todo column holds both cards in
	// position order; the other columns stay empty.
	list, err := store.List()
	if err != nil {
		t.Fatalf("board store.List: %v", err)
	}
	if len(list) != 3 || list[0].Name != "todo" {
		t.Fatalf("board list = %+v, want the three fixed columns todo-first", list)
	}
	wantTitles := []string{"Buy milk", "Buy bread"}
	if len(list[0].Cards) != len(wantTitles) {
		t.Fatalf("todo column holds %+v, want the two created cards", list[0].Cards)
	}
	for i, c := range list[0].Cards {
		if c.Title != wantTitles[i] || c.Position != i {
			t.Errorf("todo position %d = %+v, want title %q at position %d", i, c, wantTitles[i], i)
		}
	}
}

// The board trims the title before storing (board's text rule), and the 201
// body is the created card — the stored card. So a title with surrounding
// whitespace comes back trimmed, and that is what the board holds.
func TestPostCardsReturnsTrimmedTitle(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(openStore(t), store))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/cards", "application/json",
		strings.NewReader(`{"title": "  Buy milk \t"}`))
	if err != nil {
		t.Fatalf("POST /cards: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var card map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("body is not card JSON: %v", err)
	}
	if card["title"] != "Buy milk" {
		t.Errorf("created title = %v, want the trimmed %q", card["title"], "Buy milk")
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("board store.List: %v", err)
	}
	if len(list[0].Cards) != 1 || list[0].Cards[0].Title != "Buy milk" {
		t.Errorf("board holds %+v, want the one trimmed card", list[0].Cards)
	}
}

// Card api/03 — Create blank title is a stated 422.
// Given an open board
// When  a client posts an empty or whitespace-only title to /cards
// Then  the response is 422 with the error "title is required"
//
//	And the board is unchanged
//
// The contract states the same refusal when the title is merely absent: an
// empty object, no body at all, or well-formed JSON that is not an object.
// The refusal is board's required outcome stated once at the mapping site —
// the blank rule has one owner, not two.
func TestPostCardsBlankOrAbsentTitleIsStated422(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(openStore(t), store))
	defer srv.Close()

	for _, body := range []string{
		`{"title": ""}`,       // empty
		`{"title": " \t\n "}`, // whitespace-only
		`{}`,                  // absent title key
		``,                    // no body at all
		`"just a string"`,     // well-formed JSON, not an object — carries no title
	} {
		resp, err := http.Post(srv.URL+"/cards", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST /cards (%s): %v", body, err)
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d for body %s, want 422", resp.StatusCode, body)
		}
		assertCreateError(t, respBody, "title is required")
	}

	// And the board is unchanged: every column still holds nothing.
	assertBoardEmpty(t, store)
}

// Malformed JSON stays a transport error — 400 `{"error": "invalid request"}`
// — the module's convention from the retired POST /todos handler ("Malformed
// JSON is a transport error: 400 invalid request, distinct from the 422 rule
// refusals"), kept for POST /cards. A title that is present but not a string
// is the same transport class: the body is not the contract's shape (the
// todo handler answered that body 400 through its struct decode).
func TestPostCardsMalformedJSONIs400(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(openStore(t), store))
	defer srv.Close()

	for _, body := range []string{
		`{"title": `,   // truncated mid-value — the body the todo-era pin exercised
		`{bad`,         // broken syntax
		`{"title": 5}`, // present but not a string — not the contract's body shape
	} {
		resp, err := http.Post(srv.URL+"/cards", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST /cards (%s): %v", body, err)
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d for body %s, want 400", resp.StatusCode, body)
		}
		assertCreateError(t, respBody, "invalid request")
	}

	assertBoardEmpty(t, store)
}

// assertCreateError asserts the contract's one error shape carries the stated
// message verbatim.
func assertCreateError(t *testing.T, body []byte, want string) {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("error body is not JSON: %v (body %s)", err, body)
	}
	if got["error"] != want {
		t.Errorf("error = %q, want %q (body %s)", got["error"], want, body)
	}
}

// assertBoardEmpty is the unchanged-board probe: after a rejection the List
// read must show every column exactly as it was — nothing created.
func assertBoardEmpty(t *testing.T, store *board.Store) {
	t.Helper()
	list, err := store.List()
	if err != nil {
		t.Fatalf("board store.List: %v", err)
	}
	for _, col := range list {
		if len(col.Cards) != 0 {
			t.Errorf("column %q holds %+v after a rejected create, want unchanged (empty)", col.Name, col.Cards)
		}
	}
}
