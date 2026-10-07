package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"

	_ "modernc.org/sqlite" // registers the driver for the seeding connection below

	"todo/board"
)

// Card api/01 — Board read shape.
// Given a board holding cards in all three columns
// When a client sends GET /board
// Then the response is 200
//
//	And the body carries columns in the fixed order todo, in_progress, done
//	  with their display titles
//	And each column's cards array is in position order with id, title,
//	  column, position
func TestGetBoardIsFixedColumnsWithDisplayTitlesInPositionOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kanban.db")
	store := openBoardStoreAt(t, path)

	// Seed all three columns: todo through Create (the real append
	// primitive); in_progress/done by direct insert through the same schema
	// (the contract operation relocating cards there lands later, board/06 —
	// same pattern as board/store_list_test.go). The in_progress pair is
	// inserted bottom-first so position order and insertion order disagree:
	// only a position-ordered read can satisfy the fixture.
	for _, title := range []string{"first-todo", "second-todo"} {
		if _, err := store.Create(title); err != nil {
			t.Fatalf("seed Create(%q): %v", title, err)
		}
	}
	seedDirect(t, path, board.InProgress, "later-bottom", 1)
	seedDirect(t, path, board.InProgress, "earlier-top", 0)
	seedDirect(t, path, board.Done, "shipped", 0)

	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/board")
	if err != nil {
		t.Fatalf("GET /board: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	t.Logf("GET /board body: %s", body)

	// Decode generically so the JSON field names themselves are asserted,
	// not just a struct that happens to tolerate extra keys.
	var raw struct {
		Columns []struct {
			Title string                   `json:"title"`
			Cards []map[string]interface{} `json:"cards"`
		} `json:"columns"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("body is not the contract JSON: %v (body %s)", err, body)
	}
	if got, want := rawKeys(body), []string{"columns"}; !equalKeys(got, want) {
		t.Fatalf("top-level keys = %v, want exactly %v (body %s)", got, want, body)
	}
	if len(raw.Columns) != 3 {
		t.Fatalf("columns = %d, want the fixed 3 (body %s)", len(raw.Columns), body)
	}

	wantTitles := []string{"To Do", "In Progress", "Done"}
	wantNames := []string{"todo", "in_progress", "done"}
	wantOrder := [][]string{{"first-todo", "second-todo"}, {"earlier-top", "later-bottom"}, {"shipped"}}
	for i, col := range raw.Columns {
		if col.Title != wantTitles[i] {
			t.Errorf("column %d title = %q, want %q (fixed order with display titles)", i, col.Title, wantTitles[i])
		}
		for pos, card := range col.Cards {
			if !equalKeys(mapKeys(card), []string{"assignee", "column", "id", "position", "title"}) {
				t.Fatalf("column %d card %d keys = %v, want exactly the five contract fields id, title, column, position, assignee", i, pos, mapKeys(card))
			}
			// These cards are all seeded unassigned, and the contract's
			// DTO row says assignee is present on EVERY card — name or
			// null — so each payload here carries the explicit null (api/13).
			if v, present := card["assignee"]; !present || v != nil {
				t.Errorf("column %d card %d assignee = %v (present=%v), want explicit null — seeded cards are unassigned (api/13)", i, pos, v, present)
			}
			if got := card["title"]; got != wantOrder[i][pos] {
				t.Errorf("column %d position %d title = %v, want %q (cards in position order)", i, pos, got, wantOrder[i][pos])
			}
			if got := card["column"]; got != wantNames[i] {
				t.Errorf("column %d card %d column = %v, want %q", i, pos, got, wantNames[i])
			}
			if got := card["position"]; got != float64(pos) {
				t.Errorf("column %d card %d position = %v, want %d (ascending)", i, pos, got, pos)
			}
			id, ok := card["id"].(float64)
			if !ok || id < 1 {
				t.Errorf("column %d card %d id = %v, want a positive integer", i, pos, card["id"])
			}
		}
	}
}

// An empty column is an empty array on the contract — [], never null (the
// established convention of every list endpoint in this module).
func TestGetBoardEmptyColumnsAreEmptyArrays(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openBoardStore(t)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/board")
	if err != nil {
		t.Fatalf("GET /board: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var raw struct {
		Columns []struct {
			Title json.RawMessage `json:"title"`
			Cards json.RawMessage `json:"cards"`
		} `json:"columns"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("body is not the contract JSON: %v (body %s)", err, body)
	}
	if len(raw.Columns) != 3 {
		t.Fatalf("columns = %d, want the fixed 3 even when empty (body %s)", len(raw.Columns), body)
	}
	for i, col := range raw.Columns {
		if string(col.Cards) != "[]" {
			t.Errorf("column %d cards = %s, want [] (never null)", i, col.Cards)
		}
	}
}

// The failure path ui/03 depends on: a board store whose read fails answers
// a non-200 — the stated store-failure status of this module. The closed
// store makes List fail for real (no fake).
func TestGetBoardStoreFailureIs500(t *testing.T) {
	store := openBoardStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("closing the store to force the failure path: %v", err)
	}

	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/board")
	if err != nil {
		t.Fatalf("GET /board: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (List failure maps to the store-failure status)", resp.StatusCode)
	}
}

// rawKeys lists the top-level JSON object's keys, sorted.
func rawKeys(body []byte) []string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil
	}
	return mapKeys(m)
}

// mapKeys lists a decoded object's keys, sorted.
func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func equalKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// openBoardStoreAt opens a real board store at an explicit path so a second
// connection can seed it.
func openBoardStoreAt(t *testing.T, path string) *board.Store {
	t.Helper()
	store, err := board.Open(path)
	if err != nil {
		t.Fatalf("board.Open(%q): %v", path, err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// seedDirect places one card at an explicit position of a column through a
// separate connection to the same file — the api package cannot reach the
// store's handle (unexported), and no contract operation relocates cards
// until board/06. Exactly the Given's state, same schema, same constraints.
func seedDirect(t *testing.T, path string, col board.Column, title string, position int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("seed open %q: %v", path, err)
	}
	defer db.Close()
	res, err := db.Exec(`INSERT INTO cards (title, "column", position) VALUES (?, ?, ?)`,
		title, string(col), position)
	if err != nil {
		t.Fatalf("seed insert(%q, %q, %d): %v", col, title, position, err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("seed insert(%q, %q, %d): rows affected %d, err %v", col, title, position, n, err)
	}
}
