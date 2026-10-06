package main

import (
	"path/filepath"
	"strings"
	"testing"

	"todo/board"
)

// Card server/05 — Interrupted import leaves no half board.
// Given a todo data file exists and the board has never been created
// When  the server process stops hard during the import
// Then  the next start shows either the fully imported board or a clean
//
//	board with no trace of a partial import
//
//	And the import completes on that next start if it had not landed
//
// The stop is exercised through the import's failure side, which is what a
// hard stop leaves observable in either timing: Store.Import commits in one
// transaction (board/14), so between Begin and Commit the only states a kill
// can produce are committed-fully or untouched — the same all-or-nothing
// boundary a mid-validation stop hits, and the boundary this test drives
// deterministically. A poisoned source row makes Import refuse before any
// statement executes, so the "stopped during import" process is one whose
// import never landed: startup fails loudly (the workplan's stated loud
// failure — a stated error, no partially serving process), and what it
// leaves behind must be provably a clean board, after which the next start
// completes the import.
func TestInterruptedImportLeavesNoHalfBoard(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.db") // absent: the board has never been created
	todoPath := filepath.Join(dir, "todos.db")
	addr := freeAddr(t)

	// Good material first, then the poison: a whitespace-only title (the
	// board's text rule trims it to blank) and an over-long one. Either
	// alone stops the import; both pin that one bad entry refuses the
	// whole import, not the entries beside it.
	seedTodoSource(t, todoPath, []fixtureTodo{
		{Title: "A"},
		{Title: "D", Done: true},
		{Title: "   "},
		{Title: strings.Repeat("z", board.MaxTextLen+1)},
	})
	sourceBefore := fileSHA256(t, todoPath)

	// The start whose import does not land: loud failure, stated reason,
	// nothing listening.
	out := runExpectFailure(t, "--addr", addr, "--board-db", boardPath, "--todo-db", todoPath)
	t.Logf("stated failure: %s", out)
	if !strings.Contains(out, "import todos") {
		t.Errorf("failure reason does not name the todo import: %s", out)
	}
	if dialable(t, addr) {
		t.Errorf("address %s is listening despite the failed import", addr)
	}

	// No half board: the failed import executed no statement, so the board
	// shows no trace of a partial import — the imported-but-blank A, the
	// done-side D, or anything else would all be traces. Whatever file the
	// start left, it holds zero cards.
	if cards := boardCardsDirect(t, boardPath); len(cards) != 0 {
		t.Fatalf("failed import left a half board: %+v", cards)
	}
	// The source itself survived the failed start byte-for-byte — read-only
	// held even on the failure path.
	if after := fileSHA256(t, todoPath); after != sourceBefore {
		t.Fatalf("todo data file changed during the failed import: sha256 %s, want %s", after, sourceBefore)
	}

	// The import completes on the next start if it had not landed: with the
	// poison removed — the workplan's repair path, the server never invents
	// data — the still-empty board re-imports and the fully imported board
	// is all the card's alternative allows.
	deleteTodoSourceRows(t, todoPath, "   ", strings.Repeat("z", board.MaxTextLen+1))

	startServer(t, addr, boardPath, "--todo-db", todoPath)
	got := getBoard(t, addr)
	assertColumnLayout(t, got.Columns[0], "To Do", board.Todo, "A")
	assertColumnLayout(t, got.Columns[1], "In Progress", board.InProgress)
	assertColumnLayout(t, got.Columns[2], "Done", board.Done, "D")
	if ids := boardCardIDs(t, got); len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("completing import wrote card ids %v, want the fresh contiguous [1 2]", ids)
	}
}

// boardCardsDirect reads the board file straight through the board module —
// the between-processes view of the truth, used when no server is (or may
// be) running on the path.
func boardCardsDirect(t *testing.T, path string) []board.Card {
	t.Helper()
	store, err := board.Open(path)
	if err != nil {
		t.Fatalf("open board %s for direct read: %v", path, err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close board %s: %v", path, err)
		}
	}()
	listing, err := store.List()
	if err != nil {
		t.Fatalf("list board %s: %v", path, err)
	}
	var cards []board.Card
	for _, column := range listing {
		cards = append(cards, column.Cards...)
	}
	return cards
}
