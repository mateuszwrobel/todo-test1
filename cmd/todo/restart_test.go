package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"todo/board"
)

// Card server/04 — Board survives server restart.
// Given the server ran, the board accumulated cards in a mix of columns and
// positions, and the process stopped
// When  the server starts again at the same paths
// Then  the board lists the same cards with the same texts, columns, and positions
//
// Evolution, not duplication: KW1's TestRestartResumesState (itself a
// retarget of the todo-flavored restart test, dispositioned under the old
// server/08) already proved the resume machinery — run 1 reads committed
// state over HTTP, SIGTERM, run 2 at the same paths must show it unchanged.
// The KW1 disposition deferred the mixed shape ("until the move operation
// lands, every seeded card legitimately sits in the todo column"); moves
// landed at KW5, so this is the card-shaped upgrade:
//
//   - the accumulation happens while the server runs, over the composed HTTP
//     surface — creates, an edit, column moves, an explicit indexed move,
//     and a delete — so the resting state is a genuine mix across all three
//     columns at positions that disagree with creation order;
//   - run 1's board is pinned to the exact truth the operation sequence
//     dictates (an independent layout table, so the restart comparison
//     cannot pass vacuously on an arbitrary board);
//   - run 2 must match run 1 cell-for-cell — one Card equality pins id,
//     title, column, and position together.
//
// The page-side resume assertion retired with the todo page at KW1; board
// page rendering across restart belongs to the ui lane.
func TestBoardSurvivesServerRestart(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.db") // absent: run 1 creates it
	addr := freeAddr(t)

	// Run 1: the server itself accumulates the board.
	srv := startServer(t, addr, boardPath)

	alpha := createBoardCard(t, addr, "alpha")
	bravo := createBoardCard(t, addr, "bravo")
	charlie := createBoardCard(t, addr, "charlie")
	delta := createBoardCard(t, addr, "delta")
	echo := createBoardCard(t, addr, "echo")

	// Mix the resting state: one edit, three column moves (Change's
	// bottom-append), one indexed move to the top of an occupied column,
	// and a delete whose gap closes. The resulting layout — derived from
	// the operations' stated semantics, not from observing the server —
	// disagrees with creation order everywhere it can.
	patchCard(t, addr, alpha, `{"title": "alpha v2"}`)
	patchCard(t, addr, bravo, `{"column": "in_progress"}`)
	patchCard(t, addr, charlie, `{"column": "in_progress"}`)
	patchCard(t, addr, echo, `{"column": "done"}`)
	patchCard(t, addr, delta, `{"column": "in_progress", "position": 0}`)
	deleteCard(t, addr, charlie)

	// Exact truth after run 1's operations: todo holds only the edited
	// alpha; in_progress holds delta (indexed insert to 0) above bravo
	// (charlie's delete closed its gap at 2); done holds echo. charlie is
	// gone from every column.
	want := []boardColumn{
		{Title: "To Do", Cards: []board.Card{
			{ID: alpha, Title: "alpha v2", Column: board.Todo, Position: 0},
		}},
		{Title: "In Progress", Cards: []board.Card{
			{ID: delta, Title: "delta", Column: board.InProgress, Position: 0},
			{ID: bravo, Title: "bravo", Column: board.InProgress, Position: 1},
		}},
		{Title: "Done", Cards: []board.Card{
			{ID: echo, Title: "echo", Column: board.Done, Position: 0},
		}},
	}
	before := getBoard(t, addr)
	if !reflect.DeepEqual(before.Columns, want) {
		t.Fatalf("run 1 board does not hold the truth the operations dictate:\n got %+v\nwant %+v", before.Columns, want)
	}

	// Stop the process (SIGTERM), then start again at the same paths.
	// Clean-exit semantics are card server/07's own contract; here the
	// restart story.
	if err := srv.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	if err := srv.Wait(); err != nil {
		t.Logf("first run exited: %v", err)
	}

	startServer(t, addr, boardPath) // run 2

	// Run 2 lists the same cards with the same texts, columns, and
	// positions — cell-for-cell against run 1.
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

// patchCard sends PATCH /cards/{id} with the given JSON body, requires 200,
// and returns the card as the contract answered it.
func patchCard(t *testing.T, addr string, id int64, body string) board.Card {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, "http://"+addr+"/cards/"+strconv.FormatInt(id, 10),
		strings.NewReader(body))
	if err != nil {
		t.Fatalf("build PATCH /cards/%d: %v", id, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /cards/%d: %v", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH /cards/%d with %s status = %d, want 200", id, body, resp.StatusCode)
	}
	var card board.Card
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("PATCH /cards/%d body is not card JSON: %v", id, err)
	}
	return card
}

// deleteCard sends DELETE /cards/{id} and requires the contract's 204.
func deleteCard(t *testing.T, addr string, id int64) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, "http://"+addr+"/cards/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		t.Fatalf("build DELETE /cards/%d: %v", id, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /cards/%d: %v", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /cards/%d status = %d, want 204", id, resp.StatusCode)
	}
}
