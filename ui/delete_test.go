package ui

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"todo/board"
)

// clickDelete activates a card's delete control the way htmx does: a real
// DELETE to the ui fragment endpoint (hx-delete issues exactly this verb and
// target; the no-reload swap itself is htmx browser behavior, the e2e lane's
// to pin).
func clickDelete(t *testing.T, uiURL string, id int64) (int, string) {
	t.Helper()
	target := uiURL + "/ui/cards/" + strconv.FormatInt(id, 10)
	req, err := http.NewRequest(http.MethodDelete, target, nil)
	if err != nil {
		t.Fatalf("new DELETE %s: %v", target, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", target, err)
	}
	defer resp.Body.Close()
	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, buf.String()
}

// Card ui/07 — Delete drops one card.
// Given the page shows a card
// When  the user activates that card's delete control
// Then  the card disappears and its column's remaining cards keep their
//
//	order with no gap
//
//	And every other card on the board is untouched
//
// (The disappearance being visible without a page reload is the fragment
// shape below — a swap body, not a document; htmx swapping it in place is
// browser behavior the e2e lane pins.)
func TestDeleteDropsMiddleCardKeepsOrder(t *testing.T) {
	store, uiSrv := realChain(t, "Write weekly report", "Fix login redirect", "Buy milk")

	// Given the page shows the board, every card wired for delete-in-place.
	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	if !strings.Contains(cardHTML(t, page, 2), `hx-delete="/ui/cards/2"`) {
		t.Fatalf("card 2 has no delete wiring:\n%s", page)
	}

	// When the middle card's Delete control is activated.
	status, frag := clickDelete(t, uiSrv.URL, 2)
	if status != http.StatusOK {
		t.Fatalf("DELETE /ui/cards/2 status = %d, want 200 (body %s)", status, frag)
	}

	// Then: a fragment (no document → nothing to reload), and it carries the
	// board — the result is visible immediately, not silent.
	if strings.Contains(strings.ToLower(frag), "<!doctype") || strings.Contains(frag, "<html") {
		t.Errorf("delete answered a full document, not a swap fragment:\n%s", frag)
	}
	if !strings.Contains(frag, `id="board"`) {
		t.Errorf("delete answered no board content — the result would be invisible:\n%s", frag)
	}

	// The card is gone, and the column packs up with no gap: card 1 then
	// card 3, in order, nothing between them.
	todo := columnHTML(t, frag, "to-do")
	if strings.Contains(todo, `id="card-2"`) {
		t.Errorf("deleted card still on the board:\n%s", todo)
	}
	at1, at3 := strings.Index(todo, `id="card-1"`), strings.Index(todo, `id="card-3"`)
	if at1 < 0 || at3 < 0 {
		t.Fatalf("To Do lost a sibling of the deleted card:\n%s", todo)
	}
	if at1 > at3 {
		t.Errorf("remaining cards changed order around the delete:\n%s", todo)
	}
	// The stated titles: exactly the two survivors, untouched.
	if strings.Contains(todo, "Fix login redirect") {
		t.Errorf("deleted card's title still rendered:\n%s", todo)
	}
	if !strings.Contains(todo, "Write weekly report") || !strings.Contains(todo, "Buy milk") {
		t.Errorf("sibling titles not kept:\n%s", todo)
	}

	// The contract's truth: two To Do cards, the middle one gone, order kept.
	if got := cardTitles(t, store, board.Todo); len(got) != 2 ||
		got[0] != "Write weekly report" || got[1] != "Buy milk" {
		t.Errorf("store To Do column = %q, want only the deleted card removed", got)
	}
}

// Card ui/07 (Done arm) — a card in the Done column is just as deletable;
// no frozen-done (J6: done is just a column). The deletion is a normal
// operation — same status, same fragment mechanism — and every card in
// every other column is untouched by it.
func TestDeleteDoneCardIsNotSpecial(t *testing.T) {
	store, uiSrv := realChain(t, "Migrate todo list", "Set up CI")
	moveTo(t, store, 2, board.Done)

	_, page := getPage(t, uiSrv.URL+"/")
	if !strings.Contains(cardHTML(t, page, 2), `hx-delete="/ui/cards/2"`) ||
		!strings.Contains(cardHTML(t, page, 2), `card--done`) {
		t.Fatalf("Done card 2 lacks its done treatment or delete wiring:\n%s", page)
	}

	// Deleting a Done card is a normal operation — no frozen-done refusal,
	// no different status.
	status, frag := clickDelete(t, uiSrv.URL, 2)
	if status != http.StatusOK {
		t.Fatalf("DELETE /ui/cards/2 status = %d, want 200 (body %s)", status, frag)
	}

	// The Done column now shows its stated empty treatment, and the card is
	// gone from everywhere.
	done := columnHTML(t, frag, "done")
	if strings.Contains(done, `id="card-2"`) || !strings.Contains(done, `data-empty="true"`) {
		t.Errorf("Done column not left empty as stated:\n%s", done)
	}
	if strings.Contains(frag, `id="card-2"`) {
		t.Errorf("deleted Done card rendered somewhere:\n%s", frag)
	}
	// The sibling in To Do is untouched — title and controls intact.
	if got := columnHTML(t, frag, "to-do"); !strings.Contains(got, "Migrate todo list") ||
		!strings.Contains(got, `hx-delete="/ui/cards/1"`) {
		t.Errorf("To Do sibling changed by the Done delete:\n%s", frag)
	}
	if got := cardTitles(t, store, board.Done); len(got) != 0 {
		t.Errorf("store Done column = %q, want empty", got)
	}
	if got := cardTitles(t, store, board.Todo); len(got) != 1 || got[0] != "Migrate todo list" {
		t.Errorf("store To Do column = %q, want untouched", got)
	}
}

// Card ui/07 (stale arm) / parent scenario s11 (delete leg) — a delete of a
// card the server no longer holds states the failure in the contract's own
// words and re-renders the truth without the card — the shared stale-failure
// surface (stale.go), one mechanism per failure class across verbs.
func TestDeleteOfUnknownCardStatesMissing(t *testing.T) {
	store, uiSrv := realChain(t, "Still here")

	status, frag := clickDelete(t, uiSrv.URL, 999)

	// The contract's 404, mirrored onto the fragment.
	if status != http.StatusNotFound {
		t.Fatalf("DELETE /ui/cards/999 status = %d, want 404 (body %s)", status, frag)
	}
	// The stated failure at the top of the swap surface — the contract's own
	// "no such card" in the design system's banner, with the board under it
	// the server's truth without the stale card.
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
		t.Errorf("store To Do column = %q, want untouched by the stale delete", got)
	}
}

// Card ui/07 (already-deleted arm) — the same stated 404 when the page's
// card was deleted between rendering and the click: the first delete drops
// the card normally, the second delete of the same id is the contract's
// "no such card" over the truth, which already lacks it.
func TestDeleteOfAlreadyDeletedCardStatesMissing(t *testing.T) {
	store, uiSrv := realChain(t, "Gone soon", "Living on")

	status, frag := clickDelete(t, uiSrv.URL, 1)
	if status != http.StatusOK {
		t.Fatalf("first DELETE /ui/cards/1 status = %d, want 200 (body %s)", status, frag)
	}
	if strings.Contains(frag, `id="card-1"`) || !strings.Contains(frag, "Living on") {
		t.Fatalf("first delete did not drop the card cleanly:\n%s", frag)
	}

	// The stale click on the same card.
	status, frag = clickDelete(t, uiSrv.URL, 1)
	if status != http.StatusNotFound {
		t.Fatalf("second DELETE /ui/cards/1 status = %d, want 404 (body %s)", status, frag)
	}
	if !strings.Contains(frag, `class="banner" role="alert">no such card<`) {
		t.Errorf("stale delete did not state the failure:\n%s", frag)
	}
	if strings.Contains(frag, "Gone soon") {
		t.Errorf("already-deleted card rendered under the failure:\n%s", frag)
	}
	if !strings.Contains(frag, "Living on") {
		t.Errorf("board truth not re-rendered under the failure:\n%s", frag)
	}
	if got := cardTitles(t, store, board.Todo); len(got) != 1 || got[0] != "Living on" {
		t.Errorf("store To Do column = %q, want the survivor only", got)
	}
}
