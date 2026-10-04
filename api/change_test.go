package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// Card api/09 — Change with empty body (defensive path).
// When  a PATCH /todos/{id} with an empty JSON object arrives — or a body
//
//	with no recognizable JSON fields at all —
//
// Then  the response is 422 stating that at least one field is required
func TestPatchEmptyBodyIs422(t *testing.T) {
	store := openStore(t)
	created, err := store.Create("unchanged")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	for _, body := range []string{`{}`, ``} {
		resp, respBody := patchTodo(t, srv.URL, created.ID, body)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("PATCH body %q status = %d, want 422 (resp %s)", body, resp.StatusCode, respBody)
		}
		var errBody map[string]string
		if err := json.Unmarshal([]byte(respBody), &errBody); err != nil {
			t.Fatalf("body %q: error body is not JSON: %v (resp %s)", body, err, respBody)
		}
		if !strings.Contains(errBody["error"], "at least one field") {
			t.Errorf("body %q: error = %q, want it to state that at least one field is required", body, errBody["error"])
		}
	}

	// No state change from either rejected request.
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0] != (todos.Todo{ID: created.ID, Title: "unchanged", Done: false}) {
		t.Fatalf("state changed on empty-body patches: %+v", list)
	}
}

// Card api/05 — Change text of a not-done todo.
// When  a PATCH /todos/{id} with {"title": "new text"} arrives for an
//
//	existing not-done todo —
//
// Then  the response is 200 with the updated todo JSON and done state
//
//	unchanged, and the change persists.
//	An empty/whitespace-only title is the contract's 422 "title is
//	required" — the rule arrives through the store's todos/02
//	validation, never re-implemented here.
//	An over-the-limit title is 422 stating the limit, stated from
//	todos.MaxTitleLength — the constant's single owner.
func TestPatchTitleOnNotDoneTodo(t *testing.T) {
	store := openStore(t)
	created, err := store.Create("Walk the dog")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	// The happy direction: 200 with the updated todo, done still false.
	resp, body := patchTodo(t, srv.URL, created.ID, `{"title": "Walk the dog in the park"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH {title} status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	var got todos.Todo
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("PATCH response is not todo JSON: %v (body %s)", err, body)
	}
	if got != (todos.Todo{ID: created.ID, Title: "Walk the dog in the park", Done: false}) {
		t.Errorf("PATCH {title} = %+v, want {ID:%d Title:Walk the dog in the park Done:false}", got, created.ID)
	}
	// It persists.
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0] != got {
		t.Fatalf("PATCH title did not persist: %+v", list)
	}

	// Empty and whitespace-only titles arrive as the store's invalid-text
	// outcome, stated as the contract's 422.
	for _, raw := range []string{`{"title": ""}`, `{"title": "   "}`} {
		resp, body = patchTodo(t, srv.URL, created.ID, raw)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("PATCH body %s status = %d, want 422 (body %s)", raw, resp.StatusCode, body)
		}
		var errBody map[string]string
		if err := json.Unmarshal([]byte(body), &errBody); err != nil {
			t.Fatalf("body %s: error body is not JSON: %v (resp %s)", raw, err, body)
		}
		if errBody["error"] != "title is required" {
			t.Errorf("body %s: error = %q, want %q", raw, errBody["error"], "title is required")
		}
	}

	// Over the limit: 422 stating the limit number — owned by one constant.
	long := strings.Repeat("x", todos.MaxTitleLength+1)
	resp, body = patchTodo(t, srv.URL, created.ID, `{"title": "`+long+`"}`)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("over-limit PATCH status = %d, want 422 (body %s)", resp.StatusCode, body)
	}
	var errBody map[string]string
	if err := json.Unmarshal([]byte(body), &errBody); err != nil {
		t.Fatalf("over-limit: error body is not JSON: %v (resp %s)", err, body)
	}
	if msg := errBody["error"]; !strings.Contains(msg, strconv.Itoa(todos.MaxTitleLength)) ||
		!strings.Contains(msg, "limit") {
		t.Errorf("over-limit error = %q, want it to state the %d-character limit",
			msg, todos.MaxTitleLength)
	}

	// Every rejection above left the todo exactly at the happy-path result.
	list, err = store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0] != (todos.Todo{ID: created.ID, Title: "Walk the dog in the park", Done: false}) {
		t.Fatalf("rejected patches changed the todo: %+v", list)
	}
}

// The stored form is canonical — the trimmed title, the same rule Create
// applies (todos/02): one text the rules speak about.
func TestPatchTitleStoresTrimmedForm(t *testing.T) {
	store := openStore(t)
	created, err := store.Create("padded")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	resp, body := patchTodo(t, srv.URL, created.ID, `{"title": "   Buy milk\t"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH padded title status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	var got todos.Todo
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("response is not todo JSON: %v (body %s)", err, body)
	}
	if got.Title != "Buy milk" {
		t.Errorf("title = %q, want the trimmed form %q", got.Title, "Buy milk")
	}
}

// Card api/07 — Title edit on done todo refused.
// When  a PATCH /todos/{id} carrying a title arrives for a done todo
// Then  the response is 422 with {"error": "cannot edit a done todo"}
//
//	And the todo is unchanged.
//	The done-only reopen path is unaffected — reopening still answers
//	200 and unlocks the title direction again (todos/07's story on the
//	contract).
func TestPatchTitleOnDoneTodoRefused(t *testing.T) {
	store := openStore(t)
	created, err := store.Create("Buy milk")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	done := true
	if _, err := store.Change(created.ID, todos.ChangeFields{Done: &done}); err != nil {
		t.Fatalf("seed Change(done=true): %v", err)
	}
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	// Title carried — even alongside done — the frozen refusal answers.
	for _, body := range []string{`{"title": "Buy oat milk"}`, `{"title": "Buy oat milk", "done": true}`} {
		resp, respBody := patchTodo(t, srv.URL, created.ID, body)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("PATCH %s status = %d, want 422 (body %s)", body, resp.StatusCode, respBody)
		}
		var errBody map[string]string
		if err := json.Unmarshal([]byte(respBody), &errBody); err != nil {
			t.Fatalf("PATCH %s: error body is not JSON: %v (resp %s)", body, err, respBody)
		}
		if errBody["error"] != "cannot edit a done todo" {
			t.Errorf("PATCH %s: error = %q, want %q", body, errBody["error"], "cannot edit a done todo")
		}
	}

	// Unchanged: still the original text, still done.
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0] != (todos.Todo{ID: created.ID, Title: "Buy milk", Done: true}) {
		t.Fatalf("frozen patches changed the todo: %+v", list)
	}

	// Reopen unaffected: done-only answers 200, and the title direction
	// then succeeds — the unlock path over the contract.
	resp, respBody := patchTodo(t, srv.URL, created.ID, `{"done": false}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reopen status = %d, want 200 (body %s)", resp.StatusCode, respBody)
	}
	resp, respBody = patchTodo(t, srv.URL, created.ID, `{"title": "Buy oat milk"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("title after reopen status = %d, want 200 (body %s)", resp.StatusCode, respBody)
	}
	list, err = store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0] != (todos.Todo{ID: created.ID, Title: "Buy oat milk", Done: false}) {
		t.Fatalf("reopen-then-edit did not persist: %+v", list)
	}
}
