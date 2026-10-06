package board

import (
	"fmt"
	"strings"
	"testing"
)

// columnFixture describes one column of a test board: its name and its card
// texts top-to-bottom — position is the index.
type columnFixture struct {
	name   Column
	titles []string
}

// Card board/05 — List is fixed columns in position order.
// Given a board holding cards spread across the three columns with positions
// 0..n-1 in each
// When  the board is listed
// Then  the columns arrive in the order todo, in_progress, done
//
//	And each column's cards arrive top-to-bottom by position
func TestListIsFixedColumnsInPositionOrder(t *testing.T) {
	tests := []struct {
		name    string
		columns []columnFixture
	}{
		{
			"spread across all three columns",
			[]columnFixture{
				{Todo, []string{"t0", "t1", "t2"}},
				{InProgress, []string{"p0", "p1"}},
				{Done, []string{"d0", "d1", "d2", "d3"}},
			},
		},
		{
			"middle column empty, fixed order stands",
			[]columnFixture{
				{Todo, []string{"only"}},
				{InProgress, nil},
				{Done, []string{"finished"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := openStore(t)
			seedBoard(t, store, tt.columns)

			board := mustList(t, store)

			if len(board) != len(columns) {
				t.Fatalf("List returned %d columns, want the fixed %d", len(board), len(columns))
			}
			for i, want := range columns {
				if board[i].Name != want {
					t.Fatalf("column %d = %q, want %q (fixed order todo, in_progress, done)",
						i, board[i].Name, want)
				}
			}
			for _, fixture := range tt.columns {
				got := board[columnIndex(t, board, fixture.name)].Cards
				if len(got) != len(fixture.titles) {
					t.Fatalf("column %q holds %d cards, want %d",
						fixture.name, len(got), len(fixture.titles))
				}
				for pos, wantTitle := range fixture.titles {
					c := got[pos]
					if c.Position != pos {
						t.Errorf("column %q index %d has position %d, want %d (top-to-bottom by position)",
							fixture.name, pos, c.Position, pos)
					}
					if c.Title != wantTitle {
						t.Errorf("column %q index %d = %q, want %q",
							fixture.name, pos, c.Title, wantTitle)
					}
					if c.Column != fixture.name {
						t.Errorf("card %q reports column %q, want %q", c.Title, c.Column, fixture.name)
					}
				}
			}
		})
	}
}

func columnIndex(t *testing.T, board []ColumnCards, name Column) int {
	t.Helper()
	for i, col := range board {
		if col.Name == name {
			return i
		}
	}
	t.Fatalf("List is missing column %q", name)
	return -1
}

// seedBoard places a fixture board. The todo column is filled through Create —
// the real append primitive. The in_progress and done fixtures are placed by
// direct insert: the Given's state is a column holding specific cards at
// specific positions, and an open store's contract (Create, Move) could reach
// that state only by way of moves a test may itself want to observe, so the
// insert writes exactly the Given — column membership with contiguous
// positions 0..n-1 — through the same schema and constraints Create uses.
func seedBoard(t *testing.T, store *Store, fixtures []columnFixture) {
	t.Helper()
	for _, fixture := range fixtures {
		for pos, title := range fixture.titles {
			if fixture.name == Todo {
				if _, err := store.Create(title); err != nil {
					t.Fatalf("seed Create(%q): %v", title, err)
				}
				continue
			}
			placeCard(t, store, fixture.name, title, pos)
		}
	}
}

// placeCard inserts one card directly at an explicit position of a column.
func placeCard(t *testing.T, store *Store, col Column, title string, position int) {
	t.Helper()
	res, err := store.db.Exec(
		`INSERT INTO cards (title, "column", position) VALUES (?, ?, ?)`,
		title, string(col), position,
	)
	if err != nil {
		t.Fatalf("place(%q, %q, %d): %v", col, title, position, err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("place(%q, %q, %d): rows affected %d, err %v", col, title, position, n, err)
	}
}

// The ordered read is the index-backed one: a column's cards come back by
// position, never by insertion or identifier order. The fixture inserts the
// bottom card first, so identifier order and position order disagree and the
// listing can only satisfy position order.
func TestListOrdersByPositionNotInsertion(t *testing.T) {
	store := openStore(t)
	placeCard(t, store, InProgress, "bottom-by-position", 1) // inserted first, sits second
	placeCard(t, store, InProgress, "top-by-position", 0)    // inserted later,  sits first

	board := mustList(t, store)
	inProgress := board[columnIndex(t, board, InProgress)]
	if len(inProgress.Cards) != 2 {
		t.Fatalf("in_progress holds %d cards, want 2", len(inProgress.Cards))
	}
	if inProgress.Cards[0].Title != "top-by-position" || inProgress.Cards[1].Title != "bottom-by-position" {
		t.Errorf("in_progress order = %s, want position order top, bottom",
			titles(inProgress.Cards))
	}
}

func titles(cards []Card) string {
	parts := make([]string, len(cards))
	for i, c := range cards {
		parts[i] = fmt.Sprintf("%q@%d", c.Title, c.Position)
	}
	return strings.Join(parts, ", ")
}
