package ui

import (
	"net/http"
	"strings"
	"testing"
)

// Card ui/02 — Empty list is stated, not blank.
// Given the store contains no todos
// When  the user opens the page
// Then  the page shows a distinct "no todos" state with the create control
//
//	ready
func TestEmptyListIsStatedNotBlank(t *testing.T) {
	api := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	status, page := getPage(t, uiServer(t, api.URL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	if !strings.Contains(page, "No todos") {
		t.Errorf("empty list is not stated:\n%s", page)
	}
	// Distinct: no list rows, and not the load-failure state.
	if strings.Contains(page, `id="todo-list"`) {
		t.Errorf("empty page renders a todo list container:\n%s", page)
	}
	if strings.Contains(page, `id="load-error"`) {
		t.Errorf("empty page masquerades as the failure state:\n%s", page)
	}
	// Create control ready.
	if !strings.Contains(page, `name="title"`) || !strings.Contains(page, ">Add<") {
		t.Errorf("create control not ready on the empty page:\n%s", page)
	}
}
