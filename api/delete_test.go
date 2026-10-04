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
