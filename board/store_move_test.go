package board

import (
	"errors"
	"testing"
)

// Card board/06 — Move changes column and keeps neighbors' order.
// Given a todo column holding cards [A, B, C] and an in_progress column holding [X, Y]
// When  card B is moved to in_progress at position 1
// Then  in_progress holds [X, B, Y] with positions 0, 1, 2
//
//	And todo holds [A, C] with positions 0, 1
//	And B's text and identifier are unchanged
//
// The scenario pins the first row; the table checks the general contract —
// any source column, any target, any index — with the Gherkin's exact
// placement spelled out as literal title sequences: the moved card lands at
// exactly the target index, the target's neighbors keep their relative order
// around it (the ones above stay above, the ones below stay below), and the
// source column closes its gap in the survivors' order. Identifiers ride
// along pinned: every cell's id must equal the id the title carried before
// the move, so B — and X, Y, A, C — are provably the same cards. done-in and
// done-out rows pin that column membership is the only difference: moving
// out of, or into, done runs the same placement mechanics as any column.
func TestMoveCrossColumnKeepsNeighborsOrder(t *testing.T) {
	tests := []struct {
		name    string
		columns []columnFixture
		from    Column
		fromPos int
		to      Column
		index   int
		wantTo  []string // target column top-to-bottom after the move
		wantSrc []string // source column top-to-bottom after the gap close
	}{
		{
			"scenario: todo middle to in_progress at position 1",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{InProgress, []string{"X", "Y"}},
			},
			Todo, 1, InProgress, 1,
			[]string{"X", "B", "Y"},
			[]string{"A", "C"},
		},
		{
			"to index 0 of a populated column",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{InProgress, []string{"X", "Y"}},
			},
			Todo, 1, InProgress, 0,
			[]string{"B", "X", "Y"},
			[]string{"A", "C"},
		},
		{
			"to the end index of a populated column",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{InProgress, []string{"X", "Y"}},
			},
			Todo, 1, InProgress, 2,
			[]string{"X", "Y", "B"},
			[]string{"A", "C"},
		},
		{
			"into the middle of done — done-in takes index placement like any column",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{Done, []string{"D0", "D1"}},
			},
			Todo, 0, Done, 1,
			[]string{"D0", "A", "D1"},
			[]string{"B", "C"},
		},
		{
			"out of done into todo — done-out closes its gap like any column",
			[]columnFixture{
				{Todo, []string{"T0", "T1"}},
				{Done, []string{"D0", "D1"}},
			},
			Done, 1, Todo, 1,
			[]string{"T0", "D1", "T1"},
			[]string{"D0"},
		},
		{
			"into an empty column at index 0",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
			},
			Todo, 2, Done, 0,
			[]string{"C"},
			[]string{"A", "B"},
		},
		{
			"from a single-card column, which ends up empty",
			[]columnFixture{
				{Todo, []string{"Z"}},
				{InProgress, []string{"X", "Y"}},
			},
			Todo, 0, InProgress, 1,
			[]string{"X", "Z", "Y"},
			nil,
		},
		{
			"done middle to in_progress middle",
			[]columnFixture{
				{InProgress, []string{"P0", "P1"}},
				{Done, []string{"D0", "D1"}},
			},
			Done, 0, InProgress, 1,
			[]string{"P0", "D0", "P1"},
			[]string{"D1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, tt.columns)

			before := mustList(t, store)
			byTitle := cardsByTitle(before)
			target := before[columnIndex(t, before, tt.from)].Cards[tt.fromPos]

			moved, err := store.Move(target.ID, tt.to, tt.index)
			if err != nil {
				t.Fatalf("Move(%d, %q, %d): %v", target.ID, tt.to, tt.index, err)
			}
			if moved.ID != target.ID || moved.Title != target.Title {
				t.Errorf("moved card = %+v, want identity and text of %+v kept", moved, target)
			}
			if moved.Column != tt.to {
				t.Errorf("moved Column = %q, want %q", moved.Column, tt.to)
			}
			if moved.Position != tt.index {
				t.Errorf("moved Position = %d, want %d (exact target index)", moved.Position, tt.index)
			}

			after := mustList(t, store)
			assertColumnCells(t, after, tt.to, tt.wantTo, byTitle)
			assertColumnCells(t, after, tt.from, tt.wantSrc, byTitle)

			// Every column neither left nor entered is cell-for-cell untouched.
			for _, col := range before {
				if col.Name == tt.from || col.Name == tt.to {
					continue
				}
				kept := after[columnIndex(t, after, col.Name)]
				if len(kept.Cards) != len(col.Cards) {
					t.Fatalf("untouched column %q holds %s, want %s",
						col.Name, titles(kept.Cards), titles(col.Cards))
				}
				for i := range col.Cards {
					if kept.Cards[i] != col.Cards[i] {
						t.Errorf("untouched column %q cell %d = %+v, want %+v",
							col.Name, i, kept.Cards[i], col.Cards[i])
					}
				}
			}
			assertContiguous(t, after)
		})
	}
}

// The cards are silent on out-of-range indexes, so Move clamps into the valid
// insertion range: a negative index lands at the top, an index past the end
// lands at the bottom — and the bottom clamp reproduces exactly what Change's
// column direction appends to (the KW3 bottom-append path, now expressible
// through both operations with the same end state).
func TestMoveClampsOutOfRangePositions(t *testing.T) {
	tests := []struct {
		name    string
		columns []columnFixture
		from    Column
		fromPos int
		to      Column
		index   int
		wantTo  []string
		wantSrc []string
	}{
		{
			"negative index clamps to the top",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{InProgress, []string{"X", "Y"}},
			},
			Todo, 1, InProgress, -7,
			[]string{"B", "X", "Y"},
			[]string{"A", "C"},
		},
		{
			"index far past the end clamps to the bottom",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{InProgress, []string{"X", "Y"}},
			},
			Todo, 1, InProgress, 99,
			[]string{"X", "Y", "B"},
			[]string{"A", "C"},
		},
		{
			"any index into an empty column clamps to its only slot",
			[]columnFixture{
				{Todo, []string{"A"}},
			},
			Todo, 0, Done, 5,
			[]string{"A"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, tt.columns)

			before := mustList(t, store)
			byTitle := cardsByTitle(before)
			target := before[columnIndex(t, before, tt.from)].Cards[tt.fromPos]

			moved, err := store.Move(target.ID, tt.to, tt.index)
			if err != nil {
				t.Fatalf("Move(%d, %q, %d): %v", target.ID, tt.to, tt.index, err)
			}
			wantIndex := tt.index
			if wantIndex < 0 {
				wantIndex = 0
			}
			if n := len(before[columnIndex(t, before, tt.to)].Cards); wantIndex > n {
				wantIndex = n
			}
			if moved.Position != wantIndex {
				t.Errorf("moved Position = %d, want clamped %d", moved.Position, wantIndex)
			}
			after := mustList(t, store)
			assertColumnCells(t, after, tt.to, tt.wantTo, byTitle)
			assertColumnCells(t, after, tt.from, tt.wantSrc, byTitle)
			assertContiguous(t, after)
		})
	}
}

// The bottom clamp and the KW3 bottom-append are the same placement: the same
// seeded board, the same card, Move-past-the-end versus Change's column
// direction, end cell-for-cell identical.
func TestMovePastEndEqualsChangeBottomAppend(t *testing.T) {
	seed := []columnFixture{
		{Todo, []string{"A", "B", "C"}},
		{InProgress, []string{"X", "Y"}},
	}
	viaMove := openStore(t)
	seedBoard(t, viaMove, seed)
	target := mustList(t, viaMove)[columnIndex(t, mustList(t, viaMove), Todo)].Cards[1]
	if _, err := viaMove.Move(target.ID, InProgress, 99); err != nil {
		t.Fatalf("Move(%d, in_progress, 99): %v", target.ID, err)
	}

	viaChange := openStore(t)
	seedBoard(t, viaChange, seed)
	target2 := mustList(t, viaChange)[columnIndex(t, mustList(t, viaChange), Todo)].Cards[1]
	if _, err := viaChange.Change(target2.ID, nil, ptr(InProgress)); err != nil {
		t.Fatalf("Change(%d, nil, in_progress): %v", target2.ID, err)
	}

	if before, after := mustList(t, viaMove), mustList(t, viaChange); !boardEqual(before, after) {
		t.Errorf("Move-past-end and Change differ:\n move:  %s\n change: %s",
			flatten(before), flatten(after))
	}
}

// Rejected moves — identifiers no card holds, columns outside the enum —
// decide before anything is written: the named outcome alongside the zero
// Card, the board cell-for-cell as it was, and (moves being pure placement)
// no identifier consumed anywhere in the transaction. Unknown id and invalid
// column together report ErrInvalidColumn: the enum guard runs before the
// transaction opens, so it is what the call answers, matching Change.
func TestMoveRejectionsChangeNothing(t *testing.T) {
	tests := []struct {
		name    string
		unknown bool
		col     Column
		want    error
	}{
		{"unknown id, valid target", true, InProgress, ErrCardNotFound},
		{"unknown id, same-column move to its own column", true, Todo, ErrCardNotFound},
		{"unknown id past the highest", true, Done, ErrCardNotFound},
		{"valid id, empty column value", false, Column(""), ErrInvalidColumn},
		{"valid id, wrong case", false, Column("Todo"), ErrInvalidColumn},
		{"valid id, hyphen instead of underscore", false, Column("in-progress"), ErrInvalidColumn},
		{"valid id, arbitrary junk", false, Column("backlog"), ErrInvalidColumn},
		{"unknown id and invalid column — the enum guard answers first", true, Column("backlog"), ErrInvalidColumn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, []columnFixture{
				{Todo, []string{"t0", "t1", "t2"}},
				{InProgress, []string{"p0"}},
			})
			before := mustList(t, store)
			lastCreated, err := store.Create("last")
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			before = mustList(t, store)

			id := before[columnIndex(t, before, Todo)].Cards[0].ID
			if tt.unknown {
				id = unknownID(t, store)
			}

			got, err := store.Move(id, tt.col, 1)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Move(%d, %q, 1) err = %v, want %v", id, tt.col, err, tt.want)
			}
			if got != (Card{}) {
				t.Errorf("Move returned %+v on rejection, want the zero Card", got)
			}
			after := mustList(t, store)
			if !boardEqual(before, after) {
				t.Errorf("rejected move changed the board:\n before: %s\n after:  %s",
					flatten(before), flatten(after))
			}

			next, err := store.Create("next card")
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if next.ID != lastCreated.ID+1 {
				t.Errorf("identifier after rejected move = %d, want %d — a rejected move must not consume ids",
					next.ID, lastCreated.ID+1)
			}
		})
	}
}

// Card board/07 — Reorder within a column.
// Given a column holding cards [A, B, C] at positions 0, 1, 2
// When  card A is moved to position 2 of the same column
// Then  the column holds [B, C, A] with positions 0, 1, 2
//
// Same-column Move is remove-then-insert: position indexes the column minus
// the moved card, which is what turns A→2 into [B, C, A] — not an insertion
// past a still-counted A. The table covers the scenario, the same move from
// below (down one, up to top), the exact-position no-op the splice makes
// provably inert (board cell-for-cell identical, card returned unchanged), a
// column's only card, and the clamp ends inside one column. The card keeps
// its identifier, its text, and its column — only position is re-decided.
// This is the behavior the KW3 same-column no-op disclaimed; the pin there
// survives as a statement about Change, and this test is where the store
// makes the claim.
func TestMoveReordersWithinColumn(t *testing.T) {
	tests := []struct {
		name    string
		columns []columnFixture
		from    Column
		fromPos int
		index   int
		want    []string // the reordered column, top-to-bottom
	}{
		{
			"scenario: top card to position 2 — A over B and C",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
			},
			Todo, 0, 2,
			[]string{"B", "C", "A"},
		},
		{
			"down one: middle card to the bottom",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
			},
			Todo, 1, 2,
			[]string{"A", "C", "B"},
		},
		{
			"up to top: bottom card to position 0",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
			},
			Todo, 2, 0,
			[]string{"C", "A", "B"},
		},
		{
			"exact-position no-op: middle card to its own index",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
			},
			Todo, 1, 1,
			[]string{"A", "B", "C"},
		},
		{
			"exact-position no-op: top card to index 0",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{Done, []string{"D0", "D1"}},
			},
			Todo, 0, 0,
			[]string{"A", "B", "C"},
		},
		{
			"a column's only card to position 0 stays put",
			[]columnFixture{
				{InProgress, []string{"Z"}},
			},
			InProgress, 0, 0,
			[]string{"Z"},
		},
		{
			"negative index clamps to the top",
			[]columnFixture{
				{Done, []string{"D0", "D1", "D2"}},
			},
			Done, 2, -3,
			[]string{"D2", "D0", "D1"},
		},
		{
			"past-end index clamps to the bottom",
			[]columnFixture{
				{Done, []string{"D0", "D1", "D2"}},
			},
			Done, 0, 99,
			[]string{"D1", "D2", "D0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, tt.columns)

			before := mustList(t, store)
			byTitle := cardsByTitle(before)
			target := before[columnIndex(t, before, tt.from)].Cards[tt.fromPos]

			moved, err := store.Move(target.ID, tt.from, tt.index)
			if err != nil {
				t.Fatalf("Move(%d, %q, %d): %v", target.ID, tt.from, tt.index, err)
			}
			if moved.ID != target.ID || moved.Title != target.Title || moved.Column != tt.from {
				t.Errorf("moved card = %+v, want %+v with identity, text, and column kept", moved, target)
			}
			wantIndex := tt.index
			if wantIndex < 0 {
				wantIndex = 0
			}
			if n := len(before[columnIndex(t, before, tt.from)].Cards) - 1; wantIndex > n {
				wantIndex = n
			}
			if moved.Position != wantIndex {
				t.Errorf("moved Position = %d, want %d", moved.Position, wantIndex)
			}

			after := mustList(t, store)
			assertColumnCells(t, after, tt.from, tt.want, byTitle)

			// Only the reordered column may differ — every other column is
			// cell-for-cell exactly as it was.
			for _, col := range before {
				if col.Name == tt.from {
					continue
				}
				kept := after[columnIndex(t, after, col.Name)]
				if len(kept.Cards) != len(col.Cards) {
					t.Fatalf("untouched column %q holds %s, want %s",
						col.Name, titles(kept.Cards), titles(col.Cards))
				}
				for i := range col.Cards {
					if kept.Cards[i] != col.Cards[i] {
						t.Errorf("untouched column %q cell %d = %+v, want %+v — a reorder touches one column",
							col.Name, i, kept.Cards[i], col.Cards[i])
					}
				}
			}
			// The no-op rows are pinned harder than by order alone: the whole
			// board must be identical, down to positions the splice rewrote
			// with the same values.
			if tt.fromPos == wantIndex && !boardEqual(before, after) {
				t.Errorf("exact-position move changed the board:\n before: %s\n after:  %s",
					flatten(before), flatten(after))
			}
			assertContiguous(t, after)
		})
	}
}

// cardsByTitle indexes a listing by title — every fixture title in this file
// is unique across the board — so each expected cell can pin its identifier
// as well as its text and position.
func cardsByTitle(board []ColumnCards) map[string]Card {
	byTitle := make(map[string]Card)
	for _, col := range board {
		for _, c := range col.Cards {
			byTitle[c.Title] = c
		}
	}
	return byTitle
}

// assertColumnCells pins one column's exact top-to-bottom state after a move:
// each cell carries the named card's identifier (identity, not a copy), the
// listed title, and a contiguous position. An empty want means the column
// must hold no cards at all.
func assertColumnCells(t *testing.T, board []ColumnCards, name Column, want []string, byTitle map[string]Card) {
	t.Helper()
	col := board[columnIndex(t, board, name)]
	if len(col.Cards) != len(want) {
		t.Fatalf("column %q holds %s, want %v", name, titles(col.Cards), want)
	}
	for i, title := range want {
		cell := col.Cards[i]
		if cell.Title != title || cell.Position != i || cell.ID != byTitle[title].ID {
			t.Errorf("column %q cell %d = %+v, want card %q (%+v)",
				name, i, cell, title, byTitle[title])
		}
	}
}

// assertContiguous pins the module invariant on a listing: every column's
// positions are exactly 0..n-1, top-to-bottom.
func assertContiguous(t *testing.T, board []ColumnCards) {
	t.Helper()
	for _, col := range board {
		for pos, c := range col.Cards {
			if c.Position != pos {
				t.Errorf("column %q position gap: card at index %d reports position %d — %s",
					col.Name, pos, c.Position, titles(col.Cards))
			}
		}
	}
}
