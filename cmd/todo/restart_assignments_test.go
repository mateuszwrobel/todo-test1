package main

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
	"syscall"
	"testing"

	"todo/board"
)

// Card server/09 — Assignments survive restart.
// Given assignments set over HTTP across all columns, including an assigned
// card in Done and cards with no assignee
// When  the process restarts with the same data files
// Then  every card reports exactly the same assignee state as before
//
// The KW6 restart machinery is restart_test.go's (card server/04): run 1
// accumulates state over the composed HTTP surface, SIGTERM, run 2 at the
// same paths. This is the assignment-shaped slice of the same pattern:
//
//   - every assignment lands through the composed HTTP surface — PATCH
//     /cards/{id} carrying the assignee field (name assigns, null clears),
//     column moves carrying assignments along — so the stored states are
//     the contract's own writes, never a direct data-file shortcut;
//   - the resting state covers the scenario's whole Given: assignments in
//     every column (To Do, In Progress, and one in Done), a card assigned
//     and then cleared again, and a card never assigned at all;
//   - the Done-assigned card is staged assign-then-move: the freeze
//     refuses assigning a card that already sits in Done, so this is also
//     the honest shape of "an assigned card in Done" over HTTP;
//   - "the same assignee state" is pinned twice: cell-for-cell on the
//     decoded Card equality (id, title, column, position, assignee) and
//     at the wire spelling — the assignee key present on EVERY card
//     payload, exactly a name or null, identical across the restart.
func TestAssignmentsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.db") // absent: run 1 creates it
	addr := freeAddr(t)

	// Run 1: the assignments accumulate over the server's own HTTP surface.
	srv := startServer(t, addr, boardPath)

	alpha := createBoardCard(t, addr, "alpha")
	bravo := createBoardCard(t, addr, "bravo")
	charlie := createBoardCard(t, addr, "charlie")
	delta := createBoardCard(t, addr, "delta")
	echo := createBoardCard(t, addr, "echo")

	// Mix the resting state (operation semantics, not observed output):
	//   alpha   — assigned Ada, stays in To Do (the pure assign direction);
	//   bravo   — assigned Grace, then moved to In Progress (a move places
	//             and carries the assignment along — assign survives a move);
	//   charlie — assigned Barbara, then cleared with null: the resting
	//             state is unassigned, proven through a real set-then-clear;
	//   delta   — assigned Linus, then moved into Done: the assigned Done
	//             card (assigning in place there would be the freeze's 422);
	//   echo    — never assigned: the plain unassigned card.
	patchCard(t, addr, alpha, `{"assignee": "Ada"}`)
	patchCard(t, addr, bravo, `{"assignee": "Grace"}`)
	patchCard(t, addr, bravo, `{"column": "in_progress"}`)
	patchCard(t, addr, charlie, `{"assignee": "Barbara"}`)
	patchCard(t, addr, charlie, `{"assignee": null}`)
	patchCard(t, addr, delta, `{"assignee": "Linus"}`)
	patchCard(t, addr, delta, `{"column": "done"}`)

	// Exact truth after run 1's operations — an independent layout table,
	// so the across-restart comparison cannot pass vacuously on whatever
	// the operations happened to produce.
	want := []boardColumn{
		{Title: "To Do", Cards: []board.Card{
			{ID: alpha, Title: "alpha", Column: board.Todo, Position: 0, Assignee: "Ada"},
			{ID: charlie, Title: "charlie", Column: board.Todo, Position: 1},
			{ID: echo, Title: "echo", Column: board.Todo, Position: 2},
		}},
		{Title: "In Progress", Cards: []board.Card{
			{ID: bravo, Title: "bravo", Column: board.InProgress, Position: 0, Assignee: "Grace"},
		}},
		{Title: "Done", Cards: []board.Card{
			{ID: delta, Title: "delta", Column: board.Done, Position: 0, Assignee: "Linus"},
		}},
	}
	before := getBoard(t, addr)
	if !reflect.DeepEqual(before.Columns, want) {
		t.Fatalf("run 1 board does not hold the assignments the operations dictate:\n got %+v\nwant %+v", before.Columns, want)
	}
	wireBefore := wireAssignees(t, addr)
	assertWireMatchesBoard(t, wireBefore, before, "run 1")

	// Stop the process (SIGTERM) and start again at the same data file and
	// address — the same stop step restart_test.go uses.
	if err := srv.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	if err := srv.Wait(); err != nil {
		t.Logf("first run exited: %v", err)
	}

	startServer(t, addr, boardPath) // run 2

	// Every card reports exactly the same assignee state as before —
	// cell-for-cell on the decoded equality (the Assignee field participates
	// in ==) and at the raw wire spelling of the field.
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
	wireAfter := wireAssignees(t, addr)
	if !reflect.DeepEqual(wireAfter, wireBefore) {
		t.Errorf("assignee states changed across restart at the wire:\nbefore %s\n after %s",
			renderWire(wireBefore), renderWire(wireAfter))
	}
	assertWireMatchesBoard(t, wireAfter, after, "run 2")
}

// wireAssignees reads GET /board and returns each card's assignee exactly as
// the wire spells it: a *string holding the name, or nil for the JSON null.
// The contract sentence is "assignee: name or null ON EVERY Card" (api/13),
// so a payload that carries NEITHER spelling — the key simply absent — is a
// third state the contract does not have, and this reads it as a failure
// rather than silently decoding it into "unassigned". The test package
// mirrors the wire shape locally, the main_test.go convention.
func wireAssignees(t *testing.T, addr string) map[int64]*string {
	t.Helper()
	resp, err := http.Get("http://" + addr + "/board")
	if err != nil {
		t.Fatalf("GET /board: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /board status = %d, want 200 (%s)", resp.StatusCode, body)
	}
	var raw struct {
		Columns []struct {
			Cards []map[string]json.RawMessage `json:"cards"`
		} `json:"columns"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("GET /board body %s: %v", body, err)
	}
	got := map[int64]*string{}
	for _, col := range raw.Columns {
		for _, card := range col.Cards {
			var id int64
			if err := json.Unmarshal(card["id"], &id); err != nil {
				t.Fatalf("card id in %v: %v", card, err)
			}
			field, ok := card["assignee"]
			if !ok {
				t.Errorf("card %d payload carries no assignee key — the contract spells every Card name-or-null, not absent", id)
				continue
			}
			if string(field) == "null" {
				got[id] = nil
				continue
			}
			var name string
			if err := json.Unmarshal(field, &name); err != nil || name == "" {
				t.Errorf("card %d assignee %s is neither a roster name nor null", id, field)
				continue
			}
			got[id] = &name
		}
	}
	return got
}

// assertWireMatchesBoard cross-checks the wire map against the decoded
// listing: each card's stored state (empty string = unassigned) must equal
// the wire's name-or-null, so a decoded-equality pass cannot hide a
// payload-side drift.
func assertWireMatchesBoard(t *testing.T, wire map[int64]*string, listed boardPayload, when string) {
	t.Helper()
	seen := map[int64]bool{}
	for _, col := range listed.Columns {
		for _, card := range col.Cards {
			seen[card.ID] = true
			w, ok := wire[card.ID]
			if !ok {
				t.Errorf("%s: card %d absent from the wire assignee map", when, card.ID)
				continue
			}
			if (w == nil) != (card.Assignee == "") || (w != nil && *w != card.Assignee) {
				t.Errorf("%s: card %d wire assignee %v does not match the decoded %q", when, card.ID, w, card.Assignee)
			}
		}
	}
	for id := range wire {
		if !seen[id] {
			t.Errorf("%s: wire carries assignee for card %d that GET /board does not list", when, id)
		}
	}
}

func renderWire(wire map[int64]*string) string {
	out := "{"
	for id := range wire {
		v := "null"
		if s := wire[id]; s != nil {
			v = strconv.Quote(*s)
		}
		out += strconv.FormatInt(id, 10) + ":" + v + ","
	}
	return out + "}"
}
