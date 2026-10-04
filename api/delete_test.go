package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"todo/todos"
)

// Card api/10 — Delete.
// When  a DELETE /todos/{id} arrives for an existing todo
// Then  the response is 204 with no body
//
//	And a later GET does not include it
func TestDeleteExistingReturns204AndVanishesFromList(t *testing.T) {
	store := openStore(t)
	keep, err := store.Create("keep me")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	gone, err := store.Create("delete me")
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/todos/"+strconv.FormatInt(gone.ID, 10), nil)
	if err != nil {
		t.Fatalf("build DELETE request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /todos/%d: %v", gone.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("204 must carry no body, got %q", body)
	}

	getResp, err := http.Get(srv.URL + "/todos")
	if err != nil {
		t.Fatalf("GET /todos after delete: %v", err)
	}
	defer getResp.Body.Close()
	var list []todos.Todo
	if err := json.NewDecoder(getResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0] != keep {
		t.Fatalf("list after delete = %+v, want exactly [%+v]", list, keep)
	}
}

// Card api/11 — Delete missing todo.
// Given no todo exists with identifier X
// When  a DELETE /todos/X arrives
// Then  the response is 404 with { "error": "no such todo" }
func TestDeleteMissingTodoReturns404(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openStore(t)))
	defer srv.Close()

	const missing = 987654321
	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/todos/"+strconv.FormatInt(missing, 10), nil)
	if err != nil {
		t.Fatalf("build DELETE request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /todos/%d: %v", missing, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("DELETE status = %d, want 404", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("body is not the contract's JSON error object: %v", err)
	}
	if body["error"] != "no such todo" {
		t.Errorf(`body = %v, want {"error":"no such todo"}`, body)
	}
}

// A non-numeric id is unparseable input, not an invalid-but-parseable value:
// the contract's DELETE section states 400 with {"error":"invalid request"}.
func TestDeleteNonNumericIdReturns400(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openStore(t)))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/todos/not-a-number", nil)
	if err != nil {
		t.Fatalf("build DELETE request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE /todos/not-a-number: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("DELETE status = %d, want 400", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("body is not the contract's JSON error object: %v", err)
	}
	if body["error"] != "invalid request" {
		t.Errorf(`body = %v, want {"error":"invalid request"}`, body)
	}
}
