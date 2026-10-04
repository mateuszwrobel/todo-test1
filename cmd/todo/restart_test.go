package main

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"

	"todo/todos"
)

// Card server/03 — Restart resumes state.
// Given todos exist in the data file
// When  the command is stopped and started again with the same path
// Then  the page and the JSON contract report the same todos with the same
//
//	states
//
// A real process is started, driven over HTTP, stopped, and started again on
// the same data file. Note: POST /todos is not mounted yet (W2 create is a
// pending replay), so mid-run state changes here go through the toggle PATCH
// over already-seeded todos — the same stand-in the W3 e2e uses.
func TestRestartResumesState(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "restart.db")
	store, err := todos.Open(dbPath)
	if err != nil {
		t.Fatalf("seed Open: %v", err)
	}
	want := []todos.Todo{
		{ID: 1, Title: "write report", Done: false},
		{ID: 2, Title: "water plants", Done: true},
		{ID: 3, Title: "buy milk", Done: false},
	}
	for _, td := range want {
		created, err := store.Create(td.Title)
		if err != nil {
			t.Fatalf("seed Create %q: %v", td.Title, err)
		}
		want[created.ID-1].ID = created.ID
		if td.Done {
			done := true
			if _, err := store.Change(created.ID, todos.ChangeFields{Done: &done}); err != nil {
				t.Fatalf("seed Change %d: %v", created.ID, err)
			}
			want[created.ID-1].Done = true
		}
	}
	store.Close()

	addr := freeAddr(t)

	// Run 1: drive a toggle through the live HTTP surface, read the state.
	srv := startServer(t, addr, dbPath)
	req, err := http.NewRequest(http.MethodPatch, "http://"+addr+"/todos/1", strings.NewReader(`{"done":true}`))
	if err != nil {
		t.Fatalf("build PATCH: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /todos/1: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH status = %d (%s), want 200", resp.StatusCode, respBody)
	}
	want[0].Done = true

	before := getTodos(t, addr)
	if !todosEqual(before, want) {
		t.Fatalf("after toggle GET /todos = %+v, want %+v", before, want)
	}

	// Stop the command (SIGTERM), then start it again on the same path.
	// The exit status itself is server/04's contract — here the restart story.
	if err := srv.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	if err := srv.Wait(); err != nil {
		t.Logf("first run exited: %v", err)
	}

	startServer(t, addr, dbPath)

	// The JSON contract reports the same todos with the same states.
	after := getTodos(t, addr)
	if !todosEqual(after, before) {
		t.Fatalf("JSON contract changed across restart:\nbefore %+v\n after %+v", before, after)
	}

	// The page reports the same todos with the same states too.
	resp, err = http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET / after restart: %v", err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	rowRe := regexp.MustCompile(`(?s)<li id="todo-\d+" data-state="(done|not-done)">.*?<span class="title">([^<]*)</span>`)
	rows := rowRe.FindAllStringSubmatch(string(page), -1)
	if len(rows) != len(before) {
		t.Fatalf("page shows %d rows, want %d:\n%s", len(rows), len(before), page)
	}
	for i, td := range before {
		state := "not-done"
		if td.Done {
			state = "done"
		}
		if rows[i][1] != state {
			t.Errorf("page row %d state = %q, want %q", i, rows[i][1], state)
		}
		if !strings.Contains(rows[i][2], td.Title) {
			t.Errorf("page row %d title = %q, want %q", i, rows[i][2], td.Title)
		}
	}
}

func getTodos(t *testing.T, addr string) []todos.Todo {
	t.Helper()
	resp, err := http.Get("http://" + addr + "/todos")
	if err != nil {
		t.Fatalf("GET /todos: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /todos status = %d, want 200", resp.StatusCode)
	}
	var got []todos.Todo
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("GET /todos body %s: %v", body, err)
	}
	return got
}

func todosEqual(a, b []todos.Todo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
