package api

import (
	"encoding/json"
	"fmt"
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
	srv := httptest.NewServer(NewHandler(store))
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

// Card api/11 — Delete unknown id is a stated 404.
// Given no card exists with identifier Z
// When  a client sends DELETE /cards/Z
// Then  the response is 404 with the error "no such card"
//
// Every unknown identifier reaches the same stated outcome by one path: the
// board's existence read is the delete transaction's first statement
// (board/12), so no unknown class is special and the rejected delete leaves
// the board exactly as it was. The table aims at each class of unknown the
// contract permits — zero, a negative, the never-used high value, one just
// past the highest live id, one far past it, and an already-deleted id (the
// gone card is an unknown card: delete twice, the second delete is this
// scenario). The wording is PATCH's, byte for byte — one wording per error
// class across verbs.
func TestDeleteCardUnknownIDIsStated404(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	kept := createCardThroughAPI(t, srv.URL, "kept card")
	gone := createCardThroughAPI(t, srv.URL, "deleted once")
	goneID := int64(gone["id"].(float64))

	// The gone identifier becomes unknown through a real delete (api/10),
	// so the deleted-twice row needs no contrivance — and this is the
	// baseline the rejected deletes must leave untouched.
	resp, body := deleteCard(t, srv.URL, goneID)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("first DELETE /cards/%d status = %d, want 204 (body %s)", goneID, resp.StatusCode, body)
	}
	before := boardSnapshot(t, store)
	if len(before[0].Cards) != 1 || before[0].Cards[0].ID != int64(kept["id"].(float64)) {
		t.Fatalf("baseline after the real delete = %+v, want only the kept card", before[0].Cards)
	}

	const neverUsed = 987654321
	for _, unknown := range []int64{
		0,              // zero is outside the issued id space (autoincrement starts at 1)
		-7,             // negative
		goneID + 1,     // just past the highest live id
		goneID + 1<<40, // far past it
		neverUsed,      // never seen by this store
		goneID,         // deleted once already — a gone card is an unknown card
	} {
		resp, body := deleteCard(t, srv.URL, unknown)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("DELETE /cards/%d status = %d, want 404 (body %s)", unknown, resp.StatusCode, body)
		}
		assertCreateError(t, []byte(body), "no such card")
	}

	assertBoardUnchanged(t, store, before)
}

// A non-numeric card id is unparseable input, not an unknown identifier:
// the module's standing transport class — 400 with {"error":"invalid
// request"}, the same refusal create and change state, reached before the
// store is touched at all.
func TestDeleteCardNonNumericIDIs400(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	createCardThroughAPI(t, srv.URL, "untouched card")
	before := boardSnapshot(t, store)

	for _, bad := range []string{"not-a-number", "1x"} {
		req, err := http.NewRequest(http.MethodDelete, srv.URL+"/cards/"+bad, nil)
		if err != nil {
			t.Fatalf("build DELETE /cards/%q request: %v", bad, err)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("DELETE /cards/%q: %v", bad, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("DELETE /cards/%q status = %d, want 400 (body %s)", bad, resp.StatusCode, body)
		}
		assertCreateError(t, body, "invalid request")
	}

	assertBoardUnchanged(t, store, before)
}

// deleteCard sends DELETE /cards/{id} and returns the response with its
// body fully read (the 204 pin needs the body's exact emptiness).
func deleteCard(t *testing.T, url string, id int64) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/cards/%d", url, id), nil)
	if err != nil {
		t.Fatalf("new DELETE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /cards/%d: %v", id, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp, string(b)
}
