package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"todo/board"
)

// Card api/10 — Delete answers 204.
// Given a card exists with identifier N
// When  a client sends DELETE /cards/N
// Then  the response is 204 with no body
//
//	And a later GET /board omits the card
//
// The wire pin: 204 carries provably zero bytes. The truth after the
// deletion is the later board read — the deleted identifier is gone from
// every column, and the survivors stay contiguous from position 0 because
// board/11 closed the departure's gap in the delete's own transaction. Both
// the HTTP body and the store's List are probed: the contract's read and the
// store must tell the same truth.
func TestDeleteCardAnswers204AndBoardOmitsIt(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(openStore(t), store))
	defer srv.Close()

	first := createCardThroughAPI(t, srv.URL, "first survivor")
	second := createCardThroughAPI(t, srv.URL, "second survivor")
	gone := createCardThroughAPI(t, srv.URL, "deleted bottom card")
	goneID := int64(gone["id"].(float64))

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/cards/"+strconv.FormatInt(goneID, 10), nil)
	if err != nil {
		t.Fatalf("build DELETE request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /cards/%d: %v", goneID, err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204 (body %s)", resp.StatusCode, body)
	}
	if len(body) != 0 {
		t.Errorf("204 must carry no body, got %q", body)
	}

	// The card's stated later read: GET /board on the same listener omits
	// the card. The test mirrors the contract shape locally, as every api
	// read test here does.
	resp, err = http.Get(srv.URL + "/board")
	if err != nil {
		t.Fatalf("GET /board after delete: %v", err)
	}
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read board body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /board status = %d, want 200 (%s)", resp.StatusCode, body)
	}
	var got struct {
		Columns []struct {
			Title string       `json:"title"`
			Cards []board.Card `json:"cards"`
		} `json:"columns"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("GET /board body %s: %v", body, err)
	}
	if len(got.Columns) != 3 {
		t.Fatalf("columns = %d, want the fixed 3 (%s)", len(got.Columns), body)
	}
	var seen []board.Card
	for _, col := range got.Columns {
		for _, card := range col.Cards {
			if card.ID == goneID {
				t.Fatalf("column %q still lists the deleted card %d: %+v", col.Title, goneID, col.Cards)
			}
			seen = append(seen, card)
		}
	}
	want := []board.Card{
		{ID: int64(first["id"].(float64)), Title: "first survivor", Column: board.Todo, Position: 0},
		{ID: int64(second["id"].(float64)), Title: "second survivor", Column: board.Todo, Position: 1},
	}
	if len(seen) != len(want) {
		t.Fatalf("board after delete holds %+v, want exactly %+v", seen, want)
	}
	for i, card := range seen {
		if card != want[i] {
			t.Errorf("survivor %d = %+v, want %+v — positions must stay contiguous after the bottom card leaves", i, card, want[i])
		}
	}

	// The store's own truth matches the wire's, cell for cell.
	list := boardSnapshot(t, store)
	if len(list) != 3 {
		t.Fatalf("store List returned %d columns, want 3", len(list))
	}
	if len(list[0].Cards) != len(want) {
		t.Fatalf("todo column holds %+v, want %+v", list[0].Cards, want)
	}
	for i, card := range list[0].Cards {
		if card != want[i] {
			t.Errorf("store survivor %d = %+v, want %+v", i, card, want[i])
		}
	}
	for _, col := range list[1:] {
		if len(col.Cards) != 0 {
			t.Errorf("column %q holds %d cards, want none", col.Name, len(col.Cards))
		}
	}
}

// Card api/10 — Delete.
// When  a DELETE /todos/{id} arrives for an existing todo
// Then  the response is 204 with no body
//
//	And a later GET does not include it
func TestDeleteExistingReturns204AndVanishesFromList(t *testing.T) {
	store := openStore(t)
	keep, err := store.Create("keep me")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	gone, err := store.Create("delete me")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	srv := httptest.NewServer(NewHandler(store, openBoardStore(t)))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/todos/"+strconv.FormatInt(gone.ID, 10), nil)
	if err != nil {
		t.Fatalf("build DELETE request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /todos/%d: %v", gone.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("204 must carry no body, got %q", body)
	}

	// The card's "later GET does not include it" now reads against the store
	// directly: GET /todos retired with api/01, and GET /board answers the
	// board store, not this one. The deletion's observable truth is List.
	list, err := store.List()
	if err != nil {
		t.Fatalf("store List after delete: %v", err)
	}
	if len(list) != 1 || list[0] != keep {
		t.Fatalf("list after delete = %+v, want exactly [%+v]", list, keep)
	}
}

// Card api/11 — Delete missing todo.
// Given no todo exists with identifier X
// When  a DELETE /todos/X arrives
// Then  the response is 404 with { "error": "no such todo" }
func TestDeleteMissingTodoReturns404(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openStore(t), openBoardStore(t)))
	defer srv.Close()

	const missing = 987654321
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/todos/"+strconv.FormatInt(missing, 10), nil)
	if err != nil {
		t.Fatalf("build DELETE request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /todos/%d: %v", missing, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("DELETE status = %d, want 404", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("body is not the contract's JSON error object: %v", err)
	}
	if body["error"] != "no such todo" {
		t.Errorf(`body = %v, want {"error":"no such todo"}`, body)
	}
}

// A non-numeric id is unparseable input, not an invalid-but-parseable value:
// the contract's DELETE section states 400 with {"error":"invalid request"}.
func TestDeleteNonNumericIdReturns400(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openStore(t), openBoardStore(t)))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/todos/not-a-number", nil)
	if err != nil {
		t.Fatalf("build DELETE request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /todos/not-a-number: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("DELETE status = %d, want 400", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("body is not the contract's JSON error object: %v", err)
	}
	if body["error"] != "invalid request" {
		t.Errorf(`body = %v, want {"error":"invalid request"}`, body)
	}
}
