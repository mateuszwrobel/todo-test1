package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// toggleAPI is the api contract stand-in for the toggle tests: it serves
// GET /todos and PATCH /todos/{id} over real HTTP against an in-memory list,
// recording the JSON bodies it was sent so the tests can pin that the ui
// performs the operation THROUGH the contract.
type toggleAPI struct {
	mu      sync.Mutex
	todos   []todo
	patches []string
}

func (f *toggleAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/todos":
		list := f.todos
		if list == nil {
			list = []todo{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/todos/"):
		var id int64
		if _, err := fmt.Sscan(strings.TrimPrefix(r.URL.Path, "/todos/"), &id); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.patches = append(f.patches, string(body))
		var fields struct {
			Done *bool `json:"done"`
		}
		if err := json.Unmarshal(body, &fields); err != nil || fields.Done == nil {
			http.Error(w, `{"error":"at least one field is required"}`, http.StatusUnprocessableEntity)
			return
		}
		for i := range f.todos {
			if f.todos[i].ID == id {
				f.todos[i].Done = *fields.Done
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(f.todos[i])
				return
			}
		}
		http.Error(w, `{"error":"no such todo"}`, http.StatusNotFound)
	default:
		http.NotFound(w, r)
	}
}

func (f *toggleAPI) patchLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.patches...)
}

// Card ui/08 — Toggle marks done and reopens.
// Given the page shows a not-done todo
// When  the user activates its done toggle
// Then  the row shows the todo as done without a full reload
//
//	(the swap is an htmx fragment response, not the full page)
//
//	And activating the same toggle again shows it not-done
//	And the row's text and position never change from toggling
func TestToggleMarksDoneAndReopens(t *testing.T) {
	fapi := &toggleAPI{todos: []todo{
		{ID: 3, Title: "first task", Done: false},
		{ID: 7, Title: "second task", Done: false},
	}}
	api := fakeAPI(t, fapi.ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	// The checkbox of the not-done row drives a PATCH to the ui's fragment
	// endpoint, targeting the list swap (no full reload).
	row := findRow(t, page, "3")
	if !strings.Contains(row, `hx-patch="/ui/todos/3"`) {
		t.Errorf("row 3 checkbox does not PATCH the fragment endpoint:\n%s", row)
	}
	for _, want := range []string{`hx-target="#todos-area"`, `hx-swap="innerHTML"`} {
		if !strings.Contains(row, want) {
			t.Errorf("row 3 toggle missing swap wiring %s:\n%s", want, row)
		}
	}
	// The not-done row's toggle carries the done value it must send.
	if !strings.Contains(row, `"done": true`) {
		t.Errorf("not-done row 3 toggle does not carry {\"done\": true}:\n%s", row)
	}

	// Simulate the htmx request the checkbox makes: PATCH with the new done
	// value. The response is a list fragment in which the row shows done —
	// the same list position, the same text.
	fragStatus, frag := patchFragment(t, uiSrv.URL+"/ui/todos/3", "true")
	if fragStatus != http.StatusOK {
		t.Fatalf("PATCH /ui/todos/3 status = %d, want 200 (body %s)", fragStatus, frag)
	}
	if strings.Contains(frag, "<html") || strings.Contains(frag, "<!doctype") {
		t.Fatalf("toggle response is a full page, want a fragment:\n%s", frag)
	}
	assertRowState(t, frag, "3", "done", "first task")
	assertRowState(t, frag, "7", "not-done", "second task")
	if !strings.Contains(findRow(t, frag, "3"), "checked") {
		t.Errorf("done row 3 checkbox does not show done:\n%s", findRow(t, frag, "3"))
	}
	if !strings.Contains(findRow(t, frag, "3"), `"done": false`) {
		t.Errorf("done row 3 toggle does not carry {\"done\": false}:\n%s", findRow(t, frag, "3"))
	}
	assertRowOrder(t, frag, "3", "7")

	// The operation went through the api contract as a JSON done field.
	if got := fapi.patchLog(); len(got) != 1 || !strings.Contains(got[0], `"done":true`) {
		t.Errorf("api contract saw patches %v, want one carrying {\"done\": true}", got)
	}

	// Reopen: the same control again → not-done, text and position intact.
	fragStatus, frag = patchFragment(t, uiSrv.URL+"/ui/todos/3", "false")
	if fragStatus != http.StatusOK {
		t.Fatalf("reopen PATCH status = %d, want 200 (body %s)", fragStatus, frag)
	}
	assertRowState(t, frag, "3", "not-done", "first task")
	if strings.Contains(findRow(t, frag, "3"), "checked") {
		t.Errorf("reopened row 3 still shows done:\n%s", findRow(t, frag, "3"))
	}
	if !strings.Contains(findRow(t, frag, "3"), `"done": true`) {
		t.Errorf("reopened row 3 toggle does not carry {\"done\": true}:\n%s", findRow(t, frag, "3"))
	}
	assertRowOrder(t, frag, "3", "7")
}

func findRow(t *testing.T, html, id string) string {
	t.Helper()
	m := regexp.MustCompile(
		`(?s)<li[^>]*id="todo-` + regexp.QuoteMeta(id) + `"[^>]*>.*?</li>`,
	).FindString(html)
	if m == "" {
		t.Fatalf("no row for id %s:\n%s", id, html)
	}
	return m
}

func assertRowState(t *testing.T, html, id, wantState, wantTitle string) {
	t.Helper()
	row := findRow(t, html, id)
	if !strings.Contains(row, `data-state="`+wantState+`"`) {
		t.Errorf("row %s state, want %s:\n%s", id, wantState, row)
	}
	if !strings.Contains(row, wantTitle) {
		t.Errorf("row %s lost its text %q:\n%s", id, wantTitle, row)
	}
}

func assertRowOrder(t *testing.T, html string, ids ...string) {
	t.Helper()
	seen := regexp.MustCompile(`id="todo-(\d+)"`).FindAllStringSubmatch(html, -1)
	var got []string
	for _, m := range seen {
		got = append(got, m[1])
	}
	if strings.Join(got, ",") != strings.Join(ids, ",") {
		t.Errorf("row order = %v, want %v", got, ids)
	}
}

func patchFragment(t *testing.T, url, done string) (int, string) {
	t.Helper()
	body := strings.NewReader("done=" + done)
	req, err := http.NewRequest(http.MethodPatch, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(b)
}
