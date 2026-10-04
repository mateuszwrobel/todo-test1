package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// createAPI is an in-memory stand-in for the api contract over real HTTP,
// speaking the contract's create semantics (201 + created JSON; 422 with the
// stated refusal). The ui module reaches it only via its base URL.
type createAPI struct {
	todos    []todo // guarded by test sequencing
	nextID   int64
	maxTitle int
}

func (f *createAPI) serve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/todos":
			list := f.todos
			if list == nil {
				list = []todo{}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(list)
		case r.Method == http.MethodPost && r.URL.Path == "/todos":
			var req struct {
				Title string `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			title := strings.TrimSpace(req.Title)
			switch {
			case title == "":
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"error": "title is required"}`))
				return
			case len([]rune(title)) > f.maxTitle:
				w.WriteHeader(http.StatusUnprocessableEntity)
				fmt.Fprintf(w, `{"error": "title exceeds the %d-character limit"}`, f.maxTitle)
				return
			}
			f.nextID++
			created := todo{ID: f.nextID, Title: title, Done: false}
			f.todos = append(f.todos, created)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(created)
		default:
			http.NotFound(w, r)
		}
	}
}

func postCreateForm(t *testing.T, uiURL, title string) (int, string) {
	t.Helper()
	resp, err := http.PostForm(uiURL+"/ui/todos", url.Values{"title": {title}})
	if err != nil {
		t.Fatalf("POST /ui/todos: %v", err)
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, buf.String()
}

// Card ui/05 — Create appends without reload.
// Given the page shows the list
// When  the user types text into the create input and submits
// Then  the response swaps in fresh list content without a full page reload
//
//	And the new todo is the last row, marked not-done
//	And the create input and Add button are ready for the next todo
//
// The fragment shape below is what makes "without reload" possible: a swap-
// targeted fragment, not a whole document. The no-navigation guarantee
// itself is asserted in the browser (e2e/w2-create.js).
func TestCreateAppendsWithoutReload(t *testing.T) {
	api := &createAPI{nextID: 0, maxTitle: 500}
	api.todos = []todo{{ID: 1, Title: "existing", Done: false}}
	apiSrv := fakeAPI(t, api.serve())
	uiSrv := uiServer(t, apiSrv.URL)

	// Given the page shows the list, wired for htmx create-in-place.
	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	for _, want := range []string{`hx-post="/ui/todos"`, `id="todos-area"`, `id="create-area"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %s:\n%s", want, page)
		}
	}

	// When the create form submits.
	status, frag := postCreateForm(t, uiSrv.URL, "Buy milk")
	if status != http.StatusCreated {
		t.Fatalf("POST /ui/todos status = %d, want 201 (body %s)", status, frag)
	}

	// Then: a fragment (no document → no page reload semantics).
	if strings.Contains(strings.ToLower(frag), "<!doctype") || strings.Contains(frag, "<html") {
		t.Errorf("create answered a full document, not a swap fragment:\n%s", frag)
	}
	// The new todo is the last row, marked not-done.
	iExisting := strings.Index(frag, "existing")
	iNew := strings.Index(frag, "Buy milk")
	if iExisting < 0 || iNew < 0 {
		t.Fatalf("fragment missing rows (existing at %d, new at %d):\n%s", iExisting, iNew, frag)
	}
	if iNew < iExisting {
		t.Errorf("new todo is not the last row:\n%s", frag)
	}
	newRow := frag[max(0, iNew-400):]
	if !strings.Contains(newRow, `data-state="not-done"`) {
		t.Errorf("new row does not show not-done state:\n%s", frag)
	}
	// The create area comes along reset for the next todo (out-of-band swap),
	// with an empty input and an Add button.
	if !strings.Contains(frag, `id="create-area"`) || !strings.Contains(frag, "hx-swap-oob") {
		t.Errorf("fragment carries no out-of-band create-area reset:\n%s", frag)
	}
	if !strings.Contains(frag, `name="title" value=""`) || !strings.Contains(frag, ">Add<") {
		t.Errorf("create control not ready for the next todo:\n%s", frag)
	}
	// And the contract saw the create.
	if len(api.todos) != 2 || api.todos[1].Title != "Buy milk" || api.todos[1].Done {
		t.Errorf("api contract state = %+v, want the created todo appended not-done", api.todos)
	}
}
