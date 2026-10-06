package main

import (
	"path/filepath"
	"testing"

	"todo/board"
)

// Card server/02 (delete-all arm) — the once outlives every card the import
// placed. This is the live-reproduced resurrection defect turned into a
// green pin: first start imports the todos, the user then deletes every
// imported card, and the server restarts with the superseded todo file
// STILL present. The deleted todos must stay deleted — the board is empty,
// with no fresh-identifier copies. Under the guard this fix replaces (board
// emptiness) this flow resurrected the whole list, because an emptied board
// re-answered "never created" while the marker shows the decision was made
// and finished long ago.
//
// Given a todo data file exists holding not-done todos [A, C] and a done
//
//	todo [B], and the board has never been created
//
// When  the server starts
// Then  the board holds the imported cards
// When  every card is deleted
//
//	And the server restarts with the todo data file still present
//
// Then  all three columns hold no cards — nothing re-imported
//
//	And the todo data file is unchanged on disk
func TestDeleteAllAfterImportRestartDoesNotResurrectTodos(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.db") // absent: the board has never been created
	todoPath := filepath.Join(dir, "todos.db")   // present for EVERY start — the repro's premise
	addr := freeAddr(t)

	seedTodoSource(t, todoPath, []fixtureTodo{
		{Title: "A"}, {Title: "B", Done: true}, {Title: "C"},
	})
	sourceBefore := fileSHA256(t, todoPath)

	srv := startServer(t, addr, boardPath, "--todo-db", todoPath)

	// First start imported exactly once: the cards are there to be deleted.
	imported := getBoard(t, addr)
	assertColumnLayout(t, imported.Columns[0], "To Do", board.Todo, "A", "C")
	assertColumnLayout(t, imported.Columns[1], "In Progress", board.InProgress)
	assertColumnLayout(t, imported.Columns[2], "Done", board.Done, "B")
	ids := boardCardIDs(t, imported)
	if len(ids) != 3 {
		t.Fatalf("first start imported %v, want the three cards [1 2 3]", ids)
	}

	// The user clears the board — every card, every column, through the
	// contract itself.
	for _, id := range ids {
		deleteCard(t, addr, id)
	}
	cleared := getBoard(t, addr)
	assertColumnLayout(t, cleared.Columns[0], "To Do", board.Todo)
	assertColumnLayout(t, cleared.Columns[1], "In Progress", board.InProgress)
	assertColumnLayout(t, cleared.Columns[2], "Done", board.Done)

	// Restart with the source still present: the marker, not the emptiness,
	// answers the guard — an empty board stays empty. A resurrection would
	// show A, C and B back with fresh identifiers (4, 5, 6).
	terminateServer(t, srv)
	startServer(t, addr, boardPath, "--todo-db", todoPath)

	after := getBoard(t, addr)
	assertColumnLayout(t, after.Columns[0], "To Do", board.Todo)
	assertColumnLayout(t, after.Columns[1], "In Progress", board.InProgress)
	assertColumnLayout(t, after.Columns[2], "Done", board.Done)
	if resurrected := boardCardIDs(t, after); len(resurrected) != 0 {
		t.Fatalf("deleted todos resurrected across the restart: %v — the import ran a second time", resurrected)
	}

	// The source was read at most once and never written.
	if again := fileSHA256(t, todoPath); again != sourceBefore {
		t.Fatalf("todo data file changed across the delete-all restart: sha256 %s, want %s", again, sourceBefore)
	}
}
