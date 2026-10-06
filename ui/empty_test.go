package ui

import (
	"net/http"
	"strings"
	"testing"
)

// Card ui/02 — Empty board renders as stated empty.
// Given the server holds no cards
// When  the user opens the page
// Then  the three columns render with their empty treatment — visibly an
//
//	empty board, not a blank or broken page
func TestEmptyBoardRendersAsStatedEmpty(t *testing.T) {
	api := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"columns": [
			{"title": "To Do", "cards": []},
			{"title": "In Progress", "cards": []},
			{"title": "Done", "cards": []}
		]}`))
	})
	status, page := getPage(t, uiServer(t, api.URL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	// The board still renders: three columns, each stating its emptiness —
	// visibly an empty board, not a blank page.
	if !strings.Contains(page, `id="board"`) {
		t.Fatalf("empty board does not render the board surface:\n%s", page)
	}
	for _, anchor := range []string{"to-do", "in-progress", "done"} {
		col := columnHTML(t, page, anchor)
		if !strings.Contains(col, `class="column__empty"`) {
			t.Errorf("empty column #%s lacks the empty treatment:\n%s", anchor, col)
		}
		if !strings.Contains(col, "No cards") {
			t.Errorf("empty column #%s does not state its emptiness:\n%s", anchor, col)
		}
	}

	// Distinct: no cards anywhere (an empty board, not a stale render).
	if strings.Contains(page, `id="card-`) {
		t.Errorf("empty board renders cards:\n%s", page)
	}
	// And not the load-failure masquerade: emptiness stated is not failure.
	if strings.Contains(page, `id="load-error"`) || strings.Contains(page, "Could not load") {
		t.Errorf("empty board masquerades as the failure state:\n%s", page)
	}
}
