package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type fakeTodo struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// deleteStubAPI is a stateful stand-in for the api contract's delete surface:
// GET /todos answers the current list, DELETE /todos/{id} removes the row
// (204) or states 404. State lives in the fake, so the ui module's re-read
// after the operation observes exactly what its DELETE did — the coupling
// stays HTTP-only, as in composition.
func deleteStubAPI(t *testing.T, items []fakeTodo) *httptest.Server {
	t.Helper()
	state := items
	return fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/todos" && r.Method == http.MethodGet {
			list := state
			if list == nil {
				list = []fakeTodo{}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
			return
		}
		if prefix := "/todos/"; strings.HasPrefix(r.URL.Path, prefix) && r.Method == http.MethodDelete {
			id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, prefix), 10, 64)
			if err != nil {
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}
			for i, td := range state {
				if td.ID == id {
					state = append(append([]fakeTodo{}, state[:i]...), state[i+1:]...)
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "no such todo"})
			return
		}
		http.NotFound(w, r)
	})
}

func deleteViaUI(t *testing.T, uiSrvURL string, id int64) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, uiSrvURL+"/ui/todos/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		t.Fatalf("build fragment DELETE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /ui/todos/%d: %v", id, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read fragment body: %v", err)
	}
	return resp.StatusCode, string(body)
}

// assertListFragment checks the swapped-in content is a page-shell-free list
// fragment holding exactly want, in order, with texts and done states intact.
func assertListFragment(t *testing.T, fragment string, want []fakeTodo) {
	t.Helper()
	if strings.Contains(fragment, "<!doctype") {
		t.Fatalf("fragment is a full page, breaking the no-reload swap: %s", fragment)
	}
	for _, td := range want {
		liOpen := `<li id="todo-` + strconv.FormatInt(td.ID, 10) + `" data-state="`
		if td.Done {
			liOpen += `done`
		} else {
			liOpen += `not-done`
		}
		liOpen += `">`
		if !strings.Contains(fragment, liOpen) {
			t.Fatalf("row todo-%d missing (with its done state %q): %s",
				td.ID, strings.TrimSuffix(strings.TrimPrefix(liOpen, `<li id="todo-`+strconv.FormatInt(td.ID, 10)+`" data-state="`), `">`), fragment)
		}
		if !strings.Contains(fragment, td.Title) {
			t.Fatalf("fragment lost the text of todo-%d: %s", td.ID, fragment)
		}
	}
	if got := strings.Count(fragment, "<li "); got != len(want) {
		t.Fatalf("fragment carries %d rows, want %d: %s", got, len(want), fragment)
	}
}

// Card ui/13 — Delete drops one row.
// Given the page shows several todos
// When  the user activates delete on one row
// Then  the swapped-in content omits exactly that todo
//
//	And every other row keeps its text, done state, and relative order
func TestDeleteFragmentOmitsExactlyOneRow(t *testing.T) {
	api := deleteStubAPI(t, []fakeTodo{
		{ID: 3, Title: "alpha", Done: false},
		{ID: 5, Title: "beta", Done: true},
		{ID: 9, Title: "gamma", Done: false},
	})
	uiSrv := uiServer(t, api.URL)

	status, fragment := deleteViaUI(t, uiSrv.URL, 5)
	if status != http.StatusOK {
		t.Fatalf("fragment status = %d, want 200", status)
	}
	if strings.Contains(fragment, `id="todo-5"`) {
		t.Fatalf("swapped-in content still shows the deleted todo: %s", fragment)
	}
	assertListFragment(t, fragment, []fakeTodo{
		{ID: 3, Title: "alpha", Done: false},
		{ID: 9, Title: "gamma", Done: false},
	})
	if iAlpha, iGamma := strings.Index(fragment, "alpha"), strings.Index(fragment, "gamma"); iAlpha > iGamma {
		t.Fatalf("relative order broken (alpha after gamma): %s", fragment)
	}
}

// Card ui/13 (cont.) — deleting the last todo lands the page in the
// "no todos" state.
func TestDeleteLastTodoLandsEmptyState(t *testing.T) {
	api := deleteStubAPI(t, []fakeTodo{{ID: 7, Title: "the last one"}})
	uiSrv := uiServer(t, api.URL)

	status, fragment := deleteViaUI(t, uiSrv.URL, 7)
	if status != http.StatusOK {
		t.Fatalf("fragment status = %d, want 200", status)
	}
	if !strings.Contains(fragment, `id="empty-state"`) {
		t.Fatalf("fragment does not state the empty state: %s", fragment)
	}
	if strings.Contains(fragment, "<ul") {
		t.Fatalf("empty-state fragment still carries a list: %s", fragment)
	}
}

// The rendered row's delete control must carry the htmx wiring that reaches
// the fragment endpoint — that wiring is what makes the removal a swap, not
// a navigation.
func TestRenderedRowsWireDeleteToFragmentEndpoint(t *testing.T) {
	api := deleteStubAPI(t, []fakeTodo{{ID: 5, Title: "wired", Done: false}})
	uiSrv := uiServer(t, api.URL)

	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	if !strings.Contains(page, `hx-delete="/ui/todos/5"`) {
		t.Fatalf(`row delete control is not wired with hx-delete="/ui/todos/{id}": %s`, page)
	}
	if !strings.Contains(page, `hx-target="#todo-list"`) || !strings.Contains(page, `hx-swap="outerHTML"`) {
		t.Fatalf(`delete control must target the list for an outerHTML swap: %s`, page)
	}
}
