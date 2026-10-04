package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Card ui/03 — Load failure is stated, not faked.
// Given the list cannot be read (server not reachable)
// When  the user opens or reloads the page
// Then  the page shows that the todos could not be loaded
//
//	And it never shows an empty or stale list as if it were the truth
func TestLoadFailureApiUnreachable(t *testing.T) {
	// A server that was up and is now closed: nothing listens there.
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()

	status, page := getPage(t, uiServer(t, deadURL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200 (page itself serves, stating failure)", status)
	}
	assertFailureStated(t, page)
}

func TestLoadFailureNonOKStatus(t *testing.T) {
	api := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	})
	status, page := getPage(t, uiServer(t, api.URL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200 (page itself serves, stating failure)", status)
	}
	assertFailureStated(t, page)
}

func assertFailureStated(t *testing.T, page string) {
	t.Helper()
	if !strings.Contains(page, "Could not load todos") {
		t.Errorf("page does not state that the todos could not be loaded:\n%s", page)
	}
	// Never an empty or stale-list masquerade.
	if strings.Contains(page, "No todos") {
		t.Errorf("failure page fakes the empty state:\n%s", page)
	}
	if strings.Contains(page, `id="todo-list"`) {
		t.Errorf("failure page fakes a list:\n%s", page)
	}
}
