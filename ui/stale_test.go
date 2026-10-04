package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// Card ui/14 — Missing todo states the failure for any operation.
// Given a page stale about a todo that no longer exists
// When  the user toggles, edits, or deletes it
// Then  no change occurs anywhere
//
//	And the page states that the todo does not exist
//	And no row is left looking like the failed operation succeeded
//
// staleAPI is not a separate server: one stateful stand-in for the api
// contract covers all three row operations below — the missing id is simply
// absent, so PATCH and DELETE answer the contract's 404
// {"error":"no such todo"} while GET /todos keeps serving the surviving
// truth the failed page must be re-rendered from.

type fakeAPIStale struct {
	state []fakeTodo
}

func (f *fakeAPIStale) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/todos" && r.Method == http.MethodGet {
		list := f.state
		if list == nil {
			list = []fakeTodo{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
		return
	}
	if prefix := "/todos/"; strings.HasPrefix(r.URL.Path, prefix) {
		id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, prefix), 10, 64)
		if err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		for i, td := range f.state {
			if td.ID == id {
				switch r.Method {
				case http.MethodDelete:
					f.state = append(append([]fakeTodo{}, f.state[:i]...), f.state[i+1:]...)
					w.WriteHeader(http.StatusNoContent)
					return
				case http.MethodPatch:
					var fields map[string]any
					body, _ := io.ReadAll(r.Body)
					_ = json.Unmarshal(body, &fields)
					if done, ok := fields["done"].(bool); ok {
						f.state[i].Done = done
					}
					if title, ok := fields["title"].(string); ok && title != "" {
						f.state[i].Title = title
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(f.state[i])
					return
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no such todo"})
		return
	}
	http.NotFound(w, r)
}

// assertMissingTodoFragment checks the stated failure surface every row
// operation shares: 404 answered as HTML, the banner stating the contract's
// reason exactly once, and the server-truth list (or empty state) under it
// with the missing row gone — never a full page, never a silent body.
func assertMissingTodoFragment(t *testing.T, status int, ctype, fragment string, missingID int64, want []fakeTodo) {
	t.Helper()
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
	if !strings.HasPrefix(ctype, "text/html") {
		t.Fatalf("Content-Type = %q, want HTML so the page can swap the stated failure", ctype)
	}
	if strings.Contains(fragment, "<!doctype") {
		t.Fatalf("failure is a full page, breaking the no-reload swap: %s", fragment)
	}
	if got := strings.Count(fragment, `id="missing-todo-banner"`); got != 1 {
		t.Fatalf("banner rendered %d times, want exactly 1: %s", got, fragment)
	}
	if !strings.Contains(fragment, "no such todo") {
		t.Fatalf("fragment does not state the contract's missing-todo reason: %s", fragment)
	}
	if strings.Contains(fragment, `id="todo-`+strconv.FormatInt(missingID, 10)+`"`) {
		t.Fatalf("fragment still renders the missing todo — a row looks like the operation applied: %s", fragment)
	}
	assertListFragment(t, fragment, want)
}

func requestFragment(t *testing.T, method, urlStr, body string) (int, string, string) {
	t.Helper()
	req, err := http.NewRequest(method, urlStr, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build %s request: %v", method, err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, urlStr, err)
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), buf.String()
}

// Toggle on a missing todo → the stated failure for the toggle operation.
func TestToggleOnMissingTodoStatesFailure(t *testing.T) {
	api := fakeAPI(t, (&fakeAPIStale{state: []fakeTodo{
		{ID: 3, Title: "survivor", Done: false},
	}}).ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	status, ctype, fragment := requestFragment(t, http.MethodPatch, uiSrv.URL+"/ui/todos/9?done=true", "")
	assertMissingTodoFragment(t, status, ctype, fragment, 9,
		[]fakeTodo{{ID: 3, Title: "survivor", Done: false}})
}

// Edit-save on a missing todo → the same stated failure for the edit operation.
func TestEditSaveOnMissingTodoStatesFailure(t *testing.T) {
	api := fakeAPI(t, (&fakeAPIStale{state: []fakeTodo{
		{ID: 3, Title: "survivor", Done: false},
	}}).ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	status, ctype, fragment := requestFragment(t, http.MethodPatch, uiSrv.URL+"/ui/todos/9",
		url.Values{"title": {"edited into the void"}}.Encode())
	assertMissingTodoFragment(t, status, ctype, fragment, 9,
		[]fakeTodo{{ID: 3, Title: "survivor", Done: false}})
}

// Delete of a missing todo → the same stated failure for the delete operation.
func TestDeleteOfMissingTodoStatesFailure(t *testing.T) {
	api := fakeAPI(t, (&fakeAPIStale{state: []fakeTodo{
		{ID: 3, Title: "survivor", Done: false},
	}}).ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	status, ctype, fragment := requestFragment(t, http.MethodDelete, uiSrv.URL+"/ui/todos/9", "")
	assertMissingTodoFragment(t, status, ctype, fragment, 9,
		[]fakeTodo{{ID: 3, Title: "survivor", Done: false}})
}

// The surface stays coherent when nothing survives: the banner states the
// failure above the truthfully-rendered empty state — no fake success row.
func TestMissingTodoFailureOverEmptyState(t *testing.T) {
	api := fakeAPI(t, (&fakeAPIStale{state: nil}).ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	status, ctype, fragment := requestFragment(t, http.MethodDelete, uiSrv.URL+"/ui/todos/9", "")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
	if !strings.HasPrefix(ctype, "text/html") {
		t.Fatalf("Content-Type = %q, want HTML so the page can swap the stated failure", ctype)
	}
	if !strings.Contains(fragment, `id="missing-todo-banner"`) ||
		!strings.Contains(fragment, `id="empty-state"`) {
		t.Fatalf("failure over the empty list must carry banner and empty state: %s", fragment)
	}
	if strings.Contains(fragment, "<ul") {
		t.Fatalf("empty truth still renders a list: %s", fragment)
	}
}

// Success paths render no banner: the stated failure appears only when the
// contract says the todo does not exist.
func TestSuccessfulOperationsRenderNoBanner(t *testing.T) {
	api := fakeAPI(t, (&fakeAPIStale{state: []fakeTodo{
		{ID: 3, Title: "survivor", Done: false},
		{ID: 9, Title: "doomed", Done: false},
	}}).ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	status, _, fragment := requestFragment(t, http.MethodPatch, uiSrv.URL+"/ui/todos/3?done=true", "")
	if status != http.StatusOK || strings.Contains(fragment, `id="missing-todo-banner"`) {
		t.Fatalf("successful toggle carries a failure surface (%d): %s", status, fragment)
	}
	status, _, fragment = requestFragment(t, http.MethodPatch, uiSrv.URL+"/ui/todos/3",
		url.Values{"title": {"renamed"}}.Encode())
	if status != http.StatusOK || strings.Contains(fragment, `id="missing-todo-banner"`) {
		t.Fatalf("successful edit carries a failure surface (%d): %s", status, fragment)
	}
	status, _, fragment = requestFragment(t, http.MethodDelete, uiSrv.URL+"/ui/todos/9", "")
	if status != http.StatusOK || strings.Contains(fragment, `id="missing-todo-banner"`) {
		t.Fatalf("successful delete carries a failure surface (%d): %s", status, fragment)
	}
}

// The missing-todo surface is served by one owner and the page routes HTML
// failures into the stated region: document the swap contract the browser
// depends on (region innerHTML swap, single page-level error router).
func TestPageRoutesRowFailuresToTheStatedRegion(t *testing.T) {
	api := fakeAPI(t, (&fakeAPIStale{state: []fakeTodo{{ID: 3, Title: "wired"}}}).ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	if !strings.Contains(page, `id="todos-area"`) {
		t.Fatalf("page has no stated region for the missing-todo banner: %s", page)
	}
	// All three row controls target the region so success and failure swap
	// into the same surface, replacing it whole (a stale banner can never
	// outlive the next operation's swap).
	row := findRow(t, page, "3")
	if n := strings.Count(row, `hx-target="#todos-area"`); n != 3 {
		t.Fatalf("row carries %d region-target controls, want 3 (toggle, edit, delete):\n%s", n, row)
	}
	if !strings.Contains(page, `getElementById('todos-area')`) {
		t.Fatalf("page does not route failed HTML responses into the region: %s", page)
	}
}
