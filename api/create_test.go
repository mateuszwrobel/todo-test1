package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"todo/todos"
)

// Card api/01 — Create succeeds.
// Given the service is running
// When  a POST /todos with body `{ "title": "Buy milk" }` arrives
// Then  the response is 201 with the created todo's JSON, done false and a
//
//	fresh id
//
// Tested against the real store: persistence is proven, not faked.
func TestPostTodosCreates(t *testing.T) {
	store := openStore(t)
	srv := httptest.NewServer(NewHandler(store, openBoardStore(t)))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/todos", "application/json",
		strings.NewReader(`{"title": "Buy milk"}`))
	if err != nil {
		t.Fatalf("POST /todos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var created todos.Todo
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("body is not todo JSON: %v (body %s)", err, body)
	}
	if created.Title != "Buy milk" {
		t.Errorf("created title = %q, want %q", created.Title, "Buy milk")
	}
	if created.Done {
		t.Error("created done = true, want false")
	}
	if created.ID <= 0 {
		t.Errorf("created id = %d, want a fresh positive identifier", created.ID)
	}

	// Persisted through the real store.
	list, err := store.List()
	if err != nil {
		t.Fatalf("store.List: %v", err)
	}
	if len(list) != 1 || list[0] != created {
		t.Errorf("store holds %+v, want exactly the created %+v", list, created)
	}
}

// The POST contract states 400 `{"error": "invalid request"}` for malformed
// JSON (transport error, distinct from the 422 rule refusals).
func TestPostTodosMalformedJSONIs400(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openStore(t), openBoardStore(t)))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/todos", "application/json", strings.NewReader(`{"title": `))
	if err != nil {
		t.Fatalf("POST /todos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	assertCreateError(t, body, "invalid request")
}

// Card api/03 — Create over length limit.
// When  a POST /todos with a title longer than 500 characters arrives
// Then  the response is 422 stating the 500-character limit
//
//	And no todo is created
func TestPostTodosOverLimit(t *testing.T) {
	store := openStore(t)
	srv := httptest.NewServer(NewHandler(store, openBoardStore(t)))
	defer srv.Close()

	long := strings.Repeat("x", todos.MaxTitleLength+1)
	resp, err := http.Post(srv.URL+"/todos", "application/json",
		strings.NewReader(`{"title": "`+long+`"}`))
	if err != nil {
		t.Fatalf("POST /todos: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("error body is not JSON: %v (body %s)", err, body)
	}
	// The refusal states the limit: the stated number and the word "limit".
	if msg := got["error"]; !strings.Contains(msg, strconv.Itoa(todos.MaxTitleLength)) ||
		!strings.Contains(msg, "limit") {
		t.Errorf("error = %q, want it to state the %d-character limit",
			msg, todos.MaxTitleLength)
	}

	if list, err := store.List(); err != nil {
		t.Fatalf("store.List: %v", err)
	} else if len(list) != 0 {
		t.Errorf("over-limit request created todos: %+v", list)
	}
}

func assertCreateError(t *testing.T, body []byte, want string) {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("error body is not JSON: %v (body %s)", err, body)
	}
	if got["error"] != want {
		t.Errorf("error = %q, want %q (body %s)", got["error"], want, body)
	}
}

// Card api/02 — Create with blank title.
// When  a POST /todos with empty or whitespace-only title arrives
// Then  the response is 422 with `{ "error": "title is required" }`
//
//	And no todo is created
func TestPostTodosBlankTitle(t *testing.T) {
	store := openStore(t)
	srv := httptest.NewServer(NewHandler(store, openBoardStore(t)))
	defer srv.Close()

	for _, body := range []string{
		`{"title": ""}`,       // empty
		`{"title": " \t\n "}`, // whitespace-only
		`{}`,                  // missing — the contract states the same refusal
	} {
		resp, err := http.Post(srv.URL+"/todos", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST /todos (%s): %v", body, err)
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d for body %s, want 422", resp.StatusCode, body)
		}
		assertCreateError(t, respBody, "title is required")
	}

	// And no todo is created.
	if list, err := store.List(); err != nil {
		t.Fatalf("store.List: %v", err)
	} else if len(list) != 0 {
		t.Errorf("blank-title requests created todos: %+v", list)
	}
}
