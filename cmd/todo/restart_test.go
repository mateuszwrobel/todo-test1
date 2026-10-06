package main

import (
	"path/filepath"
	"syscall"
	"testing"

	"todo/board"
)

// Card server/08 lane disposition of this old todo-flavored restart test:
// it asserted restart resumption through GET /todos, retired at KW1 (api/01).
// Rewritten to the restart state that is real now — board state survives a
// restart at the same paths, the board store persisting to SQLite, so a
// reopen replays exactly what was committed. The full board-restart scenario
// (cards in a mix of columns and positions) is kanban card server/04, which
// re-executes this story at KW6: until the move operation (board/06, KW5)
// lands, every seeded card legitimately sits in the todo column, and the
// page-side resume assertion (todo rows rendered after restart) retires with
// the todo page — board page rendering across restart is the ui lane's.
//
// Scenario kept minimal and honest: seeded cards → run 1 reads them over
// HTTP → SIGTERM → run 2 at the same paths lists the same cards with the
// same identifiers, titles, columns, and positions.
func TestRestartResumesState(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "todos.db")
	boardPath := filepath.Join(dir, "kanban.db")

	// Seed the board file directly while no server holds it (the
	// single-process assumption, as the todo store seeding did).
	store, err := board.Open(boardPath)
	if err != nil {
		t.Fatalf("seed board Open: %v", err)
	}
	titles := []string{"write report", "water plants", "buy milk"}
	for _, title := range titles {
		if _, err := store.Create(title); err != nil {
			t.Fatalf("seed board Create %q: %v", title, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("seed board Close: %v", err)
	}

	addr := freeAddr(t)

	// Run 1: read the board through the live HTTP surface.
	srv := startServer(t, addr, dbPath, boardPath)
	before := getBoard(t, addr)

	// Non-vacuity: run 1 really shows the seeded cards, in List's fixed
	// column and position order.
	var seeded []board.Card
	for _, col := range before.Columns {
		seeded = append(seeded, col.Cards...)
	}
	if len(seeded) != len(titles) {
		t.Fatalf("run 1 board holds %d cards, want %d: %+v", len(seeded), len(titles), seeded)
	}
	for i, title := range titles {
		if seeded[i].Title != title {
			t.Errorf("run 1 card %d title = %q, want %q", i, seeded[i].Title, title)
		}
	}

	// Stop the command (SIGTERM), then start it again at the same paths.
	// The exit status itself is the shutdown card's contract — here the
	// restart story.
	if err := srv.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	if err := srv.Wait(); err != nil {
		t.Logf("first run exited: %v", err)
	}

	startServer(t, addr, dbPath, boardPath)

	// Run 2: the board lists the same cards with the same texts, columns,
	// and positions.
	after := getBoard(t, addr)
	if len(after.Columns) != len(before.Columns) {
		t.Fatalf("board columns changed across restart:\nbefore %+v\n after %+v", before.Columns, after.Columns)
	}
	for i := range before.Columns {
		b, a := before.Columns[i], after.Columns[i]
		if a.Title != b.Title {
			t.Errorf("column %d title changed across restart: %q, want %q", i, a.Title, b.Title)
		}
		if len(a.Cards) != len(b.Cards) {
			t.Fatalf("column %q card count changed across restart:\nbefore %+v\n after %+v", b.Title, b.Cards, a.Cards)
		}
		for j := range b.Cards {
			if a.Cards[j] != b.Cards[j] {
				t.Errorf("column %q card %d changed across restart: %+v, want %+v", b.Title, j, a.Cards[j], b.Cards[j])
			}
		}
	}
}
