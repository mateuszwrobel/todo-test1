package main

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// Card server/02 — Fresh path starts empty.
// Given no data file exists at the configured path
// When  the command is started
// Then  it starts successfully and the page shows the "no todos" state
func TestFreshPathStartsEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "not-yet-created.db") // dir exists, file absent

	// Given: nothing exists at the configured path.
	addr := freeAddr(t)
	startServer(t, addr, dbPath, filepath.Join(t.TempDir(), "not-yet-board.db")) // must start successfully

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(page), "No todos") {
		t.Errorf("page does not state the empty (no todos) state:\n%s", page)
	}
}
