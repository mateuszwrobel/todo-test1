package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Card server/03 — Start without todo data creates an empty board.
// Given no todo data file exists and the board has never been created
// When  the command is started
// Then  GET /board answers 200 with the three fixed columns
//
//	And the columns hold no cards
//	And the board data file exists afterward
//
// (This is the board-flavored rewrite of the old fresh-path start test,
// which asserted the page's "no todos" state through the retired todo list
// read; empty-board page rendering is a ui card, the board's existence on
// startup is the server-side behavior pinned here.)
func TestFreshPathStartsEmpty(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "todos.db")     // dir exists, file absent
	boardPath := filepath.Join(dir, "kanban.db") // dir exists, file absent

	addr := freeAddr(t)
	startServer(t, addr, dbPath, boardPath) // must start successfully

	// The board exists with the three fixed columns holding no cards.
	got := getBoard(t, addr)
	if len(got.Columns) != len(wantColumnTitles) {
		t.Fatalf("GET /board columns = %d, want %d: %+v", len(got.Columns), len(wantColumnTitles), got.Columns)
	}
	for i, want := range wantColumnTitles {
		if got.Columns[i].Title != want {
			t.Errorf("column %d title = %q, want %q", i, got.Columns[i].Title, want)
		}
		if len(got.Columns[i].Cards) != 0 {
			t.Errorf("column %q holds %d cards, want 0", got.Columns[i].Title, len(got.Columns[i].Cards))
		}
	}

	// The started process created the board data file on disk.
	if _, err := os.Stat(boardPath); err != nil {
		t.Errorf("board data file after start: %v", err)
	}
}
