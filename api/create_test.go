package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
