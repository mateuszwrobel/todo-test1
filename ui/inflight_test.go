package ui

import (
	"net/http"
	"strings"
	"testing"
)

// Card ui/16 — Controls serialize operations per control: while the
// operation triggered by a control is in flight, that control is disabled
// and a second activation causes no request. The htmx-native mechanism is
// hx-disabled-elt on the request origin naming exactly the control the
// user activates — "this" when the control itself issues the request
// (checkbox, delete button), an exact selector when a form does (its submit
// button, which is the control). Per-control scope: nothing else on the
// row or page is blocked.

// lineWith returns the single line of html containing want.
func lineWith(t *testing.T, html, want string) string {
	t.Helper()
	for _, line := range strings.Split(html, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	t.Fatalf("no line containing %q in:\n%s", want, html)
	return ""
}

func TestRowControlsCarryInFlightDisableWiring(t *testing.T) {
	api := deleteStubAPI(t, []fakeTodo{{ID: 5, Title: "alpha", Done: false}})
	uiSrv := uiServer(t, api.URL)

	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	row := findRow(t, page, "5")

	// The checkbox blocks itself: hx-disabled-elt="this" on the control.
	if want := lineWith(t, row, `<input type="checkbox"`); !strings.Contains(want, `hx-disabled-elt="this"`) {
		t.Fatalf("row checkbox does not disable itself while in flight: %s", want)
	}
	// The edit form's save button is the control the form submits with; the
	// selector is row-scoped so another row's save stays usable.
	form := editForm(t, row, "5")
	if !strings.Contains(form, `hx-disabled-elt="#todo-5 .save"`) {
		t.Fatalf("edit-save is not blocked while in flight: %s", form)
	}
	if !strings.Contains(form, `class="btn btn--primary save">Save`) {
		t.Fatalf("save button must carry the .save class the form's selector names: %s", form)
	}
	// The delete button blocks itself.
	if want := lineWith(t, row, `class="btn btn--secondary delete"`); !strings.Contains(want, `hx-disabled-elt="this"`) {
		t.Fatalf("row delete control does not disable itself while in flight: %s", want)
	}
}

func TestCreateControlCarriesInFlightDisableWiring(t *testing.T) {
	api := deleteStubAPI(t, []fakeTodo{{ID: 5, Title: "alpha", Done: false}})
	uiSrv := uiServer(t, api.URL)

	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	// The create form's Add button is the control; the form disables it
	// while the POST is in flight.
	if want := lineWith(t, page, `id="create-form"`); !strings.Contains(want, `hx-disabled-elt="#create-form button[type=submit]"`) {
		t.Fatalf("create form does not disable its Add button while in flight: %s", want)
	}
}
