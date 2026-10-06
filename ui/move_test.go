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

// Card ui/08 (stale leg) — dropping a card the server no longer holds takes
// the shipped stated-404 arm verbatim (edit's banner-over-truth, mirrored
// status): the move never applied, and the card ui/11 surface will own the
// consolidated statement later. Nothing moved.
func TestDragMoveOfUnknownCardStatesMissing(t *testing.T) {
	store, uiSrv, log := moveChain(t, "Still here")

	status, frag := dragMove(t, uiSrv.URL, 999, "In Progress", 0)
	if status != http.StatusNotFound {
		t.Fatalf("PATCH /ui/cards/999/move status = %d, want 404 (body %s)", status, frag)
	}
	if len(log.patches()) != 1 {
		t.Fatalf("the contract saw %v, want exactly one attempted move", log.patches())
	}
	if !strings.Contains(frag, `class="banner" role="alert">no such card<`) {
		t.Errorf("stale move did not state the failure:\n%s", frag)
	}
	if strings.Contains(frag, `id="card-999"`) || !strings.Contains(frag, "Still here") {
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
