package ui

import (
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

func findAllRowIDs(page string) []string {
	var ids []string
	for _, m := range regexp.MustCompile(`<li[^>]*id="todo-(\d+)"`).FindAllStringSubmatch(page, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// Card ui/15 — Reload matches the server.
// Given any sequence of successful operations has run from the page
// When  the user reloads
// Then  the rendered list equals a fresh read of the server state
//
// (W1 has no page write operations yet; "operations ran" is represented by
// the api's state changing between two page loads — the reload clause is
// what this card tests: every GET renders from a fresh read, nothing is
// cached in the ui module.)
func TestReloadIsFullRerenderFromFreshRead(t *testing.T) {
	var generation atomic.Int64
	lists := map[int64]string{
		1: `[{"id": 1, "title": "before", "done": false}]`,
		2: `[{"id": 1, "title": "before", "done": true},
		     {"id": 2, "title": "after", "done": false}]`,
	}
	api := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(lists[generation.Load()]))
	})
	srv := uiServer(t, api.URL)

	generation.Store(1)
	_, first := getPage(t, srv.URL+"/")
	if !strings.Contains(first, "before") || strings.Contains(first, "after") {
		t.Fatalf("first render does not match generation 1:\n%s", first)
	}

	// Server state moves on (operations ran); the user reloads.
	generation.Store(2)
	_, reloaded := getPage(t, srv.URL+"/")
	if !strings.Contains(reloaded, "before") || !strings.Contains(reloaded, "after") {
		t.Fatalf("reload does not show the fresh server state:\n%s", reloaded)
	}
	// Rendered list equals the fresh read exactly: same rows, no strays.
	rows := findAllRowIDs(reloaded)
	if len(rows) != 2 || rows[0] != "1" || rows[1] != "2" {
		t.Errorf("reload rows = %v, want exactly [1 2]:\n%s", rows, reloaded)
	}
	// And the done state of the fresh read is rendered, not a stale one.
	if !strings.Contains(reloaded, `id="todo-1" data-state="done"`) {
		t.Errorf("reload kept a stale done state for todo 1:\n%s", reloaded)
	}
}
