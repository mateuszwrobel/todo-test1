package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"todo/board"
)

// Cards ui/14–15 (KW9): the assignee control. Chip on every card the
// contract says is assigned (Done included — the chip is display, not an
// edit affordance), no chip element at all when unassigned; the edit band
// (non-Done only, the KW8 rule kept) carries the assignee select next to
// the title input; one save is ONE change request carrying title and
// assignee together; the seam's forced refusals state the contract's
// reason at the card through the existing error-at-card machinery.

// assigneeBoardJSON is the canned contract body for the render pins: an
// assigned card and an unassigned one in To Do, an assigned card in In
// Progress, an assigned card and an unassigned one in Done — every chip
// arm (present / absent × Done / not-Done) reachable on one page, with
// contract nulls spelled explicitly the way GET /board answers them.
const assigneeBoardJSON = `{
	"columns": [
		{"title": "To Do", "cards": [
			{"id": 1, "title": "Rotate the API keys", "column": "todo", "position": 1, "assignee": "Ada"},
			{"id": 2, "title": "Draft the launch note", "column": "todo", "position": 2, "assignee": null}]},
		{"title": "In Progress", "cards": [
			{"id": 3, "title": "Wire the webhook", "column": "in_progress", "position": 1, "assignee": "Grace"}]},
		{"title": "Done", "cards": [
			{"id": 4, "title": "Ship v1.2", "column": "done", "position": 1, "assignee": "Linus"},
			{"id": 5, "title": "Retire the cron job", "column": "done", "position": 2, "assignee": null}]}
	]
}`

func assigneeAPI(t *testing.T) *httptest.Server {
	t.Helper()
	return fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/board" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(assigneeBoardJSON))
	})
}

// patchCardBand submits the edit band the way htmx does — urlencoded form,
// whole band in one request — with caller-chosen fields, so a save can be
// sent title-only (the pre-KW9 band) or with the select's assignee value
// (empty = the select's Unassigned).
func patchCardBand(t *testing.T, uiURL string, id int64, fields url.Values) (int, string) {
	t.Helper()
	target := uiURL + "/ui/cards/" + strconv.FormatInt(id, 10)
	req, err := http.NewRequest(http.MethodPatch, target, strings.NewReader(fields.Encode()))
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

// cardAssignee reads one card's assignee straight from the store's truth.
func cardAssignee(t *testing.T, store *board.Store, id int64) string {
	t.Helper()
	columns, err := store.List()
	if err != nil {
		t.Fatalf("board List: %v", err)
	}
	for _, col := range columns {
		for _, c := range col.Cards {
			if c.ID == id {
				return c.Assignee
			}
		}
	}
	t.Fatalf("store has no card %d", id)
	return ""
}

// assertAscending pins that the given markers appear in this exact order
// inside the haystack — the option-order pin, stated as order rather than
// one brittle byte run.
func assertAscending(t *testing.T, label, haystack string, markers ...string) {
	t.Helper()
	prev := -1
	for _, m := range markers {
		at := strings.Index(haystack, m)
		if at < 0 {
			t.Errorf("%s: %q missing:\n%s", label, m, haystack)
			return
		}
		if at < prev {
			t.Errorf("%s: %q out of order:\n%s", label, m, haystack)
			return
		}
		prev = at
	}
}

// Card ui/14 (render arm) — the chip rides exactly the cards the contract
// assigns: every assigned card shows its name chip, unassigned cards carry
// no chip element, and Done changes nothing about the chip.
func TestAssigneeChipPerFixture(t *testing.T) {
	api := assigneeAPI(t)
	status, page := getPage(t, uiServer(t, api.URL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	for _, c := range []struct {
		id   int64
		name string // empty = the contract's null
	}{
		{1, "Ada"},   // To Do, assigned
		{2, ""},      // To Do, unassigned
		{3, "Grace"}, // In Progress, assigned
		{4, "Linus"}, // Done, assigned — the chip stands in Done (ui/15)
		{5, ""},      // Done, unassigned
	} {
		card := cardHTML(t, page, c.id)
		if c.name == "" {
			if strings.Contains(card, `class="card__assignee"`) {
				t.Errorf("unassigned card %d carries a chip element:\n%s", c.id, card)
			}
			continue
		}
		want := `<span class="card__assignee">` + c.name + `</span>`
		if !strings.Contains(card, want) {
			t.Errorf("card %d missing chip %s:\n%s", c.id, want, card)
		}
		// The chip rides beside the title, never in place of it.
		assertAscending(t, "card "+strconv.FormatInt(c.id, 10)+" markup", card,
			`class="card__title"`, `class="card__assignee"`)
	}
}

// Card ui/14 (render arm) — the band's select: Unassigned first, then the
// roster in the order the Roster port answers (contract order), the
// card's current value selected and nothing else.
func TestAssigneeSelectOptionsByteOrderAndSelection(t *testing.T) {
	api := assigneeAPI(t)
	status, page := getPage(t, uiServer(t, api.URL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	// Card 1 sits with Ada: the full option row in contract order with
	// Ada selected — Unassigned first, then Ada Grace Alan Barbara Linus.
	band := cardHTML(t, page, 1)
	if !strings.Contains(band, `<select class="input card__assignee-select" name="assignee">`) {
		t.Fatalf("card 1 band has no assignee select:\n%s", band)
	}
	assertAscending(t, "card 1 band", band,
		`name="assignee"`,
		`<option value="">Unassigned</option>`,
		`<option value="Ada" selected>Ada</option>`,
		`<option value="Grace">Grace</option>`,
		`<option value="Alan">Alan</option>`,
		`<option value="Barbara">Barbara</option>`,
		`<option value="Linus">Linus</option>`,
	)
	if got := strings.Count(band, " selected>"); got != 1 {
		t.Errorf("card 1 band selects %d options, want exactly 1:\n%s", got, band)
	}
	// The select sits next to the title input, ahead of Save.
	assertAscending(t, "card 1 band controls", band, `name="title"`, `name="assignee"`,
		`class="btn btn--primary save"`)

	// The unassigned card's band preselects Unassigned (the contract null).
	unassigned := cardHTML(t, page, 2)
	if !strings.Contains(unassigned, `<option value="" selected>Unassigned</option>`) {
		t.Errorf("card 2 band does not preselect Unassigned:\n%s", unassigned)
	}
	if got := strings.Count(unassigned, " selected>"); got != 1 {
		t.Errorf("card 2 band selects %d options, want exactly 1:\n%s", got, unassigned)
	}
}

// Card ui/15 — Done shows the chip only: the assigned Done card carries
// its chip, no assign control anywhere on it (band, select, Edit — all
// stay outside Done), and the delete + drag affordances the freeze never
// takes stay wired.
func TestDoneCardShowsChipOnly(t *testing.T) {
	api := assigneeAPI(t)
	status, page := getPage(t, uiServer(t, api.URL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}

	done := cardHTML(t, page, 4)
	if !strings.Contains(done, `<span class="card__assignee">Linus</span>`) {
		t.Errorf("Done card 4 missing its chip:\n%s", done)
	}
	for _, absent := range []string{`name="assignee"`, `<select`, `class="edit-form"`,
		`hx-patch="/ui/cards/4"`, `card__edit`} {
		if strings.Contains(done, absent) {
			t.Errorf("Done card 4 carries an assign/edit affordance %s:\n%s", absent, done)
		}
	}
	// The kept affordances: delete stays, drag stays — dragging out of Done
	// is what returns the band (pin below).
	for _, want := range []string{`card__delete`, `hx-delete="/ui/cards/4"`,
		`data-card="4"`, `draggable="true"`, `card--done`} {
		if !strings.Contains(done, want) {
			t.Errorf("Done card 4 missing %s (chip-only takes edit, not delete/drag):\n%s", want, done)
		}
	}
}

// Card ui/14 (write arm) — picking a roster name sends exactly ONE change
// request carrying title and assignee together, and the card comes back
// wearing that person's chip.
func TestAssigneeSaveIsOneRequestAndRendersChip(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Write weekly report", "Fix login redirect", "Buy milk")
	if _, err := store.Change(2, nil, nil, board.AssignTo("Ada")); err != nil {
		t.Fatalf("seed assign: %v", err)
	}
	startStatus, startPage := getPage(t, uiSrv.URL+"/")
	if startStatus != http.StatusOK ||
		!strings.Contains(cardHTML(t, startPage, 2), `<span class="card__assignee">Ada</span>`) {
		t.Fatalf("card 2 does not start assigned to Ada:\n%s", startPage)
	}

	status, frag := patchCardBand(t, uiSrv.URL, 2, url.Values{
		"title": {"Fix the login redirect"}, "assignee": {"Grace"},
	})
	if status != http.StatusOK {
		t.Fatalf("band save status = %d, want 200 (body %s)", status, frag)
	}

	// Exactly one change request left, and it carries BOTH fields.
	patches := log.patches()
	if len(patches) != 1 {
		t.Fatalf("save issued %d change requests, want exactly 1: %q", len(patches), patches)
	}
	want := `PATCH /cards/2 {"assignee":"Grace","title":"Fix the login redirect"}`
	if patches[0] != want {
		t.Errorf("change request = %q, want %q", patches[0], want)
	}
	if got := cardAssignee(t, store, 2); got != "Grace" {
		t.Errorf("store assignee = %q, want Grace", got)
	}
	if !strings.Contains(frag, `<span class="card__assignee">Grace</span>`) {
		t.Errorf("re-render does not show Grace's chip:\n%s", frag)
	}
	if strings.Contains(cardHTML(t, frag, 2), `<span class="card__assignee">Ada</span>`) {
		t.Errorf("re-render still shows the old chip beside the new one:\n%s", frag)
	}
}

// Card ui/14 (write arm) — the band picking Unassigned clears: the one
// request carries the contract null, the chip disappears, and the store
// answer is unassigned.
func TestUnassignedChoiceClearsTheChip(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Write weekly report", "Fix login redirect", "Buy milk")
	if _, err := store.Change(2, nil, nil, board.AssignTo("Alan")); err != nil {
		t.Fatalf("seed assign: %v", err)
	}

	status, frag := patchCardBand(t, uiSrv.URL, 2, url.Values{
		"title": {"Fix login redirect"}, "assignee": {""},
	})
	if status != http.StatusOK {
		t.Fatalf("unassign save status = %d, want 200 (body %s)", status, frag)
	}
	patches := log.patches()
	if len(patches) != 1 {
		t.Fatalf("unassign issued %d change requests, want exactly 1: %q", len(patches), patches)
	}
	want := `PATCH /cards/2 {"assignee":null,"title":"Fix login redirect"}`
	if patches[0] != want {
		t.Errorf("change request = %q, want %q", patches[0], want)
	}
	if got := cardAssignee(t, store, 2); got != "" {
		t.Errorf("store assignee = %q, want unassigned", got)
	}
	if strings.Contains(cardHTML(t, frag, 2), `class="card__assignee"`) {
		t.Errorf("card still renders a chip after unassigning:\n%s", cardHTML(t, frag, 2))
	}
}

// The three-state field, absent arm: a band submitted without an assignee
// (every pre-KW9 band leg, and every forced title-only seam request the
// earlier cards pin) carries NO assignee key into the contract — those
// requests stay byte-what-they-were, so their existing pins stand.
func TestTitleOnlySaveCarriesNoAssigneeField(t *testing.T) {
	_, uiSrv, log := moveChain(t, "Write weekly report", "Fix login redirect", "Buy milk")
	status, frag := patchCardForm(t, uiSrv.URL, 2, "Renamed without touching assignee")
	if status != http.StatusOK {
		t.Fatalf("title-only save status = %d, want 200 (body %s)", status, frag)
	}
	patches := log.patches()
	if len(patches) != 1 {
		t.Fatalf("title-only save issued %d change requests, want 1: %q", len(patches), patches)
	}
	want := `PATCH /cards/2 {"title":"Renamed without touching assignee"}`
	if patches[0] != want {
		t.Errorf("change request = %q, want %q (absent field, no direction)", patches[0], want)
	}
}

// Card ui/15 (seam arm) — forcing an assignee change on a Done card over
// PATCH /ui/cards/{id} mirrors the contract's 422 with "cannot edit a done
// card" at the card, on the unchanged truth: the existing error-at-card
// machinery (attachEditError), not a second mechanism. The chip and the
// done treatment stand exactly where they were.
func TestSeamForcedDoneAssigneeStatesFreezeAtCard(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Write weekly report", "Fix login redirect", "Buy milk")
	// Assign while the card is still editable (the freeze would rightly
	// refuse an assignment ON a Done card), then move it into Done.
	if _, err := store.Change(2, nil, nil, board.AssignTo("Grace")); err != nil {
		t.Fatalf("seed assign: %v", err)
	}
	moveTo(t, store, 2, board.Done)

	status, frag := patchCardBand(t, uiSrv.URL, 2, url.Values{
		"title": {"Hijacked"}, "assignee": {"Ada"},
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("forced Done assignee PATCH status = %d, want 422 (body %s)", status, frag)
	}
	card := cardHTML(t, frag, 2)
	if !strings.Contains(card, `class="error-text">cannot edit a done card<`) {
		t.Errorf("refusal not stated at the card:\n%s", card)
	}
	if !strings.Contains(card, `<span class="card__assignee">Grace</span>`) {
		t.Errorf("chip did not survive the refusal:\n%s", card)
	}
	if !strings.Contains(card, `<span class="card__title">Fix login redirect</span>`) ||
		!strings.Contains(card, `card--done`) {
		t.Errorf("truth changed under the refusal:\n%s", card)
	}
	if got := cardAssignee(t, store, 2); got != "Grace" {
		t.Errorf("store assignee changed despite the refusal: %q", got)
	}
	// The refusal path reads the truth; it never issues a second write.
	if got := log.patches(); len(got) != 1 {
		t.Errorf("refused save crossed the contract %d times, want 1: %q", len(got), got)
	}
}

// The unknown name over the seam answers the contract's own "unknown
// user" at the card — the roster screen has one owner (users, reached
// through board), the page only carries the wording.
func TestSeamForcedUnknownAssigneeStatesUnknownUserAtCard(t *testing.T) {
	store, uiSrv, _ := moveChain(t, "Write weekly report", "Fix login redirect", "Buy milk")
	status, frag := patchCardBand(t, uiSrv.URL, 2, url.Values{
		"title": {"Fix login redirect"}, "assignee": {"Zoe"},
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown-assignee PATCH status = %d, want 422 (body %s)", status, frag)
	}
	card := cardHTML(t, frag, 2)
	if !strings.Contains(card, `class="error-text">unknown user<`) {
		t.Errorf("unknown-user refusal not stated at the card:\n%s", card)
	}
	if strings.Contains(card, `class="card__assignee"`) {
		t.Errorf("refused name rendered as a chip:\n%s", card)
	}
	if got := cardAssignee(t, store, 2); got != "" {
		t.Errorf("store assignee changed despite the refusal: %q", got)
	}
}

// Card ui/15 (drag arm) — dragging out of Done returns the band with the
// select (the card keeps its assignee, so the select preselects the name);
// the Done card's chip rides through the move unchanged.
func TestDragOutOfDoneReturnsTheSelect(t *testing.T) {
	store, uiSrv, _ := moveChain(t, "Write weekly report", "Fix login redirect", "Buy milk")
	if _, err := store.Change(2, nil, nil, board.AssignTo("Linus")); err != nil {
		t.Fatalf("seed assign: %v", err)
	}
	moveTo(t, store, 2, board.Done)

	status, frag := dragMove(t, uiSrv.URL, 2, "To Do", 0)
	if status != http.StatusOK {
		t.Fatalf("drag out of Done status = %d, want 200 (body %s)", status, frag)
	}
	card := cardHTML(t, frag, 2)
	for _, want := range []string{`class="edit-form"`, `name="assignee"`,
		`<option value="Linus" selected>Linus</option>`,
		`<span class="card__assignee">Linus</span>`} {
		if !strings.Contains(card, want) {
			t.Errorf("card after drag-out missing %s (band must render normally):\n%s", want, card)
		}
	}
	if strings.Contains(card, `card--done`) {
		t.Errorf("card still wears the done treatment after the move-out:\n%s", card)
	}
}
