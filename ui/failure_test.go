package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Card ui/03 — Load failure renders as stated failure.
// Given GET /board fails (server unreachable or non-200)
// When  the user opens or reloads the page
// Then  the page states that the board could not be loaded
//
//	And it does not render empty columns as if they were the truth
//	And a reload that succeeds renders the board normally
func TestLoadFailureRendersAsStatedFailure(t *testing.T) {
	t.Run("unreachable", func(t *testing.T) {
		// A server that was up and is now closed: nothing listens there.
		dead := httptest.NewServer(http.NotFoundHandler())
		deadURL := dead.URL
		dead.Close()
		status, page := getPage(t, uiServer(t, deadURL).URL+"/")
		if status != http.StatusOK {
			t.Fatalf("GET / status = %d, want 200 (page itself serves, stating failure)", status)
		}
		assertBoardFailureStated(t, page)
	})

	t.Run("non-200", func(t *testing.T) {
		api := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
		})
		status, page := getPage(t, uiServer(t, api.URL).URL+"/")
		if status != http.StatusOK {
			t.Fatalf("GET / status = %d, want 200 (page itself serves, stating failure)", status)
		}
		assertBoardFailureStated(t, page)
	})

	// The reload arm: a GET that succeeds replaces the failure surface
	// fully — every failed read re-issues through GET semantics, so the
	// next successful read renders the board normally with nothing left
	// of the failure statement.
	failing := &atomic.Bool{}
	failing.Store(true)
	api := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(boardJSON))
	})
	srv := uiServer(t, api.URL)

	_, page := getPage(t, srv.URL+"/")
	if !strings.Contains(page, "Could not load board") {
		t.Fatalf("expected failure state first:\n%s", page)
	}
	failing.Store(false)
	status, reloaded := getPage(t, srv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("reload status = %d, want 200", status)
	}
	if strings.Contains(reloaded, "Could not load board") || strings.Contains(reloaded, `id="load-error"`) {
		t.Errorf("successful reload did not replace the failure surface:\n%s", reloaded)
	}
	for _, want := range []string{`id="board"`, `id="column-to-do"`, `id="column-in-progress"`,
		`id="column-done"`, "Draft the launch note", "Ship v1.2"} {
		if !strings.Contains(reloaded, want) {
			t.Errorf("successful reload does not render the board normally (missing %s):\n%s", want, reloaded)
		}
	}
}

func assertBoardFailureStated(t *testing.T, page string) {
	t.Helper()
	if !strings.Contains(page, "Could not load board") {
		t.Errorf("page does not state that the board could not be loaded:\n%s", page)
	}
	// Never columns-as-truth: neither a rendered (empty or otherwise)
	// board nor the empty treatment may stand in for a truth the server
	// could not give.
	if strings.Contains(page, `id="board"`) || strings.Contains(page, `<section id="column-`) {
		t.Errorf("failure page renders columns as if they were the truth:\n%s", page)
	}
	if strings.Contains(page, `class="column__empty"`) {
		t.Errorf("failure page fakes the stated empty treatment:\n%s", page)
	}
}
