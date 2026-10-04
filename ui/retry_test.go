package ui

import (
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

func regexpFind(pattern, s string) string {
	return regexp.MustCompile(pattern).FindString(s)
}

// Card ui/04 — Load failure recovers by retry.
// Given the page is in the load-failure state
// When  the user activates retry
// Then  the list is re-read and rendered, or the failure state is restated
func TestLoadFailureOffersRetry(t *testing.T) {
	var failing atomic.Bool
	failing.Store(true)
	api := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id": 1, "title": "recovered", "done": false}]`))
	})
	srv := uiServer(t, api.URL)

	// Given: failure state.
	_, page := getPage(t, srv.URL+"/")
	if !strings.Contains(page, "Could not load todos") {
		t.Fatalf("expected failure state first:\n%s", page)
	}

	// The failure state offers a retry control: a GET to the page, which
	// re-issues the read (recovery lives inside GET semantics).
	retry := regexpFind(`(?s)<a[^>]*id="retry"[^>]*href="/"[^>]*>Retry`, page)
	if retry == "" {
		retry = regexpFind(`(?s)<form[^>]*id="retry"[^>]*action="/"`, page)
	}
	if retry == "" {
		t.Fatalf("failure state offers no retry control re-issuing the read:\n%s", page)
	}

	// While the api is still down, activating retry restates the failure.
	_, again := getPage(t, srv.URL+"/")
	if !strings.Contains(again, "Could not load todos") {
		t.Errorf("retry with api still down did not restate the failure:\n%s", again)
	}

	// With the api healthy again, activating retry renders the list.
	failing.Store(false)
	_, recovered := getPage(t, srv.URL+"/")
	if !strings.Contains(recovered, "recovered") {
		t.Errorf("retry after api recovery did not render the list:\n%s", recovered)
	}
	if strings.Contains(recovered, "Could not load todos") {
		t.Errorf("recovered page still states failure:\n%s", recovered)
	}
}
