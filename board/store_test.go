package board

import (
	"os"
	"path/filepath"
	"testing"
)

// Card board/01 — Fresh store opens as an empty valid board.
// Given no data file exists at the store's path
// When  the store is opened
// Then  the board lists three columns in the order todo, in_progress, done
//
//	And every column holds no cards
func TestOpenFreshStoreIsEmptyValidBoard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "board.db")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("data file must not exist before Open, stat err = %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open on absent path failed: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Open did not create the data file: %v", err)
	}

	board := mustList(t, store)

	want := []Column{Todo, InProgress, Done}
	if len(board) != len(want) {
		t.Fatalf("List returned %d columns, want %d (todo, in_progress, done)", len(board), len(want))
	}
	for i, wantCol := range want {
		if board[i].Name != wantCol {
			t.Errorf("column %d = %q, want %q", i, board[i].Name, wantCol)
		}
		if len(board[i].Cards) != 0 {
			t.Errorf("fresh column %q holds %d cards, want 0", board[i].Name, len(board[i].Cards))
		}
	}
}

func mustList(t *testing.T, store *Store) []ColumnCards {
	t.Helper()
	board, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return board
}
