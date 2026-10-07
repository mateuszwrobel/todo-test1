package board

import (
	"errors"
	"testing"
)

// Card board/19 — Filtered-slot move keeps whole-board order, and the
// filtered read beside it (the read contract the amendment states: matching
// cards only, stored order and positions unchanged; keyword = exact roster
// name or the unassigned sentinel).
//
// Source, quoted from workplan_board_store.md (Database → Data Flow):
// "Filtered reads select `assignee = <name>` or `assignee IS NULL` (keyword
// \"unassigned\"); stored positions and order pass through unchanged. The
// filtered-slot move resolves its slot against the matching rows of the
// target column and normalizes contiguity whole-column."
//
// Every clause of the scenario gets its own leg: the slot lands relative to
// the MATCHING cards only; non-matching cards keep their relative order
// cell-for-cell; positions stay contiguous 0..n-1 in every column; slot 0
// into a no-match column places the card at the column's front. The hidden
// cards get byte-unchanged probes: every leg re-lists the whole board and
// pins the cells the move may not have touched.

// filterFixture builds an interleaved board:
//
//	todo        [g0(GRACE) p0, h1(—) p1, g2(GRACE) p2, h3(—) p3, g4(GRACE) p4]
//	in_progress [a0(ADA) p0, h1_(—) p1]
//	done        [d0(—) p0]
//
// — Grace's cards sit among hidden ones in todo, which is the scenario's
// "a column whose cards interleave those matching an assignee filter with
// those not matching it". Returns the listing and the cards by title.
func filterFixture(t *testing.T) (*Store, []ColumnCards, map[string]Card) {
	t.Helper()
	store := openStore(t)
	for _, text := range []string{"g0", "h1", "g2", "h3", "g4", "a0", "h1_", "d0"} {
		if _, err := store.Create(text); err != nil {
			t.Fatalf("Create(%q): %v", text, err)
		}
	}
	// Route a0/h1_ into in_progress and d0 into done through column moves
	// (placements, never edits), then assign.
	list := mustList(t, store)
	by := map[string]Card{}
	for _, col := range list {
		for _, c := range col.Cards {
			by[c.Title] = c
		}
	}
	if _, err := store.Move(by["a0"].ID, InProgress, 0); err != nil {
		t.Fatalf("move a0: %v", err)
	}
	if _, err := store.Move(by["h1_"].ID, InProgress, 1); err != nil {
		t.Fatalf("move h1_: %v", err)
	}
	if _, err := store.Move(by["d0"].ID, Done, 0); err != nil {
		t.Fatalf("move d0: %v", err)
	}
	for title, name := range map[string]string{"g0": "Grace", "g2": "Grace", "g4": "Grace", "a0": "Ada"} {
		id := by[title].ID
		if _, err := store.Change(id, nil, nil, AssignTo(name)); err != nil {
			t.Fatalf("assign %q: %v", title, err)
		}
	}
	list = mustList(t, store)
	by = map[string]Card{}
	for _, col := range list {
		for _, c := range col.Cards {
			by[c.Title] = c
		}
	}
	return store, list, by
}

// titlesByColumn flattens a listing to column → title sequence, the shape the
// relative-order pins read against.
func titlesByColumn(board []ColumnCards) map[Column][]string {
	out := map[Column][]string{}
	for _, col := range board {
		var titles []string
		for _, c := range col.Cards {
			titles = append(titles, c.Title)
		}
		out[col.Name] = titles
	}
	return out
}

// assertContiguous (store_move_test.go) pins "positions stay contiguous
// 0..n-1 in every column" on every fresh listing below.

// Card board/19, scenario body: the cross-column filtered move.
// Given the interleaved todo column
// When  an in_progress card is moved into todo at slot 1 among Grace's cards
// Then  it lands with exactly one Grace card before it
//
//	And the hidden todo cards keep their relative order cell-for-cell
//	And every column stays contiguous 0..n-1
//	And the moved card's returned position is the ABSOLUTE resolved index
func TestFilteredSlotMoveCrossColumn(t *testing.T) {
	store, before, by := filterFixture(t)
	moved := by["a0"] // Ada, in_progress position 0

	got, err := store.MoveFiltered(moved.ID, ptr(Todo), 1, "Grace")
	if err != nil {
		t.Fatalf("MoveFiltered(%d, Todo, 1, \"Grace\"): %v", moved.ID, err)
	}

	// Sequence [g0(G), h1(—), g2(G), h3(—), g4(G)]: matching at indices
	// [0,2,4]; slot 1 → the earliest position with one Grace card before
	// it → right after g0 → index 1. The position is the column's own
	// absolute index, not the slot: a slot 2 here would resolve to 3
	// (after g2), a slot 3+ clamps past the last match to index 5.
	if got.Column != Todo || got.Position != 1 {
		t.Fatalf("returned card = %q position %d, want todo position 1 (the resolved absolute index)", got.Column, got.Position)
	}
	if got.Assignee != "Ada" || got.Title != "a0" || got.ID != moved.ID {
		t.Errorf("returned card %+v, want a0/Ada with identity kept — a move places, it does not assign", got)
	}

	after := mustList(t, store)
	assertContiguous(t, after)
	want := titlesByColumn(after)
	if gotSeq := want[Todo]; !equalTitles(gotSeq, []string{"g0", "a0", "h1", "g2", "h3", "g4"}) {
		t.Errorf("todo sequence %v, want [g0 a0 h1 g2 h3 g4]", gotSeq)
	}
	// Hidden cards relative order: h1 still before h3 (cell-for-cell among
	// the non-matching); Grace's relative order g0 < g2 < g4 likewise kept.
	// in_progress closed its gap; done untouched.
	if gotSeq := want[InProgress]; !equalTitles(gotSeq, []string{"h1_"}) {
		t.Errorf("in_progress sequence %v, want [h1_] — the departed card's gap closed", gotSeq)
	}
	if gotSeq := want[Done]; !equalTitles(gotSeq, []string{"d0"}) {
		t.Errorf("done sequence %v, want [d0] — untouched column", gotSeq)
	}

	// The returned card agrees with the stored board cell-for-cell.
	for _, col := range after {
		for _, c := range col.Cards {
			if c.ID == moved.ID && c != got {
				t.Errorf("stored card %+v differs from returned %+v", c, got)
			}
		}
	}
	// Byte-unchanged probe on the hidden cells: everything except the moved
	// card's own cell and the in_progress gap close is exactly as it was.
	for _, beforeCol := range before {
		if beforeCol.Name == InProgress {
			continue // the source column legitimately changed (gap closed)
		}
		afterCol := after[columnIndex(t, after, beforeCol.Name)]
		if beforeCol.Name == Todo {
			// Same cells plus the moved one, same order among each group.
			var wasHid, nowHid, wasMatch, nowMatch []string
			for _, c := range beforeCol.Cards {
				if c.Assignee == "Grace" {
					wasMatch = append(wasMatch, c.Title)
				} else {
					wasHid = append(wasHid, c.Title)
				}
			}
			for _, c := range afterCol.Cards {
				if c.ID == moved.ID {
					continue
				}
				if c.Assignee == "Grace" {
					nowMatch = append(nowMatch, c.Title)
				} else {
					nowHid = append(nowHid, c.Title)
				}
			}
			if !equalTitles(wasHid, nowHid) || !equalTitles(wasMatch, nowMatch) {
				t.Errorf("todo groups changed order: hidden %v→%v, matching %v→%v — cell-for-cell kept",
					wasHid, nowHid, wasMatch, nowMatch)
			}
			continue
		}
		if !equalTitles(titlesByColumn([]ColumnCards{beforeCol})[beforeCol.Name],
			titlesByColumn([]ColumnCards{afterCol})[afterCol.Name]) {
			t.Errorf("column %q changed cell-for-cell: %v → %v", beforeCol.Name,
				beforeCol.Cards, afterCol.Cards)
		}
	}
}

// Card board/19: same-column filtered reorder — a card moving between slots
// of the column it already sits in, slot counted among matching cards.
func TestFilteredSlotMoveSameColumn(t *testing.T) {
	store, _, by := filterFixture(t)
	moved := by["g0"] // Grace at todo position 0

	got, err := store.MoveFiltered(moved.ID, ptr(Todo), 2, "Grace")
	if err != nil {
		t.Fatalf("MoveFiltered(%d, Todo, 2, \"Grace\"): %v", moved.ID, err)
	}
	// Removal first: remaining [h1(GONE-from-match: none), g2, h3, g4] with
	// matching [g2@1, g4@3] — slot 2 has no third matching card, so it
	// behaves like the last matching slot: right after g4 → absolute 4.
	if got.Position != 4 {
		t.Errorf("returned position %d, want 4 (slot at/past the matching count lands after the last match)", got.Position)
	}
	after := mustList(t, store)
	assertContiguous(t, after)
	if seq := titlesByColumn(after)[Todo]; !equalTitles(seq, []string{"h1", "g2", "h3", "g4", "g0"}) {
		t.Errorf("todo sequence %v, want [h1 g2 h3 g4 g0]", seq)
	}

	// A slot that does exist among the remaining matches: g0 back to Grace
	// slot 1 → the earliest spot after the preceding match g2 → absolute 2
	// (hidden h3 shifts down one cell, its order kept). NO stated column:
	// the nil direction re-slots within the card's current column.
	got, err = store.MoveFiltered(moved.ID, nil, 1, "Grace")
	if err != nil {
		t.Fatalf("second MoveFiltered: %v", err)
	}
	if got.Position != 2 {
		t.Errorf("returned position %d, want 2", got.Position)
	}
	after = mustList(t, store)
	assertContiguous(t, after)
	if seq := titlesByColumn(after)[Todo]; !equalTitles(seq, []string{"h1", "g2", "g0", "h3", "g4"}) {
		t.Errorf("todo sequence %v, want [h1 g2 g0 h3 g4]", seq)
	}
}

func equalTitles(a, b []string) bool {
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

// Card board/19: a HIDDEN (non-matching) card resolves its slot among the
// matching cards of the same column too — the slot never counts the card
// itself, it counts the matching cards that end up before it.
func TestFilteredSlotMoveHiddenCardSameColumn(t *testing.T) {
	store, _, by := filterFixture(t)
	moved := by["h3"] // unassigned, todo position 3

	got, err := store.MoveFiltered(moved.ID, ptr(Todo), 1, "Grace")
	if err != nil {
		t.Fatalf("MoveFiltered hidden card: %v", err)
	}
	// Remaining [g0(G)@0, h1(—)@1, g2(G)@2, g4(G)@3], matching indices
	// [0,2]; slot 1 → the earliest spot after g0 → index 1.
	if got.Position != 1 {
		t.Errorf("returned position %d, want 1", got.Position)
	}
	after := mustList(t, store)
	assertContiguous(t, after)
	if seq := titlesByColumn(after)[Todo]; !equalTitles(seq, []string{"g0", "h3", "h1", "g2", "g4"}) {
		t.Errorf("todo sequence %v, want [g0 h3 h1 g2 g4]", seq)
	}
}

// Card board/19, final clause: "slot 0 into a column holding no matching
// cards places the card at that column's front." The moved card is not
// either — in_progress holds Ada and an unassigned card, none matching
// Grace — and the whole done column is probed too.
func TestFilteredSlotMoveIntoNoMatchColumnGoesToTheFront(t *testing.T) {
	store, _, by := filterFixture(t)
	moved := by["g4"] // Grace, todo position 4

	got, err := store.MoveFiltered(moved.ID, ptr(InProgress), 0, "Grace")
	if err != nil {
		t.Fatalf("MoveFiltered into no-match column: %v", err)
	}
	if got.Column != InProgress || got.Position != 0 {
		t.Fatalf("returned card = %q position %d, want in_progress position 0 — the front", got.Column, got.Position)
	}
	after := mustList(t, store)
	assertContiguous(t, after)
	if seq := titlesByColumn(after)[InProgress]; !equalTitles(seq, []string{"g4", "a0", "h1_"}) {
		t.Errorf("in_progress sequence %v, want [g4 a0 h1_] — the hidden cards shifted down in order, nothing else", seq)
	}
	// The source column closed its gap; done byte-unchanged.
	if seq := titlesByColumn(after)[Todo]; !equalTitles(seq, []string{"g0", "h1", "g2", "h3"}) {
		t.Errorf("todo sequence %v, want [g0 h1 g2 h3]", seq)
	}
	if seq := titlesByColumn(after)[Done]; !equalTitles(seq, []string{"d0"}) {
		t.Errorf("done sequence %v, want [d0]", seq)
	}

	// The same front rule through the EMPTY end: slot 0 into the done column
	// (d0 is unassigned — no Grace there) front-places ahead of d0.
	got, err = store.MoveFiltered(moved.ID, ptr(Done), 0, "Grace")
	if err != nil {
		t.Fatalf("MoveFiltered into done at slot 0: %v", err)
	}
	if got.Position != 0 {
		t.Errorf("done position %d, want 0 — slot 0 into a no-match column is the column's front", got.Position)
	}
	after = mustList(t, store)
	assertContiguous(t, after)
	if seq := titlesByColumn(after)[Done]; !equalTitles(seq, []string{"g4", "d0"}) {
		t.Errorf("done sequence %v, want [g4 d0]", seq)
	}
}

// An unassigned card filtered-moved with the sentinel keyword: matching means
// IS NULL, the card counts itself removed first, and the hidden (here:
// assigned) cards keep their relative order. Also pins the keyword's other
// side — a name keyword never matches the unassigned cells.
func TestFilteredSlotMoveUnassignedSentinel(t *testing.T) {
	store, _, by := filterFixture(t)
	moved := by["h1_"] // unassigned, in_progress position 1

	// Into todo at slot 2 among the UNASSIGNED cards. Remaining todo
	// [g0(G), h1(—), g2(G), h3(—), g4(G)]; matching (NULL) at indices [1,3].
	// Slot 2 = at/past the matching count → right after h3 → index 4.
	got, err := store.MoveFiltered(moved.ID, ptr(Todo), 2, UnassignedKeyword)
	if err != nil {
		t.Fatalf("MoveFiltered sentinel: %v", err)
	}
	if got.Position != 4 {
		t.Errorf("returned position %d, want 4 (after the last unassigned card h3)", got.Position)
	}
	after := mustList(t, store)
	assertContiguous(t, after)
	if seq := titlesByColumn(after)[Todo]; !equalTitles(seq, []string{"g0", "h1", "g2", "h3", "h1_", "g4"}) {
		t.Errorf("todo sequence %v, want [g0 h1 g2 h3 h1_ g4]", seq)
	}
}

// The done freeze never fires on placement: a Done card filtered-moved out
// lands exactly like Move would — card board/16's unlock, through the
// filtered door.
func TestFilteredSlotMoveOutOfDoneIsNotAnEdit(t *testing.T) {
	store, _, by := filterFixture(t)
	moved := by["d0"] // unassigned, done position 0

	got, err := store.MoveFiltered(moved.ID, ptr(Todo), 0, "Grace")
	if err != nil {
		t.Fatalf("filtered move out of Done: %v — placement is never an edit", err)
	}
	if got.Column != Todo || got.Position != 0 {
		t.Errorf("returned card = %q position %d, want todo 0 (slot 0 before Grace's first card)", got.Column, got.Position)
	}
	if got.Assignee != "" {
		t.Errorf("moved card Assignee = %q, want unassigned kept", got.Assignee)
	}
	after := mustList(t, store)
	assertContiguous(t, after)
	if seq := titlesByColumn(after)[Todo]; !equalTitles(seq, []string{"d0", "g0", "h1", "g2", "h3", "g4"}) {
		t.Errorf("todo sequence %v, want [d0 g0 h1 g2 h3 g4]", seq)
	}
}

// Validation legs, board-style: every guard answers its typed error BEFORE
// any write, in Change's rank — keyword validity first (board/17's rank,
// here extended to the filter keyword), then the column enum, then the slot,
// then the existence read — and the board is unchanged cell-for-cell.
func TestFilteredSlotMoveValidation(t *testing.T) {
	store, before, by := filterFixture(t)
	target := by["g2"]

	legs := []struct {
		label   string
		id      int64
		column  *Column
		slot    int
		keyword string
		wantErr error
	}{
		{"unknown keyword outranks the column enum", target.ID, ptr(Column("backlog")), 0, "Zoidberg", ErrUnknownAssignee},
		{"unknown keyword outranks the slot guard", target.ID, ptr(Todo), -1, "grace", ErrUnknownAssignee},
		{"unknown keyword on a missing card — validity first", unknownID(t, store), nil, 0, "GRACE", ErrUnknownAssignee},
		{"empty keyword is not a name and not the sentinel", target.ID, ptr(Todo), 0, "", ErrUnknownAssignee},
		{"padded name is not a name", target.ID, ptr(Todo), 0, "Grace ", ErrUnknownAssignee},
		{"invalid column before the existence read", unknownID(t, store), ptr(Column("someday")), 0, "Grace", ErrInvalidColumn},
		{"invalid column outranks the slot guard", target.ID, ptr(Column("someday")), -1, "Grace", ErrInvalidColumn},
		{"negative slot before the existence read", unknownID(t, store), nil, -1, "Grace", ErrInvalidSlot},
		{"missing card with no stated column answers not-found", unknownID(t, store), nil, 0, "Grace", ErrCardNotFound},
	}
	for _, leg := range legs {
		got, err := store.MoveFiltered(leg.id, leg.column, leg.slot, leg.keyword)
		if !errors.Is(err, leg.wantErr) {
			t.Errorf("%s: err = %v, want %v", leg.label, err, leg.wantErr)
		}
		if got != (Card{}) {
			t.Errorf("%s: returned %+v, want the zero Card", leg.label, got)
		}
		if after := mustList(t, store); !boardEqual(before, after) {
			t.Errorf("%s: refused move changed the board:\n before: %s\n after:  %s",
				leg.label, flatten(before), flatten(after))
		}
	}
}

// The filtered read contract beside the move: matching cards only, stored
// order AND stored absolute positions unchanged (gaps and all), the fixed
// three-column shape kept, the sentinel keyword answering the unassigned
// cells, an unknown keyword answering before any query, and the read being
// read-only against the whole board.
func TestListFilteredContract(t *testing.T) {
	store, before, _ := filterFixture(t)

	t.Run("a roster name narrows every column to that user's cells", func(t *testing.T) {
		list, err := store.ListFiltered("Grace")
		if err != nil {
			t.Fatalf("ListFiltered(\"Grace\"): %v", err)
		}
		if len(list) != 3 || list[0].Name != Todo || list[1].Name != InProgress || list[2].Name != Done {
			t.Fatalf("columns = %v, want the fixed three in order", list)
		}
		// Stored positions pass through: g0@0, g2@2, g4@4 — GAPS included,
		// the hidden cells simply absent.
		if cards := list[0].Cards; len(cards) != 3 || cards[0].Position != 0 || cards[1].Position != 2 || cards[2].Position != 4 {
			t.Errorf("todo matched cards %+v, want positions 0,2,4 — stored positions unchanged", cards)
		}
		for _, col := range list {
			for _, c := range col.Cards {
				if c.Assignee != "Grace" {
					t.Errorf("column %q card %q assignee %q, want Grace — only matching cards", col.Name, c.Title, c.Assignee)
				}
				// Every listed card is cell-for-cell identical to its
				// counterpart in the unfiltered listing.
				for _, beforeCol := range before {
					if beforeCol.Name != col.Name {
						continue
					}
					for _, bc := range beforeCol.Cards {
						if bc.ID == c.ID && bc != c {
							t.Errorf("filtered cell %+v differs from stored cell %+v", c, bc)
						}
					}
				}
			}
		}
		if len(list[1].Cards) != 0 || len(list[2].Cards) != 0 {
			t.Errorf("non-todo columns hold %+v/%+v, want empty — a0 is Ada, d0 unassigned", list[1].Cards, list[2].Cards)
		}
	})

	t.Run("the sentinel answers the cards without an assignee", func(t *testing.T) {
		list, err := store.ListFiltered(UnassignedKeyword)
		if err != nil {
			t.Fatalf("ListFiltered(%q): %v", UnassignedKeyword, err)
		}
		wantSeq := map[Column][]string{Todo: {"h1", "h3"}, InProgress: {"h1_"}, Done: {"d0"}}
		for _, col := range list {
			if seq := titlesByColumn([]ColumnCards{col})[col.Name]; !equalTitles(seq, wantSeq[col.Name]) {
				t.Errorf("column %q sequence %v, want %v", col.Name, seq, wantSeq[col.Name])
			}
			for _, c := range col.Cards {
				if c.Assignee != "" {
					t.Errorf("unassigned listing carries %q assigned %q", c.Title, c.Assignee)
				}
			}
		}
	})

	t.Run("a name with no matches answers the empty three-column shape", func(t *testing.T) {
		list, err := store.ListFiltered("Linus")
		if err != nil {
			t.Fatalf("ListFiltered(\"Linus\"): %v", err)
		}
		if len(list) != 3 {
			t.Fatalf("columns = %d, want the fixed 3 even with no matches", len(list))
		}
		for _, col := range list {
			if len(col.Cards) != 0 {
				t.Errorf("column %q holds %d cards, want 0", col.Name, len(col.Cards))
			}
		}
	})

	t.Run("an unknown keyword answers before any query", func(t *testing.T) {
		for _, keyword := range []string{"grace", "Zoidberg", "", "Unassigned", "UNASSIGNED", "Ada ", "todo"} {
			list, err := store.ListFiltered(keyword)
			if !errors.Is(err, ErrUnknownAssignee) {
				t.Errorf("ListFiltered(%q): err = %v, want ErrUnknownAssignee", keyword, err)
			}
			if list != nil {
				t.Errorf("ListFiltered(%q) returned %+v beside the error, want nil", keyword, list)
			}
		}
		// Read-only contract: nothing above changed the board.
		if after := mustList(t, store); !boardEqual(before, after) {
			t.Errorf("filtered reads moved the board:\n before: %s\n after:  %s", flatten(before), flatten(after))
		}
	})
}
