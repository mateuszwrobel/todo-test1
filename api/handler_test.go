package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"todo/board"
)

// The todo surface is fully retired: api/01 deleted GET /todos, api/02
// deleted POST /todos, api/05 deleted PATCH /todos/{id}, and api/10 deleted
// DELETE /todos/{id} — the last surviving todo endpoint. With no method
// owning either todo path pattern, ServeMux answers every todo route as a
// path that does not exist: 404, no Allow header. Given the JSON contract
// handler with the whole todo surface deleted
// When a client sends GET /todos (no method owns this path)
// Then the router answers 404 Not Found
func TestGetTodosRetiredServes404PathFullyRetired(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openBoardStore(t)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/todos")
	if err != nil {
		t.Fatalf("GET /todos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /todos status = %d, want 404 (no route owns the path; api/02 retired its last owner)", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); allow != "" {
		t.Errorf("Allow = %q, want no Allow header — a 405 would need a surviving method on the path, and none remains", allow)
	}
}

// Card api/02's pin, re-executed after api/10 retired the item-path
// deletion: the same no-route 404, and nothing is created anywhere — the
// todo store itself no longer exists, and the board is untouched.
// Given the JSON contract handler with POST /todos deleted from its routes
// When a client sends POST /todos with a create body
// Then the router answers 404 — no route owns the path (a 405 would need a
// surviving method on the exact path, and none remains) — and the board is
// unchanged
func TestPostTodosRetiredServes404CreatesNothing(t *testing.T) {
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	before := boardSnapshot(t, store)
	resp, err := http.Post(srv.URL+"/todos", "application/json", strings.NewReader(`{"title": "Buy milk"}`))
	if err != nil {
		t.Fatalf("POST /todos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST /todos status = %d, want 404 (route retired: ServeMux has no pattern matching the path at all)", resp.StatusCode)
	}
	assertBoardUnchanged(t, store, before)
}

// Card api/05's pin, re-executed after api/10 retired the last surviving
// todo endpoint: back then PATCH /todos/{id} answered 405 because
// DELETE /todos/{id} still owned the path pattern; with that deletion gone
// no method owns the path, so the router's true answer is the same 404 the
// collection path gives — no Allow header.
// Given the JSON contract handler with PATCH /todos/{id} deleted and
//
//	DELETE /todos/{id} retired at api/10
//
// When a client sends PATCH /todos/1 with a change body
// Then the router answers 404 Not Found with no Allow header
func TestPatchTodosRetiredServes404PathFullyRetired(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openBoardStore(t)))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/todos/1", strings.NewReader(`{"title": "renamed"}`))
	if err != nil {
		t.Fatalf("new PATCH request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /todos/1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("PATCH /todos/1 status = %d, want 404 (api/10 retired DELETE /todos/{id}, the last method owning the path; the 405 this route answered while it survived is gone with it)", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); allow != "" {
		t.Errorf("Allow = %q, want no Allow header — no method owns /todos/{id} anymore", allow)
	}
}

// Card api/10 — DELETE /todos/{id} retires as DELETE /cards/{id} lands. The
// ledger states it (workplans/dependencies_kanban.md §KW4: "todo DELETE
// retires here"); this is the retirement pin, same style as the retired
// routes above — name the removed route, pin the router's true answer.
// Unlike PATCH's former 405 pin, the answer is the full-retirement 404:
// this deletion was the last method ever owning /todos/{id}, so nothing
// remains to make the path match with a method mismatch, and no Allow
// header accompanies the 404.
// Given the JSON contract handler with DELETE /todos/{id} deleted from its
// routes
// When a client sends DELETE /todos/1
// Then the router answers 404 Not Found with no Allow header
func TestDeleteTodosRetiredServes404PathFullyRetired(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openBoardStore(t)))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/todos/1", nil)
	if err != nil {
		t.Fatalf("build DELETE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /todos/1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("DELETE /todos/1 status = %d, want 404 (route fully retired: no method owns the path pattern anymore)", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); allow != "" {
		t.Errorf("Allow = %q, want no Allow header — the retired deletion was the path's last owner", allow)
	}
}

// openBoardStore opens a real board store in a fresh temp dir. The handler
// is always built over a real store — no fakes (wave law: wire against real
// code). It is now the only store the contract reaches: the todo store
// package retired with the last todo endpoint (api/10).
func openBoardStore(t *testing.T) *board.Store {
	t.Helper()
	store, err := board.Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatalf("board.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}
