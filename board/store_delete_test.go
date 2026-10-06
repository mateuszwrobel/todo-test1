package board

import (
	"testing"
)

// Card board/11 — Delete removes and closes the gap.
// Given a column holding cards [A, B, C] with positions 0, 1, 2
// When  card B is deleted
// Then  the column holds [A, C] with positions 0, 1
//
//	And a later created card receives a fresh identifier — identifier B is never reused
//
// The scenario pins the middle-of-three witness; the table checks the general
// contract — any column, any position, including a column's only card. Each
// row pins three observables: the source column lost exactly the target with
// survivor order kept and positions contiguous 0..n-1, the deleted card is
// gone from every column, and every column the delete never touched is
// cell-for-cell what it was.
func TestDeleteRemovesAndClosesTheGap(t *testing.T) {
	tests := []struct {
		name    string
		columns []columnFixture
		from    Column // column the card is deleted from
		atPos   int    // the deleted card's position within it
	}{
		{
			"scenario: middle of three in todo",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{InProgress, []string{"X", "Y"}},
				{Done, []string{"d0"}},
			},
			Todo, 1,
		},
		{
			"top of three",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
				{Done, []string{"d0"}},
			},
			Todo, 0,
		},
		{
			"bottom of three",
			[]columnFixture{
				{Todo, []string{"A", "B", "C"}},
			},
			Todo, 2,
		},
		{
			"the only card in its column empties it",
			[]columnFixture{
				{Todo, []string{"solo"}},
				{InProgress, []string{"p0", "p1"}},
			},
			Todo, 0,
		},
		{
			"middle of three in in_progress works the same",
			[]columnFixture{
				{Todo, []string{"t0"}},
				{InProgress, []string{"A", "B", "C"}},
			},
			InProgress, 1,
		},
		{
			"middle of three in done works the same",
			[]columnFixture{
				{Todo, []string{"t0", "t1"}},
				{Done, []string{"A", "B", "C"}},
			},
			Done, 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, tt.columns)
			before := mustList(t, store)
			target := before[columnIndex(t, before, tt.from)].Cards[tt.atPos]

			if err := store.Delete(target.ID); err != nil {
				t.Fatalf("Delete(%d): %v", target.ID, err)
			}

			after := mustList(t, store)

			// Source column: the target is gone, survivors keep their order,
			// positions contiguous from 0 — the gap closed.
			var want []Card
			for _, c := range before[columnIndex(t, before, tt.from)].Cards {
				if c.ID != target.ID {
					want = append(want, c)
				}
			}
			source := after[columnIndex(t, after, tt.from)].Cards
			if len(source) != len(want) {
				t.Fatalf("column %q holds %s after the delete, want %d cards",
					tt.from, titles(source), len(want))
			}
			for i, w := range want {
				if source[i].ID != w.ID {
					t.Errorf("column %q index %d holds card %d, want %d — survivors keep their order",
						tt.from, i, source[i].ID, w.ID)
				}
				if source[i].Position != i {
					t.Errorf("column %q index %d reports position %d, want %d (contiguous 0..n-1)",
						tt.from, i, source[i].Position, i)
				}
			}

			// The deleted card is gone from the whole board, not just its column.
			for _, col := range after {
				for _, c := range col.Cards {
					if c.ID == target.ID {
						t.Fatalf("card %d still listed in column %q", target.ID, col.Name)
					}
				}
			}

			// Untouched columns are cell-for-cell what they were — a delete
			// never re-positions another column's cards.
			for i, col := range after {
				if col.Name == tt.from {
					continue
				}
				if !boardEqual(before[i:i+1], after[i:i+1]) {
					t.Errorf("column %q changed by a delete from %q:\n before: %s\n after:  %s",
						col.Name, tt.from, flatten(before[i:i+1]), flatten(after[i:i+1]))
				}
			}
		})
	}
}

// The scenario's second Then, pinned directly: an identifier a deleted card
// held is never re-issued. Create → Delete → Create: every later card carries
// a fresh identifier strictly greater than the deleted card's — the table's
// autoincrement only counts forward, so identifier B can never come back to
// address a different card, and the count keeps moving after each accepted
// create.
func TestDeleteNeverReusesIdentifier(t *testing.T) {
	store := openStore(t)
	kept, err := store.Create("A")
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	deleted, err := store.Create("B")
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}
	if err := store.Delete(deleted.ID); err != nil {
		t.Fatalf("Delete(%d): %v", deleted.ID, err)
	}

	fresh, err := store.Create("fresh after delete")
	if err != nil {
		t.Fatalf("Create after delete: %v", err)
	}
	if fresh.ID <= deleted.ID {
		t.Errorf("identifier after deleting card %d = %d, want strictly greater — deleted identifiers are never re-issued",
			deleted.ID, fresh.ID)
	}

	// The monotonic track continues for every later create, not just the next.
	later, err := store.Create("later card")
	if err != nil {
		t.Fatalf("Create later card: %v", err)
	}
	if later.ID <= fresh.ID || later.ID <= deleted.ID {
		t.Errorf("later identifier = %d, want greater than %d and %d — identifiers keep counting forward",
			later.ID, fresh.ID, deleted.ID)
	}
	if board := mustList(t, store); board[0].Cards[0].ID != kept.ID {
		t.Errorf("survivor of the delete = %s, want card %d still at the top",
			titles(board[0].Cards), kept.ID)
	}
}
