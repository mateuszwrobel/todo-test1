package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"todo/todos"
)

// Card api/04 — List todos.
// Given todos exist
// When  a GET /todos arrives
// Then  the response is 200 with the JSON array of todos ordered by id
//
//	ascending
func TestGetTodosReturnsOrderedJSONArray(t *testing.T) {
	store := openStore(t)
	seeds := []string{"alpha", "beta", "gamma"}
	var want []todos.Todo
	for _, title := range seeds {
		created, err := store.Create(title)
		if err != nil {
			t.Fatalf("seed Create(%q): %v", title, err)
		}
		want = append(want, created)
	}

	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/todos")
	if err != nil {
		t.Fatalf("GET /todos: %v", err)
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
	var got []todos.Todo
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("body is not a JSON array: %v (body %s)", err, body)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d todos, want %d: %s", len(got), len(want), body)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("todo %d = %+v, want %+v", i, got[i], want[i])
		}
		if i > 0 && got[i].ID <= got[i-1].ID {
			t.Fatalf("ids not ascending at %d: %s", i, body)
		}
	}
}

// The list response is an array even with no todos — never null.
func TestGetTodosEmptyIsJSONArray(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openStore(t)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/todos")
	if err != nil {
		t.Fatalf("GET /todos: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var got []todos.Todo
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("body is not a JSON array: %v (body %s)", err, body)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("empty list body = %s, want []", body)
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
