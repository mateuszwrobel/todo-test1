package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todo/board"
)

// Card api/13 (workplans/scenarios-kanban/api/13-patch-assignee-returns-the-card.md)
// — the PATCH "assignee" field and the contract's name-or-null payload.
//
// The field carries an edit direction, spelled three ways by the contract:
// a roster name (assign), JSON null (clear — the field's null is a
// direction, not an absence), or absent (leave the card alone, which is why
// every pre-assignment leg in this package stayed byte-unchanged through
// this increment). Every card payload answers with assignee present as name
// or null — the DTO row of the parent contract — so the pins here byte-pin
// the full body, and the older key-set pins were flipped honestly from four
// fields to five (additive field, no pin weakened).

// Given a card that is not in Done
// When PATCH /cards/{id} carries {"assignee":"Grace"}
// Then 200 answers with the full card including that assignee and unchanged
// column and position
func TestPatchAssigneeSetReturnsTheCard(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	created := createCardThroughAPI(t, srv.URL, "Buy milk")
	id := int64(created["id"].(float64))
	if id != 1 {
		t.Fatalf("first card id = %d, want 1 — the byte-pins below name it (fresh store, autoincrement starts at 1)", id)
	}

	resp, body := patchCard(t, srv.URL, id, `{"assignee": "Grace"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	const wantBody = `{"id":1,"title":"Buy milk","column":"todo","position":0,"assignee":"Grace"}` + "\n"
	if body != wantBody {
		t.Errorf("body = %s, want the full card carrying the assignee, column and position untouched: %s", body, wantBody)
	}
	card := assertCardKeys(t, body)
	if card["assignee"] != "Grace" {
		t.Errorf("assignee = %v, want Grace", card["assignee"])
	}
	if card["column"] != "todo" || card["position"] != float64(0) {
		t.Errorf("column+position = %v/%v, want todo/0 — an assignee edit moves nothing (card api/13)", card["column"], card["position"])
	}
}

// When a later PATCH carries {"assignee":null}
// Then the card answers unassigned — the null in the body, and the board
// read says the same.
func TestPatchAssigneeNullClearsTheCard(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	created := createCardThroughAPI(t, srv.URL, "Buy milk")
	id := int64(created["id"].(float64))
	if resp, body := patchCard(t, srv.URL, id, `{"assignee": "Grace"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("set: status = %d, want 200 (body %s)", resp.StatusCode, body)
	}

	resp, body := patchCard(t, srv.URL, id, `{"assignee": null}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("clear: status = %d, want 200 — a null is the clear DIRECTION, not an empty body (body %s)", resp.StatusCode, body)
	}
	const wantBody = `{"id":1,"title":"Buy milk","column":"todo","position":0,"assignee":null}` + "\n"
	if body != wantBody {
		t.Errorf("body = %s, want the full card answering unassigned with an explicit null: %s", body, wantBody)
	}

	// The read agrees with the answer: the stored value is NULL again, and
	// the board payload carries the explicit null (name-or-null on every
	// card is contract, not only on the payload that changed).
	boardResp, err := http.Get(srv.URL + "/board")
	if err != nil {
		t.Fatalf("GET /board: %v", err)
	}
	defer boardResp.Body.Close()
	probe, err := io.ReadAll(boardResp.Body)
	if err != nil {
		t.Fatalf("read board body: %v", err)
	}
	if want := `"assignee":null`; !strings.Contains(string(probe), want) {
		t.Errorf("board body %s, want it to carry %s — cleared card reads unassigned", probe, want)
	}
}

// When the field carries a name outside the roster
// Then 422 answers with {"error":"unknown user"} and the card is unchanged.
// The validity rule is owned by the users module through board's Change;
// this leg maps board's outcome, never restates the roster.
func TestPatchAssigneeUnknownNameIsAStated422(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	created := createCardThroughAPI(t, srv.URL, "Buy milk")
	id := int64(created["id"].(float64))

	before := boardSnapshot(t, store)
	resp, body := patchCard(t, srv.URL, id, `{"assignee": "Zoe"}`)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body %s)", resp.StatusCode, body)
	}
	const wantBody = `{"error":"unknown user"}` + "\n"
	if body != wantBody {
		t.Errorf("body = %s, want exactly %s", body, wantBody)
	}
	assertBoardUnchanged(t, store, before)

	// GET probe, byte for byte: the refused card is exactly the card it was
	// — still unassigned, still in place.
	boardResp, err := http.Get(srv.URL + "/board")
	if err != nil {
		t.Fatalf("GET /board: %v", err)
	}
	defer boardResp.Body.Close()
	probe, err := io.ReadAll(boardResp.Body)
	if err != nil {
		t.Fatalf("read board body: %v", err)
	}
	if want := `{"id":1,"title":"Buy milk","column":"todo","position":0,"assignee":null}`; !strings.Contains(string(probe), want) {
		t.Errorf("board body = %s, want the untouched card %s (card api/13: unchanged)", probe, want)
	}
}

// Given a card moved into Done
// When the request carries an assignee (set or clear)
// Then 422 answers with {"error":"cannot edit a done card"} — the same
// stated message the title leg answers, one site for both directions —
// and after the move out of Done the assignee direction succeeds again.
func TestPatchAssigneeOnDoneCardRefused(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	created := createCardThroughAPI(t, srv.URL, "reviewed work")
	id := int64(created["id"].(float64))
	if resp, body := patchCard(t, srv.URL, id, `{"column": "done"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("move to done: status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	before := boardSnapshot(t, store)

	const wantErr = `{"error":"cannot edit a done card"}` + "\n"
	for _, body := range []string{`{"assignee": "Grace"}`, `{"assignee": null}`} {
		resp, got := patchCard(t, srv.URL, id, body)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("PATCH %s status = %d, want 422 (body %s)", body, resp.StatusCode, got)
		}
		if got != wantErr {
			t.Errorf("body = %s, want exactly %s — one message for the freeze, title and assignee alike (body %s)", got, wantErr, body)
		}
	}
	assertBoardUnchanged(t, store, before)

	// The column-only patch still answers 200 (a move is not an edit), and
	// the assignee direction then succeeds — the freeze is the CURRENT
	// column's, not a scar.
	if resp, body := patchCard(t, srv.URL, id, `{"column": "todo"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("move out of done: status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	resp, body := patchCard(t, srv.URL, id, `{"assignee": "Grace"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — the move out of Done unlocked the assignee direction (body %s)", resp.StatusCode, body)
	}
	card := assertCardKeys(t, body)
	if card["assignee"] != "Grace" {
		t.Errorf("assignee = %v, want Grace (body %s)", card["assignee"], body)
	}
}

// When the field names a stranger AND the card sits in Done
// Then 422 answers {"error":"unknown user"} — the unknown user wins.
// Board's Change states the ordering (board/store.go, Change, quoting the
// board workplan): roster validity is validated through the users contract
// before any write, "outranking text rules, not-found, and the done freeze."
// This leg pins that rank as seen from the wire: a stranger beside a frozen
// card is a roster problem, stated before any freeze can be. The same rank
// shows beside a blank title and beside a missing id.
func TestPatchAssigneeUnknownOutranksDoneAndNotFound(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	created := createCardThroughAPI(t, srv.URL, "reviewed work")
	id := int64(created["id"].(float64))
	if resp, body := patchCard(t, srv.URL, id, `{"column": "done"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("move to done: status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	before := boardSnapshot(t, store)

	const wantErr = `{"error":"unknown user"}` + "\n"
	legs := []struct {
		desc   string
		target string
		body   string
	}{
		{"unknown + Done", srv.URL + "/cards/1", `{"assignee": "Zoe"}`},
		{"unknown + Done + title", srv.URL + "/cards/1", `{"assignee": "Zoe", "title": "retitle"}`},
		{"unknown + a card that does not exist", srv.URL + "/cards/99999", `{"assignee": "Zoe"}`},
	}
	for _, leg := range legs {
		req, err := http.NewRequest(http.MethodPatch, leg.target, strings.NewReader(leg.body))
		if err != nil {
			t.Fatalf("%s: new request: %v", leg.desc, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", leg.desc, err)
		}
		got, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: read body: %v", leg.desc, err)
		}
		if resp.StatusCode != http.StatusUnprocessableEntity || string(got) != wantErr {
			t.Errorf("%s: status = %d body = %s, want 422 exactly %s — validity outranks (board Change ordering)", leg.desc, resp.StatusCode, got, wantErr)
		}
	}
	assertBoardUnchanged(t, store, before)
}

// An assignee alone satisfies the contract's "at least one field" clause —
// including its null spelling, which is a direction (clear), not an absence:
// clearing an already-unassigned card is a no-edit edit that answers 200
// with the card unchanged. And the pre-existing empty-change legs (absent
// keys, all-null title/column/position) stay 422 — an absent assignee adds
// no direction.
func TestPatchAssigneeIsAFieldAllByItself(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	created := createCardThroughAPI(t, srv.URL, "Buy milk")
	id := int64(created["id"].(float64))

	resp, body := patchCard(t, srv.URL, id, `{"assignee": null}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — a null assignee carries the clear direction, so the body is not empty (body %s)", resp.StatusCode, body)
	}

	// The no-fields bodies keep their api/09 refusal: an assignee key that
	// is absent contributes no direction.
	for _, body := range []string{`{}`, `{"title": null, "column": null, "position": null}`} {
		req, err := http.NewRequest(http.MethodPatch, srv.URL+"/cards/1", strings.NewReader(body))
		if err != nil {
			t.Fatalf("new PATCH: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PATCH %s: %v", body, err)
		}
		got, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(got), "at least one field is required") {
			t.Errorf("PATCH %s -> status %d body %s, want 422 stating at least one field is required", body, resp.StatusCode, got)
		}
	}
}

// Shape-only, as every other field: an assignee present but neither a string
// nor null is the standing wrong-type transport class, 400 — the roster is
// never consulted here, so no board call and no storage read happen.
func TestPatchAssigneeWrongTypeIsAStated400(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	created := createCardThroughAPI(t, srv.URL, "Buy milk")
	before := boardSnapshot(t, store)

	for _, body := range []string{`{"assignee": 7}`, `{"assignee": true}`, `{"assignee": ["Grace"]}`, `{"assignee": {}}`} {
		resp, got := patchCard(t, srv.URL, int64(created["id"].(float64)), body)
		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(got, "invalid request") {
			t.Errorf("PATCH %s -> status %d body %s, want 400 invalid request (shape-only validation; the roster is board's rule)", body, resp.StatusCode, got)
		}
	}
	assertBoardUnchanged(t, store, before)
}

// The "and/or" combos the contract leaves open land on board's single
// ordering: a name beside a column rides Change's one transaction (edit +
// bottom-append), a name beside a position edits through Change and places
// through Move. A Done card met by any position leg that carries an assignee
// refuses with the freeze before Move runs — an assignee makes it an edit,
// never a bare move.
func TestPatchAssigneeCombosRouteThroughBoardOrdering(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	first := createCardThroughAPI(t, srv.URL, "one")
	second := createCardThroughAPI(t, srv.URL, "two")
	firstID := int64(first["id"].(float64))
	secondID := int64(second["id"].(float64))

	// assignee + column: one Change, card lands at the target's bottom with
	// the assignment kept.
	resp, body := patchCard(t, srv.URL, firstID, `{"assignee": "Grace", "column": "in_progress"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("assignee+column: status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	card := assertCardKeys(t, body)
	if card["assignee"] != "Grace" || card["column"] != "in_progress" || card["position"] != float64(0) {
		t.Errorf("assignee+column body = %s, want Grace at the bottom (position 0) of in_progress", body)
	}

	// assignee + position: Change edits, Move places in the current column.
	resp, body = patchCard(t, srv.URL, secondID, `{"assignee": "Alan", "position": 0}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("assignee+position: status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	card = assertCardKeys(t, body)
	if card["assignee"] != "Alan" || card["column"] != "todo" || card["position"] != float64(0) {
		t.Errorf("assignee+position body = %s, want Alan in todo at position 0", body)
	}

	// a Done card + column + position + assignee: the freeze outranks the
	// placement (Change first, Move never reached).
	third := createCardThroughAPI(t, srv.URL, "frozen")
	thirdID := int64(third["id"].(float64))
	if resp, body := patchCard(t, srv.URL, thirdID, `{"column": "done"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("move to done: status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	resp, body = patchCard(t, srv.URL, thirdID, `{"assignee": "Grace", "column": "todo", "position": 0}`)
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "cannot edit a done card") {
		t.Errorf("done + assignee + position -> status %d body %s, want 422 cannot edit a done card", resp.StatusCode, body)
	}
	if list, err := store.List(); err != nil {
		t.Fatalf("store.List: %v", err)
	} else {
		for _, col := range list {
			for _, c := range col.Cards {
				if c.ID == thirdID && (col.Name != board.Done || c.Assignee != "") {
					t.Errorf("refused combo left the frozen card in %q assigned %q, want untouched in done", col.Name, c.Assignee)
				}
			}
		}
	}
}
