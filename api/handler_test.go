package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"todo/board"
	"todo/todos"
)

// GET /todos retired by api/01; POST /todos — the surviving method that made
// the retired GET answer 405 back then — retired by api/02. With no method
// left owning the exact path /todos (the surviving endpoints live at
// /todos/{id}, which the path /todos does not match), ServeMux answers the
// path as a route that does not exist: 404, no Allow header.
// Given the JSON contract handler with the /todos collection route deleted
// When a client sends GET /todos (no method owns this path anymore)
// Then the router answers 404 Not Found
func TestGetTodosRetiredServes404PathFullyRetired(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openStore(t), openBoardStore(t)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/todos")
	if err != nil {
		t.Fatalf("GET /todos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /todos status = %d, want 404 (api/02 retired the last owner of the path; api/01's 405 pin held only while POST /todos survived)", resp.StatusCode)
	}
}

// Card api/02 — POST /todos retired as POST /cards lands. The same pin style
// as the retired GET above: name the removed route, pin the router's true
// answer for it.
// Given the JSON contract handler with POST /todos deleted from its routes
// When a client sends POST /todos with a create body
// Then the router answers 404 — no route owns the path (a 405 would need a
// surviving method on the exact path, and none remains), and nothing is
// created in the todo store
func TestPostTodosRetiredServes404CreatesNothing(t *testing.T) {
	store := openStore(t)
	srv := httptest.NewServer(NewHandler(store, openBoardStore(t)))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/todos", "application/json", strings.NewReader(`{"title": "Buy milk"}`))
	if err != nil {
		t.Fatalf("POST /todos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("POST /todos status = %d, want 404 (route retired: ServeMux has no pattern matching the path at all)", resp.StatusCode)
	}
	if list, err := store.List(); err != nil {
		t.Fatalf("todos store.List: %v", err)
	} else if len(list) != 0 {
		t.Errorf("the retired POST /todos created todos: %+v", list)
	}
}

func openStore(t *testing.T) *todos.Store {
	t.Helper()
	store, err := todos.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("todos.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// openBoardStore opens a real board store in a fresh temp dir. The handler is
// always built over a real store — no fakes (wave law: wire against real code).
func openBoardStore(t *testing.T) *board.Store {
	t.Helper()
	store, err := board.Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatalf("board.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}
