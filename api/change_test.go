package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todo/todos"
)

// patchTodo sends a PATCH /todos/{id} with the given raw JSON body.
func patchTodo(t *testing.T, url string, id int64, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/todos/%d", url, id), strings.NewReader(body))
	if err != nil {
		t.Fatalf("new PATCH request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /todos/%d: %v", id, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp, string(b)
}

// Card api/06 — Change done state in either direction.
// Given a todo exists (title "Buy milk", not done)
// When  a PATCH /todos/{id} with {"done": true} or {"done": false} arrives
// Then  the response is 200 with the updated todo JSON and the title
//
//	unchanged — and the done state persists in the store
func TestPatchDoneStateEitherDirection(t *testing.T) {
	store := openStore(t)
	created, err := store.Create("Buy milk")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	// Mark done.
	resp, body := patchTodo(t, srv.URL, created.ID, `{"done": true}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH {done:true} status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	var got todos.Todo
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("PATCH response is not todo JSON: %v (body %s)", err, body)
	}
	if got != (todos.Todo{ID: created.ID, Title: "Buy milk", Done: true}) {
		t.Errorf("PATCH {done:true} = %+v, want {ID:%d Title:Buy milk Done:true}", got, created.ID)
	}

	// Reopen.
	resp, body = patchTodo(t, srv.URL, created.ID, `{"done": false}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH {done:false} status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	got = todos.Todo{}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("PATCH response is not todo JSON: %v (body %s)", err, body)
	}
	if got != (todos.Todo{ID: created.ID, Title: "Buy milk", Done: false}) {
		t.Errorf("PATCH {done:false} = %+v, want {ID:%d Title:Buy milk Done:false}", got, created.ID)
	}

	// The last state is what the durable store now reports.
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0] != (todos.Todo{ID: created.ID, Title: "Buy milk", Done: false}) {
		t.Fatalf("store after toggles = %+v, want the todo not-done with title unchanged", list)
	}
}

// Card api/08 — Change missing todo.
// Given no todo exists with identifier X
// When  a PATCH /todos/X arrives
// Then  the response is 404 with { "error": "no such todo" }
func TestPatchMissingTodoIs404(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openStore(t)))
	defer srv.Close()

	resp, body := patchTodo(t, srv.URL, 999, `{"done": true}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("PATCH missing id status = %d, want 404 (body %s)", resp.StatusCode, body)
	}
	var errBody map[string]string
	if err := json.Unmarshal([]byte(body), &errBody); err != nil {
		t.Fatalf("error body is not JSON: %v (body %s)", err, body)
	}
	if errBody["error"] != "no such todo" {
		t.Errorf("error body = %s, want {\"error\": \"no such todo\"}", body)
	}
}
