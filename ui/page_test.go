package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// fakeAPI serves a canned GET /todos over real HTTP — the api contract stand-in
// for these tests. The ui module talks to it only via its base URL.
func fakeAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func uiServer(t *testing.T, apiBase string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(NewHandler(apiBase))
	t.Cleanup(srv.Close)
	return srv
}

func getPage(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, buf.String()
}

// Card ui/01 — Page shows the list truthfully.
// Given the store contains todos with mixed done states
// When  the user opens the page
// Then  every todo renders as a row in creation order, oldest first
//
//	And each row shows its text and its done state readably
//	And every row offers done-toggle and delete controls
//	And only not-done rows offer an edit control
func TestPageShowsTheListTruthfully(t *testing.T) {
	itemsJSON := `[
		{"id": 3, "title": "read spec", "done": true},
		{"id": 5, "title": "buy milk", "done": false},
		{"id": 9, "title": "write code", "done": false}
	]`
	api := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/todos" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(itemsJSON))
	})
	uiSrv := uiServer(t, api.URL)

	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	// Rows in creation order (the contract order = ascending id = oldest first).
	iOldest := strings.Index(page, "read spec")
	iMiddle := strings.Index(page, "buy milk")
	iNewest := strings.Index(page, "write code")
	if iOldest < 0 || iMiddle < 0 || iNewest < 0 {
		t.Fatalf("page missing todo texts:\n%s", page)
	}
	if !(iOldest < iMiddle && iMiddle < iNewest) {
		t.Errorf("rows not in creation order (oldest first):\n%s", page)
	}

	// One row per todo.
	rows := regexp.MustCompile(`(?s)<li[^>]*id="todo-\d+"`).FindAllString(page, -1)
	if len(rows) != 3 {
		t.Fatalf("found %d rows, want 3:\n%s", len(rows), page)
	}

	rowOf := func(id string) string {
		m := regexp.MustCompile(
			`(?s)<li[^>]*id="todo-` + regexp.QuoteMeta(id) + `"[^>]*>(.*?)</li>`,
		).FindStringSubmatch(page)
		if m == nil {
			t.Fatalf("no row for id %s:\n%s", id, page)
		}
		return m[1]
	}

	for _, id := range []string{"3", "5", "9"} {
		row := rowOf(id)
		// Done state readable: a checkbox reflects it.
		if !strings.Contains(row, `type="checkbox"`) {
			t.Errorf("row %s has no done-toggle checkbox:\n%s", id, row)
		}
		// Delete control on every row.
		if !strings.Contains(row, "Delete") {
			t.Errorf("row %s has no delete control:\n%s", id, row)
		}
	}
	if !strings.Contains(rowOf("3"), `checked`) {
		t.Errorf("done row 3 checkbox does not show done state:\n%s", rowOf("3"))
	}
	for _, id := range []string{"5", "9"} {
		if strings.Contains(rowOf(id), "checked") {
			t.Errorf("not-done row %s checkbox shows done state:\n%s", id, rowOf(id))
		}
	}

	// Edit control ONLY on not-done rows.
	if strings.Contains(rowOf("3"), "Edit") {
		t.Errorf("done row 3 offers an edit control:\n%s", rowOf("3"))
	}
	for _, id := range []string{"5", "9"} {
		if !strings.Contains(rowOf(id), "Edit") {
			t.Errorf("not-done row %s offers no edit control:\n%s", id, rowOf(id))
		}
	}

	// Create input + button on the page.
	if !strings.Contains(page, `<input`) || !strings.Contains(page, `name="title"`) {
		t.Errorf("page has no create input:\n%s", page)
	}
	if !strings.Contains(page, ">Add<") {
		t.Errorf("page has no create button:\n%s", page)
	}
}

// htmx is vendored and served by this module (ADR-001).
func TestServesVendoredHTMX(t *testing.T) {
	uiSrv := uiServer(t, "http://127.0.0.1:1") // api base unused here
	resp, err := http.Get(uiSrv.URL + "/static/htmx.min.js")
	if err != nil {
		t.Fatalf("GET /static/htmx.min.js: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", ct)
	}
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(buf.String(), "htmx") {
		t.Error("served asset is not the htmx library")
	}
}
