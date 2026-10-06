package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// Card ui/04 (composition arm) — POST /cards is mounted on the composed
// listener.
// Given a freshly started server
// When  POST /cards arrives with a valid body
// Then  the JSON contract answers it (201, not 404) — the route reaches the
//
//	api handler, not the page fallback — and the card is on the board read
//
// The page's create fragment endpoint needs this mount: without it the
// contract exists only in-process and the page's create would 404 through
// the listener. Surface assertion beside the existing fresh-path /board
// assertions.
func TestCardsEndpointMounted(t *testing.T) {
	dir := t.TempDir()
	addr := freeAddr(t)
	startServer(t, addr, filepath.Join(dir, "todos.db"), filepath.Join(dir, "kanban.db"))

	resp, err := http.Post("http://"+addr+"/cards", "application/json",
		strings.NewReader(`{"title": "mount probe"}`))
	if err != nil {
		t.Fatalf("POST /cards: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /cards status = %d, want 201 (is /cards mounted on the api handler?)", resp.StatusCode)
	}

	// And the created card is on the board, in To Do — the mount reaches
	// the same store the board read serves.
	got := getBoard(t, addr)
	for _, col := range got.Columns {
		if col.Title != "To Do" && len(col.Cards) != 0 {
			t.Errorf("column %q holds %d cards, want 0", col.Title, len(col.Cards))
		}
		if col.Title == "To Do" && (len(col.Cards) != 1 || col.Cards[0].Title != "mount probe") {
			t.Errorf("To Do after create = %+v, want the created card alone", col.Cards)
		}
	}
}
