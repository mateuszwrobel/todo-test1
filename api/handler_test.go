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

// GET /todos retired by api/01: ServeMux answers the removed route as an
// existing path with no matching method — TestGetTodosRetiredServes405MethodNotAllowed.
// Given the JSON contract handler with GET /todos deleted from its routes
// When a client sends GET /todos (other methods still own this path)
// Then the router answers 405 Method Not Allowed, quoting POST in Allow
func TestGetTodosRetiredServes405MethodNotAllowed(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openStore(t), openBoardStore(t)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/todos")
	if err != nil {
		t.Fatalf("GET /todos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /todos status = %d, want 405 (route retired, path still owned by POST /todos)", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); !strings.Contains(allow, http.MethodPost) {
		t.Errorf("Allow = %q, want it to list POST (the surviving /todos method)", allow)
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
