package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todo/board"
)

// Card api/15 — Move with a filter-relative slot.
// Source scenario, quoted:
//
//	Given a filtered view whose visible cards sit among hidden ones
//	When  PATCH /cards/{id} carries {"column":"doing","slot":1,"within":"Grace"}
//	Then  200 answers with the moved card and an absolute position that puts
//	      it at slot 1 among Grace's cards with the hidden cards unmoved
//	When  the body carries "position" and "slot" together, or "slot" without
//	      "within"
//	Then  422 answers with a stated error
//	When  "within" names a stranger to the roster
//	Then  422 answers with {"error":"unknown user"}
//
// ("doing" is the card's shorthand for a target column; the contract's
// columns are todo/in_progress/done, and the legs below address them by
// their real names.) The pair stands INSTEAD of position, so every
// absolute-position leg this verb already had stays byte-frozen — those
// pins live in change_test.go/change_move_test.go and did not move with
// this increment.

// slotFixture: todo [g1(Grace) 0, hidden(—) 1, g2(Grace) 2] plus xmove
// alone in in_progress (position 0) — the "visible cards sit among hidden
// ones" view for Grace is g1, g2 with hidden between them.
func slotFixture(t *testing.T) *slotFixtureBoard {
	t.Helper()
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	t.Cleanup(srv.Close)
	for _, title := range []string{"g1", "hidden", "g2"} {
		createCardThroughAPI(t, srv.URL, title)
	}
	for _, id := range []int64{1, 3} {
		if resp, body := patchCard(t, srv.URL, id, `{"assignee": "Grace"}`); resp.StatusCode != http.StatusOK {
			t.Fatalf("assign %d: status %d body %s", id, resp.StatusCode, body)
		}
	}
	moved := createCardThroughAPI(t, srv.URL, "xmove")
	movedID := int64(moved["id"].(float64))
	if movedID != 4 {
		t.Fatalf("fixture id = %d, want 4", movedID)
	}
	if resp, body := patchCard(t, srv.URL, movedID, `{"column": "in_progress"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("move to in_progress: status %d body %s", resp.StatusCode, body)
	}
	return &slotFixtureBoard{store: store, srv: srv.URL, movedID: movedID}
}

type slotFixtureBoard struct {
	store   *board.Store
	srv     string
	movedID int64
}

// todoTitles reads the stored todo sequence and positions through the
// contract's own store — the hidden-cards-unmoved probe: their RELATIVE
// order (the card's "unmoved") and whole-column contiguity.
func todoTitles(t *testing.T, store *board.Store) []string {
	t.Helper()
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, col := range list {
		if col.Name != board.Todo {
			continue
		}
		var seq []string
		for pos, c := range col.Cards {
			if c.Position != pos {
				t.Fatalf("todo position gap at %d: %+v", pos, col.Cards)
			}
			seq = append(seq, c.Title)
		}
		return seq
	}
	t.Fatalf("no todo column")
	return nil
}

// "When PATCH carries {"column":"todo","slot":1,"within":"Grace"} / Then
// 200 answers with the moved card and an absolute position that puts it at
// slot 1 among Grace's cards with the hidden cards unmoved."
//
// Resolution, spelled out: target todo minus the moved card is
// [g1(G)@0, hidden(—)@1, g2(G)@2]; matching cards [g1, g2]; slot 1 = the
// earliest absolute index with one Grace card before it = right after g1 =
// position 1. The final todo sequence is [g1, xmove, hidden, g2]: hidden
// stays between the two Grace cards in RELATIVE order and g1 < g2 keeps
// its own — cell-for-cell order kept, positions renormalized contiguous.
func TestPatchSlotPairResolvesAbsolutePosition(t *testing.T) {
	f := slotFixture(t)

	resp, body := patchCard(t, f.srv, f.movedID, `{"column": "todo", "slot": 1, "within": "Grace"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	card := assertCardKeys(t, body)
	if card["column"] != "todo" || card["position"] != float64(1) {
		t.Errorf("body = %s, want the moved card in todo at ABSOLUTE position 1 (the resolved slot)", body)
	}
	if card["assignee"] != nil {
		t.Errorf("assignee = %v, want null — the pair places, it does not assign", card["assignee"])
	}
	if seq := todoTitles(t, f.store); !equalStrings(seq, []string{"g1", "xmove", "hidden", "g2"}) {
		t.Errorf("stored todo sequence %v, want [g1 xmove hidden g2]", seq)
	}

	// slot 0 is the column's FRONT among the matching set's window: after
	// the previous move the sequence is [g1, xmove, hidden, g2]; moving
	// xmove to Grace-slot 0 → before g1 → absolute 0.
	resp, body = patchCard(t, f.srv, f.movedID, `{"column": "todo", "slot": 0, "within": "Grace"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("slot 0: status = %d (body %s)", resp.StatusCode, body)
	}
	if card := assertCardKeys(t, body); card["position"] != float64(0) {
		t.Errorf("slot 0 body = %s, want absolute position 0", body)
	}
	if seq := todoTitles(t, f.store); !equalStrings(seq, []string{"xmove", "g1", "hidden", "g2"}) {
		t.Errorf("stored todo sequence %v, want [xmove g1 hidden g2]", seq)
	}
}

// The pair WITHOUT a stated column re-slots within the card's current
// column — board resolves the current column inside the transaction, so
// validity keeps its rank over not-found without a transport read.
func TestPatchSlotPairNoColumnReSlotsInCurrentColumn(t *testing.T) {
	f := slotFixture(t)

	// g2 is Grace at todo 2. Removing it leaves [g1, hidden] with matching
	// [g1@0]; slot 1 = at/past the matching count → after the last match →
	// absolute 1 → [g1, g2, hidden].
	resp, body := patchCard(t, f.srv, 3, `{"slot": 1, "within": "Grace"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	card := assertCardKeys(t, body)
	if card["column"] != "todo" || card["position"] != float64(1) {
		t.Errorf("body = %s, want g2 in todo at absolute position 1", body)
	}
	if seq := todoTitles(t, f.store); !equalStrings(seq, []string{"g1", "g2", "hidden"}) {
		t.Errorf("stored todo sequence %v, want [g1 g2 hidden]", seq)
	}

	// The sentinel half: xmove (unassigned) to slot 0 among UNASSIGNED
	// cards of todo. After the reorder todo is [g1(G), g2(G), hidden(—)];
	// the only match is hidden@2; slot 0 → index 0 → [xmove? no — xmove is
	// in in_progress] — stated column below.
	resp, body = patchCard(t, f.srv, f.movedID, `{"column": "todo", "slot": 0, "within": "unassigned"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sentinel leg: status = %d (body %s)", resp.StatusCode, body)
	}
	if seq := todoTitles(t, f.store); !equalStrings(seq, []string{"xmove", "g1", "g2", "hidden"}) {
		t.Errorf("sentinel sequence %v, want [xmove g1 g2 hidden] — slot 0 is the earliest position with zero unassigned cards before it", seq)
	}
}

// slot 0 into a column holding no matching cards places the card at that
// column's front (board/19's final clause seen from the wire): done holds
// no Grace cards (nothing holds one), so a slot-0 move there lands at 0.
func TestPatchSlotZeroIntoNoMatchColumnFronts(t *testing.T) {
	f := slotFixture(t)

	resp, body := patchCard(t, f.srv, 1, `{"column": "done", "slot": 0, "within": "Grace"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	if card := assertCardKeys(t, body); card["column"] != "done" || card["position"] != float64(0) {
		t.Errorf("body = %s, want done position 0 — the column's front", body)
	}
}

// A Done card answers the pair with a PLACEMENT, never the freeze: the
// move directions on done cards stay open (board/16), the filtered move
// included — it carries no title and no assignee.
func TestPatchSlotPairOnDoneCardIsAPlacement(t *testing.T) {
	f := slotFixture(t)

	if resp, body := patchCard(t, f.srv, 3, `{"column": "done"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("into done: status %d body %s", resp.StatusCode, body)
	}
	resp, body := patchCard(t, f.srv, 3, `{"column": "todo", "slot": 1, "within": "Grace"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("filtered move out of done: status = %d (body %s) — placement is not an edit", resp.StatusCode, body)
	}
	if seq := todoTitles(t, f.store); !equalStrings(seq, []string{"g1", "g2", "hidden"}) {
		t.Errorf("todo sequence %v, want [g1 g2 hidden] — g2 back at Grace-slot 1", seq)
	}
}

// The stated-shape refusals, card api/15 clause 2 ("position and slot
// together, or slot without within → 422 with a stated error") plus the
// pair's other half and the edit-direction company the contract leaves
// unaccepted ("the pair is a MOVE" — the amendment adds no slot+edit
// combination, so the pair refuses company rather than inventing one):
// every refusal answers BEFORE any store call — board byte-unchanged.
func TestPatchSlotPairShapeRefusals(t *testing.T) {
	_, f := slotFixtureForFreeze(t)

	legs := []struct {
		body string
		want string
	}{
		{`{"column": "todo", "position": 1, "slot": 1, "within": "Grace"}`, `{"error":"position and slot are mutually exclusive"}` + "\n"},
		{`{"column": "todo", "slot": 1}`, `{"error":"slot requires within"}` + "\n"},
		{`{"column": "todo", "within": "Grace"}`, `{"error":"within requires slot"}` + "\n"},
		{`{"column": "todo", "slot": 1, "within": "Grace", "title": "renamed"}`, `{"error":"cannot combine slot with title or assignee"}` + "\n"},
		{`{"column": "todo", "slot": 1, "within": "Grace", "assignee": "Ada"}`, `{"error":"cannot combine slot with title or assignee"}` + "\n"},
		{`{"column": "todo", "slot": 1, "within": "unassigned", "assignee": null}`, `{"error":"cannot combine slot with title or assignee"}` + "\n"},
		{`{"slot": null, "within": null}`, `{"error":"at least one field is required"}` + "\n"},
	}
	before := boardSnapshot(t, f.store)
	for _, leg := range legs {
		resp, got := patchCard(t, f.srv, f.movedID, leg.body)
		if resp.StatusCode != http.StatusUnprocessableEntity || got != leg.want {
			t.Errorf("PATCH %s -> status %d body %s, want 422 exactly %s", leg.body, resp.StatusCode, got, leg.want)
		}
	}
	assertBoardUnchanged(t, f.store, before)
}

// The semantic refusals travel as board's typed outcomes through the one
// mapping site: negative slot → ErrInvalidSlot → 422 stated; an unknown
// "within" → ErrUnknownAssignee → the shared unknown-user body, outranking
// even the slot guard and the missing target (validity-first rank).
func TestPatchSlotPairSemanticRefusals(t *testing.T) {
	f := slotFixture(t)
	before := boardSnapshot(t, f.store)

	legs := []struct {
		target int64
		body   string
		status int
		want   string
	}{
		{f.movedID, `{"column": "todo", "slot": -1, "within": "Grace"}`, http.StatusUnprocessableEntity, `{"error":"invalid slot"}` + "\n"},
		{f.movedID, `{"column": "todo", "slot": 1, "within": "Zoe"}`, http.StatusUnprocessableEntity, `{"error":"unknown user"}` + "\n"},
		{f.movedID, `{"column": "todo", "slot": 1, "within": "grace"}`, http.StatusUnprocessableEntity, `{"error":"unknown user"}` + "\n"},
		{f.movedID, `{"column": "todo", "slot": 1, "within": "unassigned "}`, http.StatusUnprocessableEntity, `{"error":"unknown user"}` + "\n"},
		{99999, `{"column": "todo", "slot": 0, "within": "Zoe"}`, http.StatusUnprocessableEntity, `{"error":"unknown user"}` + "\n"},
		{99999, `{"column": "todo", "slot": -1, "within": "Grace"}`, http.StatusUnprocessableEntity, `{"error":"invalid slot"}` + "\n"},
		{99999, `{"column": "todo", "slot": 0, "within": "Grace"}`, http.StatusNotFound, `{"error":"no such card"}` + "\n"},
		{f.movedID, `{"column": "someday", "slot": 0, "within": "Grace"}`, http.StatusUnprocessableEntity, `{"error":"invalid column"}` + "\n"},
	}
	for _, leg := range legs {
		resp, got := patchCard(t, f.srv, leg.target, leg.body)
		if resp.StatusCode != leg.status || got != leg.want {
			t.Errorf("PATCH /cards/%d %s -> status %d body %s, want %d exactly %s",
				leg.target, leg.body, resp.StatusCode, got, leg.status, leg.want)
		}
	}
	assertBoardUnchanged(t, f.store, before)
}

// Shape-only, the standing classes: a non-integer slot or a non-string
// within is the wrong-type/non-number transport class, 400 — the same
// split position and column already draw; a well-formed NEGATIVE integer
// is shape-legal and travels to board (the semantic leg above).
func TestPatchSlotPairWrongTypesAre400(t *testing.T) {
	f := slotFixture(t)
	before := boardSnapshot(t, f.store)

	for _, body := range []string{
		`{"slot": 1.5, "within": "Grace"}`,
		`{"slot": "1", "within": "Grace"}`,
		`{"slot": 1, "within": 7}`,
		`{"slot": 1, "within": true}`,
		`{"slot": 1, "within": ["Grace"]}`,
	} {
		resp, got := patchCard(t, f.srv, f.movedID, body)
		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(got, "invalid request") {
			t.Errorf("PATCH %s -> status %d body %s, want 400 invalid request", body, resp.StatusCode, got)
		}
	}
	assertBoardUnchanged(t, f.store, before)
}

// slotFixtureForFreeze = slotFixture plus the moved card parked in todo
// and the full-board payload pinned byte-exact — the absolute-position
// frozen legs proof: with the pair in the contract, the shipped
// {"position":N} and {"column":C,"position":N} bodies move exactly as they
// did before KW10.
func slotFixtureForFreeze(t *testing.T) (string, *slotFixtureBoard) {
	t.Helper()
	f := slotFixture(t)
	if resp, body := patchCard(t, f.srv, f.movedID, `{"column": "todo"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("move to todo: status %d body %s", resp.StatusCode, body)
	}
	full := `{"columns":[{"title":"To Do","cards":[` +
		`{"id":1,"title":"g1","column":"todo","position":0,"assignee":"Grace"},` +
		`{"id":2,"title":"hidden","column":"todo","position":1,"assignee":null},` +
		`{"id":3,"title":"g2","column":"todo","position":2,"assignee":"Grace"},` +
		`{"id":4,"title":"xmove","column":"todo","position":3,"assignee":null}]},` +
		`{"title":"In Progress","cards":[]},` +
		`{"title":"Done","cards":[]}]}` + "\n"
	return full, f
}

// Frozen leg (deliver 3): the absolute-position move leg answers the
// shipped byte contract — {"column","position"} still places at exactly
// that index, and a no-param GET /board is byte-identical around it.
func TestPatchAbsolutePositionLegStaysByteFrozen(t *testing.T) {
	full, f := slotFixtureForFreeze(t)

	resp, body := getBoard(t, f.srv+"/board")
	if resp != http.StatusOK || body != full {
		t.Fatalf("GET /board = %d %s, want 200 exactly %s", resp, body, full)
	}

	// column+position moves to the stated absolute index (api/06's leg).
	if resp, body := patchCard(t, f.srv, f.movedID, `{"column": "in_progress", "position": 0}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("absolute move: status %d body %s", resp.StatusCode, body)
	} else if card := assertCardKeys(t, body); card["column"] != "in_progress" || card["position"] != float64(0) {
		t.Errorf("absolute move body = %s, want in_progress position 0", body)
	}
	if seq := todoTitles(t, f.store); !equalStrings(seq, []string{"g1", "hidden", "g2"}) {
		t.Errorf("todo after absolute move %v, want [g1 hidden g2] — gap closed, order kept", seq)
	}
}

// equalStrings compares title sequences.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
