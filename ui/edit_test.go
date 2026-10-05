package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// editAPI is the api contract stand-in for the edit tests: it serves
// GET /todos and PATCH /todos/{id} over real HTTP against an in-memory
// list, speaking the contract's edit rules — the done-frozen refusal
// (422 "cannot edit a done todo") and the invalid-text refusal
// (422 "title is required") — and recording the JSON bodies it receives
// so tests can pin that ui performs the operation THROUGH the contract.
type editAPI struct {
	mu      sync.Mutex
	todos   []todo
	patches []string
}

func (f *editAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
			Title *string `json:"title"`
			Done  *bool   `json:"done"`
		}
		if err := json.Unmarshal(body, &fields); err != nil || (fields.Title == nil && fields.Done == nil) {
			http.Error(w, `{"error":"at least one field is required"}`, http.StatusUnprocessableEntity)
			return
		}
		for i := range f.todos {
			if f.todos[i].ID != id {
				continue
			}
			// The frozen-text rule: a title for a done todo is refused.
			if fields.Title != nil && f.todos[i].Done {
				http.Error(w, `{"error":"cannot edit a done todo"}`, http.StatusUnprocessableEntity)
				return
			}
			if fields.Title != nil {
				if strings.TrimSpace(*fields.Title) == "" {
					http.Error(w, `{"error":"title is required"}`, http.StatusUnprocessableEntity)
					return
				}
				f.todos[i].Title = strings.TrimSpace(*fields.Title)
			}
			if fields.Done != nil {
				f.todos[i].Done = *fields.Done
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(f.todos[i])
			return
		}
		http.Error(w, `{"error":"no such todo"}`, http.StatusNotFound)
	default:
		http.NotFound(w, r)
	}
}

func (f *editAPI) patchLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.patches...)
}

// patchTitleFragment simulates the edit form's htmx request: a PATCH to
// the ui's fragment endpoint carrying the form-encoded title (present as a
// key even when empty — the browser sends it that way).
func patchTitleFragment(t *testing.T, uiURL string, id int64, title string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/ui/todos/%d", uiURL, id),
		strings.NewReader(url.Values{"title": {title}}.Encode()))
	if err != nil {
		t.Fatalf("new PATCH request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /ui/todos/%d: %v", id, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(b)
}

// editForm extracts the row's edit form markup (empty when the row carries
// none).
func editForm(t *testing.T, html string, id string) string {
	t.Helper()
	row := findRow(t, html, id)
	start := strings.Index(row, `<form class="edit-form"`)
	if start < 0 {
		return ""
	}
	end := strings.Index(row[start:], `</form>`)
	if end < 0 {
		t.Fatalf("row %s edit form is not closed:\n%s", id, row)
	}
	return row[start : start+end+len("</form>")]
}

// Card ui/09 — Inline edit updates in place.
// Given the page shows a not-done todo
// When  the user activates edit on the row, changes the text, and saves
// Then  the row shows the new text without a full reload
//
//	And the todo remains not-done and in the same position.
//	Cancel is client-side only: the edit surface carries a Cancel control
//	that triggers no request, so the original text stays untouched.
func TestInlineEditUpdatesInPlace(t *testing.T) {
	fapi := &editAPI{todos: []todo{
		{ID: 3, Title: "first task", Done: false},
		{ID: 5, Title: "second task", Done: true},
		{ID: 7, Title: "third task", Done: false},
	}}
	api := fakeAPI(t, fapi.ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	// The not-done row carries an edit control AND an edit band: a form
	// PATCHing the fragment endpoint with the row's text prefilled, plus
	// Save and Cancel — Cancel sends no request, so cancelling leaves the
	// original untouched by construction.
	form := editForm(t, page, "3")
	if form == "" {
		t.Fatalf("not-done row 3 carries no edit form:\n%s", findRow(t, page, "3"))
	}
	for _, want := range []string{
		`hx-patch="/ui/todos/3"`, `hx-target="#todos-area"`, `hx-swap="innerHTML"`,
		`name="title" value="first task"`, `>Save</button>`,
	} {
		if !strings.Contains(form, want) {
			t.Errorf("row 3 edit form missing %s:\n%s", want, form)
		}
	}
	if !strings.Contains(form, "Cancel") {
		t.Errorf("row 3 edit form carries no Cancel control:\n%s", form)
	}
	if !strings.Contains(editForm(t, page, "7"), `hx-patch="/ui/todos/7"`) {
		t.Errorf("not-done row 7 carries no edit form:\n%s", findRow(t, page, "7"))
	}

	// Save: the form PATCHes the fragment endpoint with the new text. The
	// answer is a list fragment (not a page → no reload): the row shows the
	// new text, stays not-done, and keeps its position between the others.
	fragStatus, frag := patchTitleFragment(t, uiSrv.URL, 3, "first task, refined")
	if fragStatus != http.StatusOK {
		t.Fatalf("PATCH title status = %d, want 200 (body %s)", fragStatus, frag)
	}
	if strings.Contains(frag, "<html") || strings.Contains(frag, "<!doctype") {
		t.Fatalf("edit response is a full page, want a fragment:\n%s", frag)
	}
	assertRowState(t, frag, "3", "not-done", "first task, refined")
	assertRowState(t, frag, "7", "not-done", "third task")
	assertRowOrder(t, frag, "3", "5", "7")

	// The operation went through the api contract as a JSON title field —
	// and nothing but the title (the done state is not the edit's business).
	if got := fapi.patchLog(); len(got) != 1 ||
		!strings.Contains(got[0], `"title":"first task, refined"`) ||
		strings.Contains(got[0], `"done"`) {
		t.Errorf("api contract saw patches %v, want one carrying only the title", got)
	}

	// The edit control reveals the band: the Edit control stays on the row
	// (W1's rule), and the page's style rules hide the band until the row
	// is in edit mode, so the list reads as text until Edit is activated.
	if !strings.Contains(findRow(t, page, "3"), `class="btn btn--secondary edit"`) {
		t.Errorf("not-done row 3 lost its Edit control:\n%s", findRow(t, page, "3"))
	}
	if !strings.Contains(page, `li.editing .edit-form`) {
		t.Errorf("page carries no edit-band display rule (li.editing .edit-form):\n%s", page)
	}
}

// Card ui/10 — Done rows carry no edit control.
// Given the page freshly loads a list containing a done todo
// Then  that row shows no edit affordance
//
//	And its text can only change after reopening via the toggle.
//
// This pins W1's render rule with a dedicated test — and now the rule
// covers the whole edit band, not just the Edit button.
func TestDoneRowsCarryNoEditControl(t *testing.T) {
	fapi := &editAPI{todos: []todo{
		{ID: 3, Title: "settled task", Done: true},
		{ID: 7, Title: "open task", Done: false},
	}}
	api := fakeAPI(t, fapi.ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	// The done row: its toggle and delete controls are there, no edit
	// control anywhere — no Edit button, no edit form, no band input.
	doneRow := findRow(t, page, "3")
	if !strings.Contains(doneRow, `data-state="done"`) {
		t.Fatalf("row 3 does not show done:\n%s", doneRow)
	}
	for _, forbidden := range []string{`class="btn btn--secondary edit"`, `class="edit-form"`, `name="title"`} {
		if strings.Contains(doneRow, forbidden) {
			t.Errorf("done row 3 carries an edit affordance %s:\n%s", forbidden, doneRow)
		}
	}
	// The not-done row shows the control beside it renders fine, so the
	// absence above is the rule, not a template accident.
	if editForm(t, page, "7") == "" {
		t.Errorf("not-done row 7 carries no edit form:\n%s", findRow(t, page, "7"))
	}

	// The text can only change after reopening via the toggle: an edit
	// PATCH against the still-done todo is refused by the contract and the
	// row re-renders done with its original text…
	status, frag := patchTitleFragment(t, uiSrv.URL, 3, "sneaky rewrite")
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("title PATCH on done row status = %d, want 422 (body %s)", status, frag)
	}
	assertRowState(t, frag, "3", "done", "settled task")

	// …and once the toggle reopens the row, the edit band is back.
	if status, frag = patchFragment(t, uiSrv.URL+"/ui/todos/3", "false"); status != http.StatusOK {
		t.Fatalf("reopen status = %d, want 200 (body %s)", status, frag)
	}
	if editForm(t, frag, "3") == "" {
		t.Errorf("reopened row 3 carries no edit form:\n%s", findRow(t, frag, "3"))
	}
}

// Card ui/11 — Stale edit of a done todo is refused visibly.
// Given a page stale about a todo's done state shows it as not-done
// When  the user submits an edit for it
// Then  the rejection states that the todo is done and its text cannot
//
//	be edited — the contract's own stated refusal.
//	And the row keeps displaying its original text, now truthfully done.
func TestStaleEditOfDoneTodoRefusedVisibly(t *testing.T) {
	fapi := &editAPI{todos: []todo{
		{ID: 3, Title: "walk the dog", Done: false},
		{ID: 7, Title: "other task", Done: false},
	}}
	api := fakeAPI(t, fapi.ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	// The page loads while the todo is not-done — it shows an edit control.
	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	if editForm(t, page, "3") == "" {
		t.Fatalf("not-done row 3 carries no edit form on load:\n%s", findRow(t, page, "3"))
	}

	// The todo becomes done server-side (another window, a toggle elsewhere)
	// while the open page stays stale and still shows the band.
	fapi.mu.Lock()
	fapi.todos[0].Done = true
	fapi.mu.Unlock()

	// The stale page submits an edit — the contract refuses with its stated
	// reason, and the swapped-in content is the server truth.
	status, frag := patchTitleFragment(t, uiSrv.URL, 3, "walk the dog in the park")
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("stale edit status = %d, want 422 (body %s)", status, frag)
	}
	if !strings.Contains(frag, `class="edit-error error-text"`) ||
		!strings.Contains(frag, "cannot edit a done todo") {
		t.Errorf("refusal not stated visibly on the row:\n%s", frag)
	}
	// The row keeps its original text — the typed text changed nothing —
	// and it shows done, so nothing looks like the edit succeeded.
	assertRowState(t, frag, "3", "done", "walk the dog")
	// A done row keeps no edit affordance even in the refusal render.
	if doneRow := findRow(t, frag, "3"); strings.Contains(doneRow, `class="edit-form"`) {
		t.Errorf("refusal render leaves an edit band on the done row:\n%s", doneRow)
	}
	// The other row is untouched, still not-done with its own band.
	assertRowState(t, frag, "7", "not-done", "other task")
}

// Card ui/12 — Empty edit text keeps the original.
// When  the user submits an edit with empty or whitespace-only text
// Then  the row keeps its original text
//
//	And the edit surface states that the text is required.
//	(Decision honored: the rejected text survives in the band for the
//	correcting submit.)
func TestEmptyEditTextKeepsTheOriginal(t *testing.T) {
	fapi := &editAPI{todos: []todo{
		{ID: 3, Title: "keep me", Done: false},
	}}
	api := fakeAPI(t, fapi.ServeHTTP)
	uiSrv := uiServer(t, api.URL)

	for _, typed := range []string{"", "   \t "} {
		status, frag := patchTitleFragment(t, uiSrv.URL, 3, typed)
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("empty edit %q status = %d, want 422 (body %s)", typed, status, frag)
		}
		// The edit surface states the contract's required-text refusal.
		if !strings.Contains(frag, `class="edit-error error-text"`) ||
			!strings.Contains(frag, "title is required") {
			t.Errorf("edit %q: required text not stated on the row:\n%s", typed, frag)
		}
		// The todo's text is untouched, and the row is still editable.
		assertRowState(t, frag, "3", "not-done", "keep me")
		// The band is open around the rejected text for the correcting
		// submit.
		form := editForm(t, frag, "3")
		if form == "" {
			t.Fatalf("edit %q: row 3 band gone after the refusal:\n%s", typed, frag)
		}
		row := findRow(t, frag, "3")
		if !strings.Contains(row, `class="row editing"`) {
			t.Errorf("edit %q: band not revealed after the refusal:\n%s", typed, row)
		}
		want := `value="` + typed + `"`
		if !strings.Contains(form, want) {
			t.Errorf("edit %q: rejected text did not survive in the band:\n%s", typed, form)
		}
	}

	// Nothing changed server-side: the original stands.
	list, err := fapi.snapshot()
	if err != nil {
		t.Fatalf("api snapshot: %v", err)
	}
	if len(list) != 1 || list[0].Title != "keep me" {
		t.Errorf("empty edits changed the todo: %+v", list)
	}
}

// snapshot reads the fake contract's list under its lock.
func (f *editAPI) snapshot() ([]todo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]todo(nil), f.todos...), nil
}
