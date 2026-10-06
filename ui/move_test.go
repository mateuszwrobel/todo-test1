package ui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"todo/api"
	"todo/board"
)

// contractCallLog records every write that crosses the ui→api boundary, so a
// test can pin the request COUNT and the request BODY, not just the visible
// outcome — the instrumentation behind "one update request carries the target
// column and drop position" (card ui/08).
type contractCallLog struct {
	mu    sync.Mutex
	calls []string // one "METHOD path body" per crossing
}

func (l *contractCallLog) record(call string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, call)
}

// patches returns the recorded PATCH /cards/... calls.
func (l *contractCallLog) patches() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, c := range l.calls {
		if strings.HasPrefix(c, "PATCH /cards/") {
			out = append(out, c)
		}
	}
	return out
}

// moveChain is realChain wrapped with the call recorder: the same real store
// under the same real api handler over real HTTP, with every PATCH crossing
// observed before it is served.
func moveChain(t *testing.T, seeds ...string) (*board.Store, *httptest.Server, *contractCallLog) {
	t.Helper()
	store, err := board.Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatalf("board.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, text := range seeds {
		if _, err := store.Create(text); err != nil {
			t.Fatalf("seed card %q: %v", text, err)
		}
	}
	log := new(contractCallLog)
	handler := api.NewHandler(store).ServeHTTP
	apiSrv := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/cards/") {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			log.record(r.Method + " " + r.URL.Path + " " + string(body))
		}
		handler(w, r)
	})
	return store, uiServer(t, apiSrv.URL), log
}

// dragMove sends what ONE accepted drop sends: a single PATCH to the ui move
// fragment endpoint carrying the target column and the drop position — the
// shell's single request site issues exactly this request (the browser drag
// gestures that reach it, and the one-fetch-per-gesture property itself, are
// the wave-end e2e lane's to pin).
func dragMove(t *testing.T, uiURL string, id int64, column string, position int) (int, string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"column": column, "position": position})
	if err != nil {
		t.Fatalf("marshal drop payload: %v", err)
	}
	target := uiURL + "/ui/cards/" + strconv.FormatInt(id, 10) + "/move"
	req, err := http.NewRequest(http.MethodPatch, target, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new PATCH %s: %v", target, err)
	}
	req.Header.Set("Content-Type", "application/json")
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

// assertOneMoveRequest pins the one-request rule on the recorder: exactly one
// PATCH crossed to the contract, for this card, with exactly this
// column-key/position body (the display-title → storage-key mapping the drag
// payload rides is part of what this asserts).
func assertOneMoveRequest(t *testing.T, log *contractCallLog, id int64, columnKey string, position int) {
	t.Helper()
	got := log.patches()
	if len(got) != 1 {
		t.Fatalf("contract saw %d PATCH calls, want exactly one per accepted drop: %q", len(got), got)
	}
	want := "PATCH /cards/" + strconv.FormatInt(id, 10)
	if !strings.HasPrefix(got[0], want+" ") {
		t.Fatalf("move crossed as %q, want it addressed %s", got[0], want)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(got[0], want+" ")), &body); err != nil {
		t.Fatalf("move body %q does not parse: %v", got[0], err)
	}
	if body["column"] != columnKey {
		t.Errorf("move body column = %v, want the contract key %q", body["column"], columnKey)
	}
	if pos, ok := body["position"].(float64); !ok || int(pos) != position {
		t.Errorf("move body position = %v, want %d", body["position"], position)
	}
}

// Card ui/08 — Drag between columns moves via one request.
// Given the page shows a card in "To Do" and cards in "In Progress"
// When  the user drags the card into "In Progress" between two cards and releases
// Then  one update request carries the target column and drop position
//
//	And the card renders in "In Progress" at the drop position and is gone from "To Do"
//	And during the drag the drop position is indicated before release
//
// (The gesture itself — dragover placing the indicator, exactly one fetch per
// drop, the browser-side swap — is client behavior the wave-end e2e lane
// pins; this lane pins everything the module owns: the wiring in the rendered
// page, the one-request-to-one-contract-call translation with the exact
// payload, and the server-truth re-render. The indicator↔index identity is
// pinned structurally: the indicator's gap and the request's index are both
// the one dropPosition count, and the drag script's presence test below
// keeps them that way.)
func TestDragBetweenColumnsMovesViaOneRequest(t *testing.T) {
	store, uiSrv, log := moveChain(t,
		"Write weekly report", "Fix login redirect", "Draft ADR-004")
	moveTo(t, store, 2, board.InProgress)
	moveTo(t, store, 3, board.InProgress)

	// Given the page shows the board with the card draggable.
	status, page := getPage(t, uiSrv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	if !strings.Contains(cardHTML(t, page, 1), `draggable="true"`) {
		t.Fatalf("To Do card 1 is not draggable:\n%s", cardHTML(t, page, 1))
	}

	// When the drop is accepted between the two In Progress cards: the drop
	// handler's ONE request, carrying display title and drop index.
	status, frag := dragMove(t, uiSrv.URL, 1, "In Progress", 1)
	if status != http.StatusOK {
		t.Fatalf("PATCH /ui/cards/1/move status = %d, want 200 (body %s)", status, frag)
	}

	// Then exactly one contract call crossed, with the contract key and the
	// drop position.
	assertOneMoveRequest(t, log, 1, "in_progress", 1)

	// And the answer is a swap fragment carrying the new truth.
	if strings.Contains(strings.ToLower(frag), "<!doctype") || strings.Contains(frag, "<html") {
		t.Errorf("move answered a full document, not a swap fragment:\n%s", frag)
	}
	if !strings.Contains(frag, `id="board"`) {
		t.Errorf("move answered no board content:\n%.200s", frag)
	}

	// The card renders in "In Progress" at the drop position (index 1:
	// between the two existing cards) and is gone from "To Do".
	at := columnHTML(t, frag, "in-progress")
	at1, at2, at3 := strings.Index(at, `id="card-1"`), strings.Index(at, `id="card-2"`), strings.Index(at, `id="card-3"`)
	if at1 < 0 || at2 < 0 || at3 < 0 {
		t.Fatalf("In Progress does not hold all three cards:\n%s", at)
	}
	if !(at2 < at1 && at1 < at3) {
		t.Errorf("moved card did not land at the drop position between the two cards:\n%s", at)
	}
	if todo := columnHTML(t, frag, "to-do"); strings.Contains(todo, `id="card-1"`) ||
		!strings.Contains(todo, `data-empty="true"`) {
		t.Errorf("source column did not lose the moved card and pack its gap:\n%s", todo)
	}

	// The store's own truth matches the render.
	if got := cardTitles(t, store, board.InProgress); len(got) != 3 ||
		got[0] != "Fix login redirect" || got[1] != "Write weekly report" || got[2] != "Draft ADR-004" {
		t.Errorf("store In Progress column = %q, want the card at the drop position", got)
	}
	if got := cardTitles(t, store, board.Todo); len(got) != 0 {
		t.Errorf("store To Do column = %q, want empty", got)
	}
}

// Card ui/08 (Done arm) — dragging into "Done" is the same one-request move
// (done is just a column), and the re-render derives the done treatment from
// the new membership alone — nothing per-card was touched.
func TestDragIntoDoneLandsWithDoneTreatment(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Wire the webhook")

	status, frag := dragMove(t, uiSrv.URL, 1, "Done", 0)
	if status != http.StatusOK {
		t.Fatalf("PATCH /ui/cards/1/move status = %d, want 200 (body %s)", status, frag)
	}
	assertOneMoveRequest(t, log, 1, "done", 0)

	done := columnHTML(t, frag, "done")
	if !strings.Contains(done, `id="card-1"`) || !strings.Contains(cardHTML(t, frag, 1), `card--done`) {
		t.Errorf("card did not land in Done with the done treatment:\n%s", done)
	}
	if got := cardTitles(t, store, board.Done); len(got) != 1 || got[0] != "Wire the webhook" {
		t.Errorf("store Done column = %q, want the dropped card", got)
	}
}

// Card ui/08 (stale leg) / card ui/11 (move leg) — dropping a card the
// server no longer holds takes the shared stale-failure surface (stale.go):
// the same banner over the truth every verb serves, mirrored status. The
// staleness is produced end-to-end through the real store — the card
// renders, then is deleted out-of-band before the drop. Nothing moved.
func TestDragMoveOfUnknownCardStatesMissing(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Still here", "Gone away")
	if err := store.Delete(2); err != nil {
		t.Fatalf("out-of-band delete: %v", err)
	}

	status, frag := dragMove(t, uiSrv.URL, 2, "In Progress", 0)
	if status != http.StatusNotFound {
		t.Fatalf("PATCH /ui/cards/2/move status = %d, want 404 (body %s)", status, frag)
	}
	if len(log.patches()) != 1 {
		t.Fatalf("the contract saw %v, want exactly one attempted move", log.patches())
	}
	if !strings.Contains(frag, `class="banner" role="alert">no such card<`) {
		t.Errorf("stale move did not state the failure:\n%s", frag)
	}
	if strings.Contains(frag, `id="card-2"`) || !strings.Contains(frag, "Still here") {
		t.Errorf("truth not re-rendered under the stale-move failure:\n%s", frag)
	}
	if got := cardTitles(t, store, board.Todo); len(got) != 1 || got[0] != "Still here" {
		t.Errorf("store To Do column = %q, want untouched by the stale move", got)
	}
}

// Card ui/08 (rejection leg) — a column name outside the fixed trio passes
// through to the contract's own guard and comes back on the shipped 422 arm:
// the contract's reason stated at the dragged card over the unchanged truth.
// No second rejection mechanism exists in this module.
func TestDragMoveToUnknownColumnStatesContractReason(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Unmovable to nowhere")

	status, frag := dragMove(t, uiSrv.URL, 1, "Backlog", 0)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("PATCH /ui/cards/1/move status = %d, want 422 (body %s)", status, frag)
	}
	assertOneMoveRequest(t, log, 1, "Backlog", 0)
	if !strings.Contains(frag, `id="edit-error-1"`) || !strings.Contains(frag, "invalid column") {
		t.Errorf("refusal not stated at the dragged card over the truth:\n%s", frag)
	}
	if got := cardTitles(t, store, board.Todo); len(got) != 1 || got[0] != "Unmovable to nowhere" {
		t.Errorf("store To Do column = %q, want untouched by the rejected move", got)
	}
}

// Card ui/08 (wiring presence) — every card in every column, Done included,
// is draggable by the card itself, and the shell's drag script keeps its
// one-request-per-drop shape: exactly one fetch site exists in the page's own
// script, reachable only through the column-targeted drop path (the
// abandonment legs return before it), with the drop indicator and the shared
// dropPosition count behind it.
func TestDragWiringIsOnEveryCardIncludingDone(t *testing.T) {
	store, uiSrv, _ := moveChain(t, "Drag me", "Already done")
	moveTo(t, store, 2, board.Done)

	_, page := getPage(t, uiSrv.URL+"/")
	for _, id := range []int64{1, 2} {
		if !strings.Contains(cardHTML(t, page, id), `draggable="true"`) {
			t.Errorf("card %d lacks its drag wiring:\n%s", id, cardHTML(t, page, id))
		}
	}
	script := page[strings.LastIndex(page, "<script>"):]
	if n := strings.Count(script, "fetch("); n != 1 {
		t.Errorf("page script has %d request sites, want exactly one per-drop fetch", n)
	}
	for _, marker := range []string{"dragstart", "dragover", "dropPosition", `closest('.column')`} {
		if !strings.Contains(script, marker) {
			t.Errorf("drag script lost its %q wiring", marker)
		}
	}
}

// assertColumnOrder pins one column's rendered order: the ids appearing in
// the given sequence, nothing else in between changing it.
func assertColumnOrder(t *testing.T, html, anchor string, ids ...int64) {
	t.Helper()
	col := columnHTML(t, html, anchor)
	prev := -1
	for _, id := range ids {
		at := strings.Index(col, `id="card-`+strconv.FormatInt(id, 10)+`"`)
		if at < 0 {
			t.Fatalf("column %s has no card %d:\n%s", anchor, id, col)
		}
		if at < prev {
			t.Errorf("column %s order does not hold card %d after the previous one:\n%s", anchor, id, col)
		}
		prev = at
	}
}

// assertStorePositionsContiguous pins the renormalization the store promises
// after every move: the column's cards sit at positions 0..n-1.
func assertStorePositionsContiguous(t *testing.T, store *board.Store, column board.Column) {
	t.Helper()
	columns, err := store.List()
	if err != nil {
		t.Fatalf("board List: %v", err)
	}
	for _, col := range columns {
		if col.Name != column {
			continue
		}
		for i, c := range col.Cards {
			if c.Position != i {
				t.Errorf("column %q card %q sits at position %d, want contiguous %d", column, c.Title, c.Position, i)
			}
		}
		return
	}
	t.Fatalf("store has no column %q", column)
}

// Card ui/09 — Drag reorder persists.
// Given a column shows at least three cards
// When  the user drags a card to a new position within the same column
// Then  the column re-renders in the new order immediately
//
//	And after a page reload the new order is still shown
//
// (Same machinery as ui/08 — one drop, one PATCH, same-column leg. "Immediately"
// is the swap fragment pinning below; the reload arm is pinned here against the
// server's truth — the browser actually issuing that reload is the wave-end
// e2e lane's to show. The drop index for a later position excludes the dragged
// card's own slot: To Do [1,2,3], line under card 3 → position 2 after removal.)
func TestDragReorderWithinColumnRerendersAndSurvivesReload(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Write weekly report", "Fix login redirect", "Buy milk")

	_, page := getPage(t, uiSrv.URL+"/")
	assertColumnOrder(t, page, "to-do", 1, 2, 3)

	status, frag := dragMove(t, uiSrv.URL, 1, "To Do", 2)
	if status != http.StatusOK {
		t.Fatalf("PATCH /ui/cards/1/move status = %d, want 200 (body %s)", status, frag)
	}
	assertOneMoveRequest(t, log, 1, "todo", 2)

	// The column re-renders in the new order immediately — the swap body
	// itself carries it, no reload required to see it.
	assertColumnOrder(t, frag, "to-do", 2, 3, 1)

	// After a page reload the new order is still shown: the fresh document
	// reads the server's truth, and the server holds the new order.
	_, reloaded := getPage(t, uiSrv.URL+"/")
	assertColumnOrder(t, reloaded, "to-do", 2, 3, 1)

	// The store's truth: the reorder persisted with contiguous positions.
	if got := cardTitles(t, store, board.Todo); len(got) != 3 ||
		got[0] != "Fix login redirect" || got[1] != "Buy milk" || got[2] != "Write weekly report" {
		t.Errorf("store To Do column = %q, want the dragged card at the drop position", got)
	}
	assertStorePositionsContiguous(t, store, board.Todo)
}

// Card ui/09 (toward-the-top arm) — dragging a later card to the column's
// top is the same same-column leg: the drop index is computed against the
// list after removal, so the line above card 2 asks for position 0 and the
// card lands first, ahead of both neighbours.
func TestDragReorderToTopOfColumnLandsFirst(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Write weekly report", "Fix login redirect", "Buy milk")

	status, frag := dragMove(t, uiSrv.URL, 3, "To Do", 0)
	if status != http.StatusOK {
		t.Fatalf("PATCH /ui/cards/3/move status = %d, want 200 (body %s)", status, frag)
	}
	assertOneMoveRequest(t, log, 3, "todo", 0)

	assertColumnOrder(t, frag, "to-do", 3, 1, 2)
	if got := cardTitles(t, store, board.Todo); len(got) != 3 ||
		got[0] != "Buy milk" || got[1] != "Write weekly report" || got[2] != "Fix login redirect" {
		t.Errorf("store To Do column = %q, want the dragged card first", got)
	}
	assertStorePositionsContiguous(t, store, board.Todo)
}

// Card ui/10 — Abandoned drag changes nothing.
// Given the page shows the board
// When  the user starts a drag and drops outside any valid target (or releases without a drop)
// Then  no request is sent and the board renders unchanged
//
// Pinned at both halves the module owns. Server side: with no accepted drop
// there is no path from a rendered page to the contract — reading the board
// crosses exactly one GET /board and zero writes, and the truth stands
// unchanged between loads. Client side: the abandonment legs never reach the
// request site — the drop handler returns before the single fetch when the
// release lands outside any .column, and dragend (the release-without-a-drop
// leg: ESC cancels the drag natively and surfaces here) issues no requests
// at all. The literal gesture proof — mouse released over the page gutter,
// request counters staying at zero in a live browser — arrives with the
// wave-end e2e lane.
func TestAbandonedDragChangesNothing(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Write weekly report", "Fix login redirect")

	// A board load with no accepted drop crosses no write at all.
	_, before := getPage(t, uiSrv.URL+"/")
	if got := log.patches(); len(got) != 0 {
		t.Errorf("page load crossed writes %v, want none", got)
	}

	// And the rendered truth between two loads is the same truth: the drag
	// phase the abandoned drag adds ends where it started.
	_, after := getPage(t, uiSrv.URL+"/")
	if before != after {
		t.Errorf("board changed across an abandoned drag:\n%s\n---\n%s", before, after)
	}
	if got := cardTitles(t, store, board.Todo); len(got) != 2 ||
		got[0] != "Write weekly report" || got[1] != "Fix login redirect" {
		t.Errorf("store To Do column = %q, want untouched by an abandoned drag", got)
	}

	// The script shape that makes zero requests structural: the drop leg
	// bails before the single fetch when no column accepts the release, and
	// the dragend leg (release without drop / ESC cancel) contains no
	// request site.
	_, page := getPage(t, uiSrv.URL+"/")
	script := page[strings.LastIndex(page, "<script>"):]
	dropLeg := script[strings.Index(script, "addEventListener('drop'"):strings.Index(script, "addEventListener('dragend'")]
	if bail := strings.Index(dropLeg, "if (!column) return;"); bail < 0 {
		t.Errorf("drop leg lost its outside-any-column bail:\n%.400s", dropLeg)
	} else if at := strings.Index(dropLeg, "fetch("); at >= 0 && at < bail {
		t.Errorf("drop leg can request before checking the drop landed on a column")
	}
	dragEndLeg := script[strings.Index(script, "addEventListener('dragend'"):]
	if strings.Contains(dragEndLeg, "fetch(") {
		t.Errorf("dragend issues a request — an abandoned drag must send nothing")
	}
}

// Card ui/10 (the verdict the card's text decides) — a drop that lands the
// card exactly where it already sits is NOT an abandonment: the card defines
// abandoning as dropping outside any valid target (or releasing without a
// drop), and the source slot inside the source column is a valid target at
// the card's own index. So it rides card ui/08's one-request rule — exactly
// one real PATCH — and what makes the outcome match ui/10's Then-clause is
// the answer: the server's re-normalized truth is identical, so the board
// renders unchanged without the client suppressing anything. The client
// computes; it does not second-guess accepted drops.
func TestSamePositionDropIsOneRealRequestNotAQuietNoOp(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Write weekly report", "Fix login redirect", "Buy milk")

	status, frag := dragMove(t, uiSrv.URL, 1, "To Do", 0)
	if status != http.StatusOK {
		t.Fatalf("PATCH /ui/cards/1/move status = %d, want 200 (body %s)", status, frag)
	}

	// One real request — accepted drop, one-PATCH rule, no client-side bail.
	assertOneMoveRequest(t, log, 1, "todo", 0)

	// The board renders unchanged: the same order back from the server.
	assertColumnOrder(t, frag, "to-do", 1, 2, 3)
	if got := cardTitles(t, store, board.Todo); len(got) != 3 ||
		got[0] != "Write weekly report" || got[1] != "Fix login redirect" || got[2] != "Buy milk" {
		t.Errorf("store To Do column = %q, want unchanged by the same-position drop", got)
	}
	assertStorePositionsContiguous(t, store, board.Todo)
}
