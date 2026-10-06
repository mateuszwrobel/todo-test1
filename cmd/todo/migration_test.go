package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"syscall"
	"testing"

	"todo/board"

	_ "modernc.org/sqlite" // fixture builder only — test sources are outside the server/08 driver pin
)

// Card server/02 — First start imports existing todos once.
// Given a todo data file exists holding not-done todos [A, B, C] and done
//
//	todos [D, E]
//
//	And the board has never been created
//
// When  the server starts
// Then  the board holds [A, B, C] as cards in the todo column in that order
//
//	and [D, E] in the done column in that order
//
//	And every card carries a fresh identifier
//	And the todo data file is unchanged on disk
//
// When  the server restarts again
// Then  the board is exactly as it was — no card is re-imported
func TestFirstStartImportsExistingTodosOnce(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.db") // absent: the board has never been created
	todoPath := filepath.Join(dir, "todos.db")
	addr := freeAddr(t)

	// The source is built with its creation order deliberately interleaved
	// across done states, and with two deleted rows (X, Y) in the middle of
	// its id sequence — deleted rows never re-import, and the id gaps prove
	// the cards' identifiers are reissued, not carried over (ids are 1..5
	// contiguous on the board although the surviving todos id them 1,2,4,5,7).
	// Creation order: A(1) D(2) [X(3)] B(4) E(5) [Y(6)] C(7).
	seedTodoSource(t, todoPath, []fixtureTodo{
		{Title: "A"}, {Title: "D", Done: true}, {Title: "X"},
		{Title: "B"}, {Title: "E", Done: true}, {Title: "Y", Done: true},
		{Title: "C"},
	})
	deleteTodoSourceRows(t, todoPath, "X", "Y")
	sourceBefore := fileSHA256(t, todoPath)

	srv := startServer(t, addr, boardPath, "--todo-db", todoPath)

	got := getBoard(t, addr)
	if len(got.Columns) != len(wantColumnTitles) {
		t.Fatalf("GET /board columns = %d, want %d: %+v", len(got.Columns), len(wantColumnTitles), got.Columns)
	}

	// Mapping and order: not-done by creation order into todo top-to-bottom,
	// done likewise into done; in_progress — which the old store never had —
	// holds nothing.
	assertColumnLayout(t, got.Columns[0], "To Do", board.Todo, "A", "B", "C")
	assertColumnLayout(t, got.Columns[1], "In Progress", board.InProgress)
	assertColumnLayout(t, got.Columns[2], "Done", board.Done, "D", "E")

	// Fresh identifiers: the five cards own the board's first five
	// autoincrements, contiguous — the source's gapped ids were not
	// preserved, every card is new to this board.
	ids := boardCardIDs(t, got)
	wantIDs := []int64{1, 2, 3, 4, 5}
	if len(ids) != len(wantIDs) {
		t.Fatalf("imported card ids = %v, want the contiguous fresh set %v", ids, wantIDs)
	}
	for i := range wantIDs {
		if ids[i] != wantIDs[i] {
			t.Fatalf("imported card ids = %v, want the contiguous fresh set %v (source ids must not survive)", ids, wantIDs)
		}
	}

	// The todo data file is unchanged on disk: the server opened it strictly
	// read-only.
	if after := fileSHA256(t, todoPath); after != sourceBefore {
		t.Fatalf("todo data file changed on disk after import: sha256 %s, want %s", after, sourceBefore)
	}

	// Restart: the board is exactly as it was — card-for-card, identifiers
	// included. The import marker committed with the first start's cards
	// answers the guard, so the still-present source is past evidence. A
	// re-import would add duplicates with new ids; equality of the whole
	// payload (ids stable across the store's close/reopen) is the
	// "no card is re-imported" pin.
	terminateServer(t, srv)
	startServer(t, addr, boardPath, "--todo-db", todoPath)

	after := getBoard(t, addr)
	if !equalBoardPayload(after, got) {
		t.Fatalf("board changed across restart — cards re-imported?\nfirst  %+v\nsecond %+v", got.Columns, after.Columns)
	}
	if again := fileSHA256(t, todoPath); again != sourceBefore {
		t.Fatalf("todo data file changed on disk across restart: sha256 %s, want %s", again, sourceBefore)
	}
}

// Card server/02 (guard arm) — the todo source is ignored once the import
// decision has been made. Given a board whose first start completed with no
// todo file to import (the start records the marker even when it places
// nothing) and a todo data file that appeared later, When the server starts,
// Then the todo file changes nothing: the board stays exactly as it was and
// the file is untouched. This is the once in "import once" read from the
// other side — the guard consults the board's recorded decision, never the
// source.
func TestImportGuardIgnoresTodoSourceOfCreatedBoard(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.db")
	todoPath := filepath.Join(dir, "todos.db") // absent for the first start
	addr := freeAddr(t)

	// First start: no source exists — the board is created and put to use.
	srv := startServer(t, addr, boardPath, "--todo-db", todoPath)
	resident := createBoardCard(t, addr, "resident")
	terminateServer(t, srv)

	// A todo data file appears after the board was created and served.
	seedTodoSource(t, todoPath, []fixtureTodo{{Title: "Q"}, {Title: "R", Done: true}})
	sourceBefore := fileSHA256(t, todoPath)

	startServer(t, addr, boardPath, "--todo-db", todoPath)

	// The guard skipped the import: exactly the resident card, nothing from
	// the source, no new identifiers.
	got := getBoard(t, addr)
	assertColumnLayout(t, got.Columns[0], "To Do", board.Todo, "resident")
	assertColumnLayout(t, got.Columns[1], "In Progress", board.InProgress)
	assertColumnLayout(t, got.Columns[2], "Done", board.Done)
	if ids := boardCardIDs(t, got); len(ids) != 1 || ids[0] != resident {
		t.Fatalf("guard did not hold — board cards = %v, want exactly the resident [%d]", ids, resident)
	}
	if after := fileSHA256(t, todoPath); after != sourceBefore {
		t.Fatalf("ignored todo data file was still modified: sha256 %s, want %s", after, sourceBefore)
	}
}

// --- migration-test fixtures ---------------------------------------------

// fixtureTodo is one row of the throwaway todo data file the fixtures build:
// a title and a done state, in insertion (= creation) order.
type fixtureTodo struct {
	Title string
	Done  bool
}

// seedTodoSource creates a todo data file at path holding rows with the
// given titles and done states, in the given order — autoincrement assigns
// the creation identifiers, exactly as the superseded store did. Titles go
// in raw: the fixtures must be able to hand the migration material the old
// app itself would have refused (server/05's poisoned rows).
//
// The schema is the superseded store's table quoted verbatim from
// git 48a4ca5^:todos/store.go:
//
//	CREATE TABLE IF NOT EXISTS todos (
//		id integer primary key autoincrement,
//		title text not null,
//		done integer not null default 0
//	)
//
// Test fixtures build throwaway files in temp dirs through the driver
// directly; the server/08 pin scopes the driver import to non-test module
// sources, and no run ever points at the repo's own data files.
func seedTodoSource(t *testing.T, path string, rows []fixtureTodo) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture todo source %s: %v", path, err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS todos (
	id integer primary key autoincrement,
	title text not null,
	done integer not null default 0
)`); err != nil {
		t.Fatalf("create fixture todo schema: %v", err)
	}
	for _, row := range rows {
		done := 0
		if row.Done {
			done = 1
		}
		if _, err := db.Exec(`INSERT INTO todos (title, done) VALUES (?, ?)`, row.Title, done); err != nil {
			t.Fatalf("seed fixture todo %q: %v", row.Title, err)
		}
	}
}

// deleteTodoSourceRows removes the named rows, leaving id gaps behind —
// creation order keeps skipping holes exactly as the old store's deletes did.
func deleteTodoSourceRows(t *testing.T, path string, titles ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture todo source %s: %v", path, err)
	}
	defer db.Close()
	for _, title := range titles {
		if _, err := db.Exec(`DELETE FROM todos WHERE title = ?`, title); err != nil {
			t.Fatalf("delete fixture todo %q: %v", title, err)
		}
	}
}

// fileSHA256 is the byte pin for the "todo data file is unchanged on disk"
// claims: the digest before any start must equal the digest after.
func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s for hashing: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// terminateServer stops a running server (SIGTERM) and waits for its exit,
// the restart harness's stop step.
func terminateServer(t *testing.T, srv *exec.Cmd) {
	t.Helper()
	if err := srv.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	if err := srv.Wait(); err != nil {
		t.Logf("server exited: %v", err) // startServer's cleanup tolerates the double wait
	}
}

// assertColumnLayout pins one column by name: its display title, the store
// column its cards must all name, and the exact top-to-bottom titles —
// position j for entry j, so the stated order is the stored order.
func assertColumnLayout(t *testing.T, got boardColumn, wantTitle string, wantColumn board.Column, wantTitles ...string) {
	t.Helper()
	if got.Title != wantTitle {
		t.Fatalf("column title = %q, want %q", got.Title, wantTitle)
	}
	if len(got.Cards) != len(wantTitles) {
		t.Fatalf("column %q holds %d cards, want %d: %+v", wantTitle, len(got.Cards), len(wantTitles), got.Cards)
	}
	for j, want := range wantTitles {
		card := got.Cards[j]
		if card.Title != want || card.Column != wantColumn || card.Position != j {
			t.Errorf("column %q card %d = %+v, want title %q in %q at position %d",
				wantTitle, j, card, want, wantColumn, j)
		}
	}
}

// boardCardIDs returns every card id on the payload, sorted — the shape the
// fresh-identifier pins compare against.
func boardCardIDs(t *testing.T, p boardPayload) []int64 {
	t.Helper()
	var ids []int64
	for _, col := range p.Columns {
		for _, card := range col.Cards {
			ids = append(ids, card.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// equalBoardPayload compares two GET /board bodies cell-for-cell, ids
// included — the across-restart equality.
func equalBoardPayload(a, b boardPayload) bool {
	if len(a.Columns) != len(b.Columns) {
		return false
	}
	for i := range a.Columns {
		ac, bc := a.Columns[i], b.Columns[i]
		if ac.Title != bc.Title || len(ac.Cards) != len(bc.Cards) {
			return false
		}
		for j := range ac.Cards {
			if ac.Cards[j] != bc.Cards[j] {
				return false
			}
		}
	}
	return true
}
