package ui

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"todo/board"
)

// cardTitles returns one column's card titles top-to-bottom from the store's
// own truth (what the next board read will render).
func cardTitles(t *testing.T, store *board.Store, column board.Column) []string {
	t.Helper()
	columns, err := store.List()
	if err != nil {
		t.Fatalf("board List: %v", err)
	}
	for _, col := range columns {
		if col.Name != column {
			continue
		}
		titles := make([]string, 0, len(col.Cards))
		for _, c := range col.Cards {
			titles = append(titles, c.Title)
		}
		return titles
	}
	t.Fatalf("board has no %q column: %+v", column, columns)
	return nil
}

// moveTo shifts a seeded card into another column straight through the
// store — the board's own truth for the state a drag (KW5) will produce.
func moveTo(t *testing.T, store *board.Store, id int64, column board.Column) {
	t.Helper()
	if _, err := store.Change(id, nil, &column); err != nil {
		t.Fatalf("seed move to %q: %v", column, err)
	}
}

// patchCardForm submits a card's edit band the way htmx does: a form PATCH
// to the ui fragment endpoint (hx-patch serializes urlencoded, exactly as
// the create form's POST does).
func patchCardForm(t *testing.T, uiURL string, id int64, title string) (int, string) {
	t.Helper()
	target := uiURL + "/ui/cards/" + strconv.FormatInt(id, 10)
	req, err := http.NewRequest(http.MethodPatch, target,
		strings.NewReader(url.Values{"title": {title}}.Encode()))
	if err != nil {
		t.Fatalf("new PATCH %s: %v", target, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", target, err)
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, buf.String()
}

// Card ui/06 (surface arm) — every card in every column carries the live
// edit band, Done included (no frozen-done: J6 states a done card's text is
// editable; done is just a column). The canned board has cards in all three
// columns, so one page proves all three arms.
func TestEditBandOnEveryCardEveryColumn(t *testing.T) {
	api := boardAPI(t)
	status, page := getPage(t, uiServer(t, api.URL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	for _, id := range []int64{1, 2, 3, 4} {
		card := cardHTML(t, page, id)
		idStr := strconv.FormatInt(id, 10)
		// The band: a prefilled form wired to the card's own fragment
		// endpoint, swapping the board area in place — the create
		// surface's no-reload mechanism on the card surface.
		for _, want := range []string{`class="edit-form"`, `hx-patch="/ui/cards/` + idStr + `"`,
			`hx-target="#board-area"`, `name="title" value="`} {
			if !strings.Contains(card, want) {
				t.Errorf("card %s band missing %s:\n%s", idStr, want, card)
			}
		}
		if !strings.Contains(card, `>Save</button>`) || !strings.Contains(card, `>Cancel</button>`) {
			t.Errorf("card %s band has no save/cancel controls:\n%s", idStr, card)
		}
		// The Edit control stays a plain button — revealing the band is
		// client-side, it triggers no request itself.
		if !strings.Contains(card, `<button type="button" class="btn btn--secondary card__edit">`) {
			t.Errorf("card %s edit control is not a plain request-free button:\n%s", idStr, card)
		}
	}

	// The delete affordance is still an inert placeholder (KW4 wires it):
	// the plain button element and nothing more.
	for _, id := range []int64{1, 2, 3, 4} {
		if !strings.Contains(cardHTML(t, page, id), `<button type="button" class="btn btn--secondary card__delete">`) {
			t.Errorf("card %d delete affordance carries wiring before KW4:\n%s", id, cardHTML(t, page, id))
		}
	}
	// The shell routes edit refusals into the swap engine — the no-reload
	// error path, no browser-side rendering.
	if !strings.Contains(page, "htmx:responseError") || !strings.Contains(page, ".edit-form") {
		t.Errorf("shell has no edit-refusal swap routing:\n%s", page)
	}
}

// Card ui/06 — Edit updates in place.
// Given the page shows a card in any column
// When  the user edits the card's text and saves
// Then  the card shows the new text at the same position in the same column
//
//	(and no page reload — the answer is a swap fragment, htmx swapping it
//	in place; the e2e lane pins that browser behavior)
func TestEditUpdatesInPlace(t *testing.T) {
	store, uiSrv := realChain(t, "Write weekly report", "Fix login redirect", "Buy milk")

	// Given the page shows the board, every card wired for edit-in-place.
	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	if !strings.Contains(cardHTML(t, page, 2), `hx-patch="/ui/cards/2"`) {
		t.Fatalf("card 2 has no edit band:\n%s", page)
	}

	// When the middle card's band saves a new text.
	status, frag := patchCardForm(t, uiSrv.URL, 2, "Fix the login redirect")
	if status != http.StatusOK {
		t.Fatalf("PATCH /ui/cards/2 status = %d, want 200 (body %s)", status, frag)
	}

	// Then: a fragment (no document → nothing to reload).
	if strings.Contains(strings.ToLower(frag), "<!doctype") || strings.Contains(frag, "<html") {
		t.Errorf("edit answered a full document, not a swap fragment:\n%s", frag)
	}
	// The card shows the new text, at the same position in the same column:
	// still card-2, still between card-1 and card-3 in To Do.
	if !strings.Contains(frag, `<span class="card__title">Fix the login redirect</span>`) {
		t.Errorf("edited title not rendered:\n%s", frag)
	}
	todo := columnHTML(t, frag, "to-do")
	prev := -1
	for _, id := range []int64{1, 2, 3} {
		at := strings.Index(todo, `id="card-`+strconv.FormatInt(id, 10)+`"`)
		if at < 0 {
			t.Fatalf("To Do lost card %d:\n%s", id, todo)
		}
		if at < prev {
			t.Errorf("To Do order changed around the edit:\n%s", todo)
		}
		prev = at
	}
	// The contract's truth: same three cards, the middle one renamed.
	if got := cardTitles(t, store, board.Todo); len(got) != 3 ||
		got[0] != "Write weekly report" || got[1] != "Fix the login redirect" || got[2] != "Buy milk" {
		t.Errorf("store To Do column = %q, want only the middle title changed", got)
	}
}

// Card ui/06 (Done arm) — a card in the Done column is just as editable;
// the edit touches text only, so the done treatment (class card--done,
// derived from column membership) survives the swap unchanged.
func TestEditDoneCardKeepsDoneTreatmentInPlace(t *testing.T) {
	store, uiSrv := realChain(t, "Migrate todo list", "Set up CI")
	moveTo(t, store, 2, board.Done)

	_, page := getPage(t, uiSrv.URL+"/")
	if !strings.Contains(cardHTML(t, page, 2), `card--done`) {
		t.Fatalf("card 2 (Done column) lacks the done treatment:\n%s", page)
	}

	// Editing a Done card is a normal operation — no frozen-done refusal,
	// no different status.
	status, frag := patchCardForm(t, uiSrv.URL, 2, "Set up CI pipelines")
	if status != http.StatusOK {
		t.Fatalf("PATCH /ui/cards/2 status = %d, want 200 (body %s)", status, frag)
	}

	// Same column, same position, new text, still the done treatment.
	done := columnHTML(t, frag, "done")
	if !strings.Contains(done, `id="card-2"`) || !strings.Contains(done, "Set up CI pipelines") {
		t.Errorf("edited Done card not rendered in the Done column:\n%s", frag)
	}
	if !strings.Contains(cardHTML(t, frag, 2), `card--done`) {
		t.Errorf("done treatment did not survive the edit swap:\n%s", cardHTML(t, frag, 2))
	}
	if strings.Contains(columnHTML(t, frag, "to-do"), `id="card-2"`) {
		t.Errorf("editing a Done card moved it:\n%s", frag)
	}
	if got := cardTitles(t, store, board.Done); len(got) != 1 || got[0] != "Set up CI pipelines" {
		t.Errorf("store Done column = %q, want only the title changed", got)
	}
	if got := cardTitles(t, store, board.Todo); len(got) != 1 || got[0] != "Migrate todo list" {
		t.Errorf("store To Do column = %q, want untouched", got)
	}
}

// Card ui/06 — Rejected edit states the reason at the editing card.
// Given the page shows a card
// When  the edit saves text the server rejects (blank or over-long)
// Then  the card's original text stays visible with the stated reason —
//
//	the contract's own message at the card, board otherwise unchanged
func TestRejectedEditStatesReasonAtCard(t *testing.T) {
	t.Run("blank", func(t *testing.T) {
		store, uiSrv := realChain(t, "Write weekly report")

		status, frag := patchCardForm(t, uiSrv.URL, 1, "   ")

		// The contract's refusal status, mirrored.
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("PATCH /ui/cards/1 status = %d, want 422 (body %s)", status, frag)
		}
		// The stated reason, the contract's exact string, at the editing
		// card — not re-worded here.
		card := cardHTML(t, frag, 1)
		if !strings.Contains(card, `id="edit-error-1"`) ||
			!strings.Contains(card, `class="error-text">title is required<`) {
			t.Errorf("refusal not stated at the editing card:\n%s", card)
		}
		// The card's original text stays visible.
		if !strings.Contains(card, `<span class="card__title">Write weekly report</span>`) {
			t.Errorf("original text not kept on the rejected card:\n%s", card)
		}
		// Board unchanged.
		if got := cardTitles(t, store, board.Todo); len(got) != 1 || got[0] != "Write weekly report" {
			t.Errorf("store To Do column = %q, want the board untouched by the refusal", got)
		}
	})

	t.Run("over the limit", func(t *testing.T) {
		store, uiSrv := realChain(t, "Write weekly report")

		status, frag := patchCardForm(t, uiSrv.URL, 1, strings.Repeat("x", board.MaxTextLen+1))
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("PATCH /ui/cards/1 status = %d, want 422 (body %s)", status, frag)
		}
		// The limit message flows through with its stated number, exactly
		// as the contract words it.
		card := cardHTML(t, frag, 1)
		if !strings.Contains(card, `class="error-text">title exceeds the 500 character limit<`) {
			t.Errorf("contract's character-limit message not surfaced verbatim:\n%s", card)
		}
		if !strings.Contains(card, `<span class="card__title">Write weekly report</span>`) {
			t.Errorf("original text not kept on the rejected card:\n%s", card)
		}
		if got := cardTitles(t, store, board.Todo); len(got) != 1 {
			t.Errorf("store To Do column = %q, want the board untouched by the refusal", got)
		}
	})
}

// Card ui/06 (cancel arm) — Cancel discards without a request. The band is
// plain markup toggled by the shell script: the Cancel control is a
// non-submitting button with no hx-* wiring, and the script's cancel branch
// hides the band without touching the network — so a discarded edit cannot
// change anything by construction. The browser behavior itself belongs to
// the e2e lane.
func TestEditCancelIsClientSideOnly(t *testing.T) {
	store, uiSrv := realChain(t, "Write weekly report")

	_, page := getPage(t, uiSrv.URL+"/")
	card := cardHTML(t, page, 1)
	if !strings.Contains(card, `<button type="button" class="btn btn--secondary cancel">`) {
		t.Errorf("cancel is not a plain non-submitting button:\n%s", card)
	}
	// The discard happens in the shell script: hiding the band, nothing
	// else — no request path for cancel exists.
	if !strings.Contains(page, "classList.remove('editing')") {
		t.Errorf("shell has no request-free cancel path:\n%s", page)
	}
	// Nothing was sent, so nothing changed.
	if got := cardTitles(t, store, board.Todo); len(got) != 1 || got[0] != "Write weekly report" {
		t.Errorf("store To Do column = %q, want untouched", got)
	}
}

// Parent scenario (s11 leg) — Stale edit states the failure.
// Given the page shows a card the server no longer holds (deleted in
// another session — here, an id the board never had, indistinguishable to
// this page until delete lands at KW4),
// When  the user saves an edit on it
// Then  the edit is not applied — the truth re-renders without the card,
//
//	nothing left faking the change — and the page states that the card
//	does not exist, in the contract's own words
func TestEditOfUnknownCardStatesMissing(t *testing.T) {
	store, uiSrv := realChain(t, "Still here")

	status, frag := patchCardForm(t, uiSrv.URL, 999, "ghost edit")

	// The contract's 404, mirrored onto the fragment.
	if status != http.StatusNotFound {
		t.Fatalf("PATCH /ui/cards/999 status = %d, want 404 (body %s)", status, frag)
	}
	// The stated failure at the top of the swap surface — the contract's
	// own "no such card" in the design system's banner, so the edit
	// surfaces WHERE the failure is and the board under it is the server's
	// truth without the stale card.
	if !strings.Contains(frag, `class="banner" role="alert">no such card<`) {
		t.Errorf("stated missing-card failure not rendered:\n%s", frag)
	}
	if strings.Contains(frag, `id="card-999"`) {
		t.Errorf("stale card left on the board:\n%s", frag)
	}
	if !strings.Contains(frag, "Still here") {
		t.Errorf("board truth not re-rendered under the failure:\n%s", frag)
	}
	// The board never gained or lost anything.
	if got := cardTitles(t, store, board.Todo); len(got) != 1 || got[0] != "Still here" {
		t.Errorf("store To Do column = %q, want untouched by the stale edit", got)
	}
}
