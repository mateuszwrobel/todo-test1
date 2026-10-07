package ui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"todo/api"
	"todo/board"
)

// filter_test.go pins the KW10 filter surface (cards ui/16–18): the filter
// dropdown the page chrome renders, the ?assignee= parameter the ui seam
// honors on its own GET / (mirroring the api's presence branch), the
// filter-naming per-column empty, and the drop-under-filter's ONE slot+within
// PATCH. The browser gestures — opening the dropdown, dragging — are the
// wave-end e2e lane's; what lives here is everything the module owns: the
// rendered chrome, the URL→contract translation, the stated surfaces, and the
// resolved whole-board truth underneath a filtered view.

// filterServers names the two servers a filter leg stands on: api is the real
// contract, page is the ui module, both over real HTTP against one store.
type filterServers struct {
	apiURL string
	page   string
}

// boardReadLog records every board READ crossing the ui→api boundary, so a
// test can pin not only what rendered but WHAT WAS ASKED FOR: an unfiltered
// load must cross as the plain byte-frozen GET /board, a filtered one as the
// same GET carrying the URL-encoded assignee parameter.
type boardReadLog struct {
	mu    sync.Mutex
	reads []string // one "METHOD path[?rawquery]" per GET /board crossing
}

func (l *boardReadLog) record(read string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reads = append(l.reads, read)
}

func (l *boardReadLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.reads...)
}

// filterChain is the real chain — real store, real api handler, real HTTP —
// wrapped so BOTH the board reads and the card writes are observed. moveChain
// (move_test.go) watches only the writes; the filter legs also need to see
// what the ui module asked the contract for.
func filterChain(t *testing.T, seeds ...string) (*board.Store, filterServers, *boardReadLog, *contractCallLog) {
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
	reads, writes := new(boardReadLog), new(contractCallLog)
	handler := api.NewHandler(store).ServeHTTP
	apiSrv := fakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/board" {
			crossing := r.Method + " " + r.URL.Path
			if r.URL.RawQuery != "" {
				crossing += "?" + r.URL.RawQuery
			}
			reads.record(crossing)
		}
		if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/cards/") {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			writes.record(r.Method + " " + r.URL.Path + " " + string(body))
		}
		handler(w, r)
	})
	servers := filterServers{apiURL: apiSrv.URL, page: uiServer(t, apiSrv.URL).URL}
	return store, servers, reads, writes
}

// dropPairPayload sends what ONE accepted drop sends while the URL carries a
// filter: a single PATCH to the ui move endpoint carrying the target column
// and the slot+within pair instead of an absolute position — exactly the
// payload the shell's one request site builds (card ui/18).
func dropPairPayload(t *testing.T, pageURL string, id int64, column string, slot int, within string) (int, string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"column": column, "slot": slot, "within": within})
	if err != nil {
		t.Fatalf("marshal pair payload: %v", err)
	}
	target := pageURL + "/ui/cards/" + strconv.FormatInt(id, 10) + "/move"
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

// assertOnePairRequest pins the pair's one-request rule on the recorder:
// exactly one PATCH crossed, with the contract's slot+within body and no
// position key beside it.
func assertOnePairRequest(t *testing.T, log *contractCallLog, id int64, columnKey string, slot int, within string) {
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
		t.Errorf("pair body column = %v, want the contract key %q", body["column"], columnKey)
	}
	if s, ok := body["slot"].(float64); !ok || int(s) != slot {
		t.Errorf("pair body slot = %v, want %d", body["slot"], slot)
	}
	if body["within"] != within {
		t.Errorf("pair body within = %v, want %q", body["within"], within)
	}
	if _, present := body["position"]; present {
		t.Errorf("pair body carries position beside the slot: %v", body)
	}
}

// fullBoard asks the contract for the whole board — the full-board GET that
// clears the filter and reveals the truth underneath.
func fullBoard(t *testing.T, apiURL string) boardResponse {
	t.Helper()
	status, body := getPage(t, apiURL+"/board")
	if status != http.StatusOK {
		t.Fatalf("full-board GET status = %d, want 200", status)
	}
	var board boardResponse
	if err := json.Unmarshal([]byte(body), &board); err != nil {
		t.Fatalf("full board body does not parse: %v\n%s", err, body)
	}
	return board
}

// columnCards names one column's cards as the full-board GET answers them.
func columnCards(t *testing.T, board boardResponse, title string) []boardCardResponse {
	t.Helper()
	for _, col := range board.Columns {
		if col.Title == title {
			return col.Cards
		}
	}
	t.Fatalf("full board has no %q column: %+v", title, board.Columns)
	return nil
}

// assignTo puts an assignee on a seeded card straight through the store —
// the board's own truth for the state the filter cuts along.
func assignTo(t *testing.T, store *board.Store, id int64, name string) {
	t.Helper()
	if _, err := store.Change(id, nil, nil, board.AssignTo(name)); err != nil {
		t.Fatalf("seed assign %q to card %d: %v", name, id, err)
	}
}

// Card ui/16 — Filter dropdown filters the board (chrome half).
// Given the board page
// When  the filter control is opened
// Then  it lists "All users", each roster name, and "Unassigned" — exactly
//
//	the roster
func TestFilterDropdownListsAllUsersRosterUnassigned(t *testing.T) {
	apiSrv := boardAPI(t)
	status, page := getPage(t, uiServer(t, apiSrv.URL).URL+"/")
	if status != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", status)
	}
	selectStart := strings.Index(page, `<select id="board-filter"`)
	if selectStart < 0 {
		t.Fatalf("page chrome has no filter dropdown:\n%s", page)
	}
	selectEnd := strings.Index(page[selectStart:], `</select>`)
	selectHTML := page[selectStart : selectStart+selectEnd]

	// The option order is contract: All users first, the roster in the order
	// the Roster port answers, Unassigned (the keyword sentinel) last.
	want := []string{
		`<option value="" selected>All users</option>`,
		`<option value="Ada">Ada</option>`,
		`<option value="Grace">Grace</option>`,
		`<option value="Alan">Alan</option>`,
		`<option value="Barbara">Barbara</option>`,
		`<option value="Linus">Linus</option>`,
		`<option value="unassigned">Unassigned</option>`,
	}
	prev := -1
	for _, option := range want {
		at := strings.Index(selectHTML, option)
		if at < 0 {
			t.Fatalf("dropdown is missing option %q:\n%s", option, selectHTML)
		}
		if at < prev {
			t.Errorf("option %q out of order:\n%s", option, selectHTML)
		}
		prev = at
	}
}

// Card ui/16 — the current selection rides the URL: unfiltered marks All
// users, ?assignee=<name> marks that name, ?assignee=unassigned marks
// Unassigned. The chrome renders from the address, never from page state.
func TestFilterDropdownSelectionReflectsURL(t *testing.T) {
	apiSrv := boardAPI(t)
	uiURL := uiServer(t, apiSrv.URL).URL
	for _, leg := range []struct {
		url      string
		selected string
	}{
		{"/", `<option value="" selected>All users</option>`},
		{"/?assignee=Grace", `<option value="Grace" selected>Grace</option>`},
		{"/?assignee=unassigned", `<option value="unassigned" selected>Unassigned</option>`},
	} {
		status, page := getPage(t, uiURL+leg.url)
		if status != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", leg.url, status)
		}
		if !strings.Contains(page, leg.selected) {
			t.Errorf("GET %s does not mark %q selected:\n%s", leg.url, leg.selected, page)
		}
	}
}

// Card ui/16 — choosing a value pushes the URL. The page keeps no filter
// state: the dropdown's wiring is a plain GET navigation to the address the
// server renders from — ?assignee=<URL-encoded value>, and the PLAIN '/' for
// All users so the parameter leaves the URL. Back and reload replay exactly
// what the address says; the browser-side click is the e2e lane's leg.
func TestFilterNavigationCarriesURLEncodedParam(t *testing.T) {
	apiSrv := boardAPI(t)
	_, page := getPage(t, uiServer(t, apiSrv.URL).URL+"/")
	if !strings.Contains(page, `window.location = select.value`) {
		t.Error("dropdown wiring does not navigate (pushes a history entry): " +
			"no window.location assignment for the select's change")
	}
	if !strings.Contains(page, `'/?assignee=' + encodeURIComponent(select.value)`) {
		t.Error(`choosing a value must navigate to '/?assignee=' + encodeURIComponent(value) — the filter travels URL-encoded`)
	}
	if !strings.Contains(page, `: '/'`) {
		t.Error("All users must navigate to the plain '/' so the parameter leaves the URL")
	}
}

// Card ui/16 — the server renders the filtered view FROM THE URL: a reload
// of the filtered address keeps the filtered board (the ui seam honors
// ?assignee= on its own GET / by mirroring the api's presence branch), and
// the unfiltered load crosses to the contract as the byte-frozen plain
// read with no query at all.
func TestFilteredBoardRendersFromURL(t *testing.T) {
	store, servers, reads, _ := filterChain(t, "Note for Grace", "Note for no one", "Another for Grace")
	assignTo(t, store, 1, "Grace")
	assignTo(t, store, 3, "Grace")

	status, page := getPage(t, servers.page+"/?assignee=Grace")
	if status != http.StatusOK {
		t.Fatalf("GET /?assignee=Grace status = %d, want 200", status)
	}
	// Only Grace's cards render, in the server's order; the hidden card is
	// simply absent — the filter cut cells, the page renders what came.
	for _, id := range []int64{1, 3} {
		if !strings.Contains(page, `id="card-`+strconv.FormatInt(id, 10)+`"`) {
			t.Errorf("filtered view is missing Grace card %d:\n%s", id, page)
		}
	}
	if strings.Contains(page, `id="card-2"`) {
		t.Errorf("filtered view renders the unassigned card:\n%s", page)
	}
	// The read that produced this view crossed as the parameterized GET.
	sawParam := false
	for _, read := range reads.all() {
		if strings.Contains(read, "?assignee=Grace") {
			sawParam = true
		}
	}
	if !sawParam {
		t.Errorf("no filtered read crossed the contract: %q", reads.all())
	}

	// The unfiltered load is the frozen leg: plain GET /board, no query.
	before := len(reads.all())
	_, _ = getPage(t, servers.page+"/")
	plain := reads.all()[before]
	if plain != "GET /board" {
		t.Errorf("unfiltered read crossed as %q, want the byte-frozen plain read %q", plain, "GET /board")
	}
}

// Card ui/16 + the seam's failure surface: a filter the contract does not
// know fails the read, and a failed read is stated the one stated way a
// failed read is stated — the load-failure surface (failedTmpl), never an
// empty board standing in for the truth, never a second message site.
func TestUnknownFilterStatesTheLoadFailure(t *testing.T) {
	_, servers, _, _ := filterChain(t, "Some card")
	status, page := getPage(t, servers.page+"/?assignee=Zoe")
	if status != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (the failure is stated in the page)", status)
	}
	if !strings.Contains(page, `id="load-error"`) || !strings.Contains(page, "Could not load board") {
		t.Errorf("unknown filter does not state the load failure:\n%s", page)
	}
	// Not columns standing in for a truth the server refused: no board
	// surface renders beside the failure.
	if strings.Contains(page, `id="board"`) {
		t.Errorf("unknown filter renders a board beside the failure:\n%s", page)
	}
}

// Card ui/17 — Filtered columns state the empty: under a filter that leaves
// one column with no matching cards, that column shows the stated empty
// treatment WITH THE FILTER NAMED, in the same column__empty markup — and
// the all-empty board's unfiltered "No cards" stays exactly as shipped.
func TestFilteredColumnsStateTheEmpty(t *testing.T) {
	store, servers, _, _ := filterChain(t, "Note for Grace")
	assignTo(t, store, 1, "Grace")
	status, page := getPage(t, servers.page+"/?assignee=Grace")
	if status != http.StatusOK {
		t.Fatalf("GET /?assignee=Grace status = %d, want 200", status)
	}
	// To Do holds Grace's card; the other two columns name the filter.
	todo := columnHTML(t, page, "to-do")
	if strings.Contains(todo, "Nothing for Grace here") {
		t.Errorf("the populated column states an empty:\n%s", todo)
	}
	for _, anchor := range []string{"in-progress", "done"} {
		col := columnHTML(t, page, anchor)
		if !strings.Contains(col, `class="column__empty"`) || !strings.Contains(col, "Nothing for Grace here") {
			t.Errorf("empty column #%s does not state the filter-named empty:\n%s", anchor, col)
		}
	}
	// Distinct from the all-empty board's wording.
	if strings.Contains(page, "No cards") {
		t.Errorf("filtered empties reuse the unfiltered wording:\n%s", page)
	}
}

// Card ui/17 for the sentinel leg: the unassigned filter's stated empty
// names unassignment, not a person ("Nothing unassigned here" — pinned).
func TestUnassignedFilterStatesItsEmpty(t *testing.T) {
	store, servers, _, _ := filterChain(t, "Note for Grace")
	assignTo(t, store, 1, "Grace")
	status, page := getPage(t, servers.page+"/?assignee=unassigned")
	if status != http.StatusOK {
		t.Fatalf("GET /?assignee=unassigned status = %d, want 200", status)
	}
	for _, anchor := range []string{"to-do", "in-progress", "done"} {
		col := columnHTML(t, page, anchor)
		if !strings.Contains(col, "Nothing unassigned here") {
			t.Errorf("empty column #%s does not state the unassigned empty:\n%s", anchor, col)
		}
	}
	if strings.Contains(page, "No cards") || strings.Contains(page, "Nothing for") {
		t.Errorf("unassigned filter reuses another empty wording:\n%s", page)
	}
}

// Card ui/18 (request half) — under an active filter the drop issues ONE
// PATCH carrying the slot+within pair instead of an absolute position, and
// the answer stays the FILTERED view: the moved card among the visible ones,
// the hidden cards absent, the no-match column naming the filter.
func TestDragUnderFilterIssuesOneSlotWithinPatch(t *testing.T) {
	store, servers, _, log := filterChain(t,
		"Grace one", "hidden one", "Grace two", "moving card")
	assignTo(t, store, 1, "Grace")
	assignTo(t, store, 3, "Grace")
	assignTo(t, store, 4, "Grace") // the drag source must itself be visible under the filter
	moveTo(t, store, 1, board.InProgress)
	moveTo(t, store, 3, board.InProgress)

	status, frag := dropPairPayload(t, servers.page, 4, "In Progress", 1, "Grace")
	if status != http.StatusOK {
		t.Fatalf("pair drop status = %d, want 200 (body %s)", status, frag)
	}
	assertOnePairRequest(t, log, 4, "in_progress", 1, "Grace")

	// The answer is the filtered view the user is looking at: Grace's cards
	// only — the dragged card lands at the visible slot between Grace's two —
	// and the hidden card never surfaces.
	if strings.Contains(frag, `id="card-2"`) {
		t.Errorf("filtered drop answer reveals the hidden card:\n%s", frag)
	}
	atOne := strings.Index(frag, `id="card-1"`)
	atMoved := strings.Index(frag, `id="card-4"`)
	atTwo := strings.Index(frag, `id="card-3"`)
	if !(atOne < atMoved && atMoved < atTwo) {
		t.Errorf("filtered answer does not show the card at the visible slot (between Grace's):\n%s", frag)
	}
	if !strings.Contains(frag, "Nothing for Grace here") {
		t.Errorf("filtered drop answer states the unfiltered empty:\n%s", frag)
	}
}

// Card ui/18 (truth half) — hidden truth under a filter: with hidden cards
// interleaved among the visible ones, the pair resolves to the earliest
// whole-board position matching the visible slot, and the full-board GET
// shows the hidden cards UNTOUCHED between the visible ones.
func TestHiddenTruthUnderFilterKeepsHiddenCards(t *testing.T) {
	store, servers, _, log := filterChain(t,
		"hidden a", "Grace b", "hidden c", "Grace d", "moving e")
	// In Progress takes an interleaved cast:
	// In Progress = hidden a, Grace b, hidden c, Grace d (absolute 1..4);
	// To Do keeps moving e, assigned Grace (the only visible drag source).
	moveTo(t, store, 1, board.InProgress)
	moveTo(t, store, 2, board.InProgress)
	moveTo(t, store, 3, board.InProgress)
	moveTo(t, store, 4, board.InProgress)
	assignTo(t, store, 2, "Grace")
	assignTo(t, store, 4, "Grace")
	assignTo(t, store, 5, "Grace")

	// The filtered view the drag happens in: visible = b, d (slots 0, 1) in
	// In Progress and e in To Do; the hidden a, c are not on the page at all.
	_, page := getPage(t, servers.page+"/?assignee=Grace")
	for _, hidden := range []int64{1, 3} {
		if strings.Contains(page, `id="card-`+strconv.FormatInt(hidden, 10)+`"`) {
			t.Fatalf("filtered render shows hidden card %d:\n%s", hidden, page)
		}
	}

	// The user drags e into In Progress at the visible slot 1 — the page's
	// one request carries the pair counted among the VISIBLE cards.
	status, frag := dropPairPayload(t, servers.page, 5, "In Progress", 1, "Grace")
	if status != http.StatusOK {
		t.Fatalf("pair drop status = %d, want 200 (body %s)", status, frag)
	}
	assertOnePairRequest(t, log, 5, "in_progress", 1, "Grace")

	// The view updates as moved: b, e, d visible, in that order.
	atB := strings.Index(frag, `id="card-2"`)
	atE := strings.Index(frag, `id="card-5"`)
	atD := strings.Index(frag, `id="card-4"`)
	if !(atB < atE && atE < atD) {
		t.Errorf("filtered view did not update as moved:\n%s", frag)
	}

	// Clearing the filter — the full-board GET — reveals the true interleaved
	// order with the hidden cards untouched: e landed between b and c (the
	// earliest absolute index with exactly one Grace card before it), and
	// a, c never moved.
	inProgress := columnCards(t, fullBoard(t, servers.apiURL), "In Progress")
	var order []int64
	for _, c := range inProgress {
		order = append(order, c.ID)
	}
	want := []int64{1, 2, 5, 3, 4}
	if len(order) != len(want) {
		t.Fatalf("In Progress order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("In Progress order = %v, want %v (hidden cards untouched)", order, want)
		}
	}
	for _, c := range inProgress {
		hiddenCard := c.ID == 1 || c.ID == 3
		if hiddenCard && c.Assignee != "" {
			t.Errorf("hidden card %d gained an assignee: %+v", c.ID, c)
		}
		if !hiddenCard && c.Assignee != "Grace" {
			t.Errorf("visible card %d lost its assignee: %+v", c.ID, c)
		}
	}
}
