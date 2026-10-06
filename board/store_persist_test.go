package board

import (
	"path/filepath"
	"testing"
)

// openStoreAt opens the store at a test-owned path — the board/13 machinery
// for tests whose subject is the file itself: close it and open it again at
// the same path. openStore hides the path inside its own temp dir, which a
// reopen test cannot reach.
func openStoreAt(t *testing.T, path string) *Store {
	t.Helper()
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// placement is one card's expected resting spot — title, column, position —
// the shape a close-and-reopen fixture pins: the same cards with the same
// texts, columns, and positions, stated before the close and re-pinned after
// the reopen.
type placement struct {
	title    string
	column   Column
	position int
}

// assertPlacements pins a listing against the fixture's placements: exactly
// this many cards on the board in total, and each named title sitting at
// exactly its column and position. Contiguous positions make a column's
// index and its position the same number, so the check reads the listing
// top-to-bottom.
func assertPlacements(t *testing.T, board []ColumnCards, want []placement) {
	t.Helper()
	total := 0
	for _, col := range board {
		total += len(col.Cards)
	}
	if total != len(want) {
		t.Fatalf("board holds %d cards, want %d — %s", total, len(want), flatten(board))
	}
	for _, p := range want {
		col := board[columnIndex(t, board, p.column)]
		if len(col.Cards) <= p.position || col.Cards[p.position].Title != p.title {
			t.Errorf("card %q not at %q position %d — column %q holds %s",
				p.title, p.column, p.position, col.Name, titles(col.Cards))
		}
	}
}

// Card board/13 — State survives store close and reopen.
// Given a board holding cards in a mix of columns and positions
// When  the store is closed and opened again at the same path
// Then  the board lists the same cards with the same texts, columns, and positions
//
// Every fixture board is built through the public operations only — Create
// (the todo bottom-append), then Move (cross-column placement and same-column
// reorder, clamped indexes included), then Delete — so the seeded state is
// exactly what a real caller leaves behind: cards resting in a mix of columns
// at positions no insertion order produced. The listing taken before Close is
// re-taken after opening the same path again and compared cell-for-cell:
// Card is comparable, so one equality per cell pins title, column, position,
// and identifier together — the reopened board answers with the same cards,
// not merely the same texts. The placements are pinned on both sides too, so
// the Given's mix is provably non-trivial rather than an accident the
// comparison could pass vacuously. An empty board rows the table as well: a
// reopened file must still list the three fixed columns with nothing in them.
func TestStateSurvivesCloseAndReopen(t *testing.T) {
	type moveTarget struct {
		title  string
		column Column
		index  int
	}
	tests := []struct {
		name    string
		creates []string
		moves   []moveTarget
		deletes []string
		want    []placement
	}{
		{
			"scenario: mix of columns and positions — reorder into in_progress, done placement, untouched todo",
			[]string{"A", "B", "C", "X", "Y"},
			[]moveTarget{
				{"X", InProgress, 0}, // X leaves todo first, lands on top of in_progress
				{"Y", InProgress, 0}, // then on top of X — insertion order and position order disagree
				{"C", Done, 1},       // done is empty, so the index clamps to its only slot
				{"B", InProgress, 1}, // into the middle of [Y, X]
			},
			nil,
			[]placement{
				{"A", Todo, 0},
				{"Y", InProgress, 0},
				{"B", InProgress, 1},
				{"X", InProgress, 2},
				{"C", Done, 0},
			},
		},
		{
			"same-column reorders — the column survives in an order Create never wrote",
			[]string{"A", "B", "C", "D"},
			[]moveTarget{
				{"A", Todo, 2}, // [B, C, A, D]
				{"D", Todo, 0}, // [D, B, C, A]
				{"B", Todo, 2}, // [D, C, B, A]
			},
			nil,
			[]placement{
				{"D", Todo, 0},
				{"C", Todo, 1},
				{"B", Todo, 2},
				{"A", Todo, 3},
			},
		},
		{
			"a delete closed its gap before the close — the reopened board shows no gap",
			[]string{"A", "B", "C"},
			[]moveTarget{{"A", Done, 0}},
			[]string{"B"},
			[]placement{
				{"C", Todo, 0},
				{"A", Done, 0},
			},
		},
		{
			"done populated at every position",
			[]string{"S1", "S2", "S3", "S4"},
			[]moveTarget{
				{"S1", Done, 0}, // [S1]
				{"S3", Done, 1}, // [S1, S3]
				{"S2", Done, 0}, // [S2, S1, S3]
				{"S4", Done, 1}, // [S2, S4, S1, S3]
			},
			nil,
			[]placement{
				{"S2", Done, 0},
				{"S4", Done, 1},
				{"S1", Done, 2},
				{"S3", Done, 3},
			},
		},
		{
			"an empty board reopens as an empty valid board",
			nil,
			nil,
			nil,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "board.db")
			store := openStoreAt(t, path)

			created := make(map[string]int64, len(tt.creates))
			for _, title := range tt.creates {
				card, err := store.Create(title)
				if err != nil {
					t.Fatalf("Create(%q): %v", title, err)
				}
				created[title] = card.ID
			}
			for _, mv := range tt.moves {
				id, ok := created[mv.title]
				if !ok {
					t.Fatalf("move target %q was never created", mv.title)
				}
				if _, err := store.Move(id, mv.column, mv.index); err != nil {
					t.Fatalf("Move(%q, %q, %d): %v", mv.title, mv.column, mv.index, err)
				}
			}
			for _, title := range tt.deletes {
				id, ok := created[title]
				if !ok {
					t.Fatalf("delete target %q was never created", title)
				}
				if err := store.Delete(id); err != nil {
					t.Fatalf("Delete(%q): %v", title, err)
				}
			}

			before := mustList(t, store)
			assertPlacements(t, before, tt.want)

			if err := store.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}

			again := openStoreAt(t, path)
			after := mustList(t, again)
			if !boardEqual(before, after) {
				t.Errorf("reopened board differs from the closed one:\n before: %s\n after:  %s",
					flatten(before), flatten(after))
			}
			assertPlacements(t, after, tt.want)
		})
	}
}
