package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Card api/14 — Board filtered by assignee.
// Source scenario, quoted:
//
//	Given a board mixing assigned and unassigned cards
//	When  GET /board carries ?assignee=Grace
//	Then  200 answers the same board shape with only Grace's cards, stored
//	      order and positions unchanged
//	When  it carries ?assignee=unassigned
//	Then  only the cards without an assignee appear
//	When  it carries an assignee value outside the roster
//	Then  422 answers with {"error":"unknown user"}
//	When  it carries no assignee parameter
//	Then  the answer is the full board, unchanged from today
//
// The fixture is one board serving all four clauses: three todo cards with
// Grace's sitting at positions 0 and 2 AROUND the unassigned card at
// position 1 — so a filtered answer that rewrote positions would show. The
// full-board payload is byte-pinned once and reused as the no-param
// expectation and the unchanged-after-refusal probe (frozen leg: the
// absent-parameter answer is this increment's non-regression proof).

// filterFixtureBoard: id 1 "one" (Grace, todo 0), id 2 "two" (unassigned,
// todo 1), id 3 "three" (Grace, todo 2); in_progress and done empty.
func filterFixtureBoard(t *testing.T) (string, string) {
	t.Helper()
	store := openBoardStore(t)
	srv := httptest.NewServer(NewHandler(store))
	t.Cleanup(srv.Close)
	for _, title := range []string{"one", "two", "three"} {
		createCardThroughAPI(t, srv.URL, title)
	}
	for _, id := range []int64{1, 3} {
		if resp, body := patchCard(t, srv.URL, id, `{"assignee": "Grace"}`); resp.StatusCode != http.StatusOK {
			t.Fatalf("assign %d: status %d body %s", id, resp.StatusCode, body)
		}
	}
	const fullBody = `{"columns":[` +
		`{"title":"To Do","cards":[` +
		`{"id":1,"title":"one","column":"todo","position":0,"assignee":"Grace"},` +
		`{"id":2,"title":"two","column":"todo","position":1,"assignee":null},` +
		`{"id":3,"title":"three","column":"todo","position":2,"assignee":"Grace"}]},` +
		`{"title":"In Progress","cards":[]},` +
		`{"title":"Done","cards":[]}]}` + "\n"
	return fullBody, srv.URL
}

func getBoard(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(body)
}

// "Then 200 answers the same board shape with only Grace's cards, stored
// order and positions unchanged" — byte-pinned: Grace's cells keep positions
// 0 and 2 with the hidden position-1 cell simply absent (the stored absolute
// positions pass through, gaps included), the three-column shape and display
// titles intact.
func TestGetBoardFilteredByGraceAnswersOnlyHerCells(t *testing.T) {
	_, h := filterFixtureBoard(t)

	status, body := getBoard(t, h+"/board?assignee=Grace")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", status, body)
	}
	const wantBody = `{"columns":[` +
		`{"title":"To Do","cards":[` +
		`{"id":1,"title":"one","column":"todo","position":0,"assignee":"Grace"},` +
		`{"id":3,"title":"three","column":"todo","position":2,"assignee":"Grace"}]},` +
		`{"title":"In Progress","cards":[]},` +
		`{"title":"Done","cards":[]}]}` + "\n"
	if body != wantBody {
		t.Errorf("body = %s, want exactly %s — Grace's cells at stored positions, other columns preserved empty", body, wantBody)
	}
}

// "Then only the cards without an assignee appear" — the sentinel keyword.
func TestGetBoardFilteredByUnassignedKeyword(t *testing.T) {
	_, h := filterFixtureBoard(t)

	status, body := getBoard(t, h+"/board?assignee=unassigned")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", status, body)
	}
	const wantBody = `{"columns":[` +
		`{"title":"To Do","cards":[` +
		`{"id":2,"title":"two","column":"todo","position":1,"assignee":null}]},` +
		`{"title":"In Progress","cards":[]},` +
		`{"title":"Done","cards":[]}]}` + "\n"
	if body != wantBody {
		t.Errorf("body = %s, want exactly %s", body, wantBody)
	}
}

// "When it carries an assignee value outside the roster / Then 422 answers
// with {"error":"unknown user"}" — the same stated message site as the
// PATCH legs (errorJSON), and the board is byte-unchanged after each.
func TestGetBoardUnknownAssigneeIsAStated422(t *testing.T) {
	full, h := filterFixtureBoard(t)

	const wantErr = `{"error":"unknown user"}` + "\n"
	for _, value := range []string{"Zoe", "grace", "GRACE", "Ada+", "Unassigned", "unassigned%20", ""} {
		status, body := getBoard(t, h+"/board?assignee="+value)
		if status != http.StatusUnprocessableEntity || body != wantErr {
			t.Errorf("?assignee=%q -> status %d body %s, want 422 exactly %s", value, status, body, wantErr)
		}
	}
	// Every refusal was a pure refusal: the full-board read is byte-exact.
	if status, body := getBoard(t, h+"/board"); status != http.StatusOK || body != full {
		t.Errorf("board after refusals: status %d body %s, want 200 exactly %s", status, body, full)
	}
}

// "When it carries no assignee parameter / Then the answer is the full
// board, unchanged from today" — the frozen leg, byte-identical payload.
func TestGetBoardWithoutAssigneeIsTheFullBoardUnchanged(t *testing.T) {
	full, h := filterFixtureBoard(t)

	status, body := getBoard(t, h+"/board")
	if status != http.StatusOK || body != full {
		t.Errorf("status %d body %s, want 200 exactly %s — no-param payload frozen", status, body, full)
	}
}

// The filtered read's store-failure leg: same 500 convention as List's.
func TestGetBoardFilteredStoreFailureIs500(t *testing.T) {
	store := openBoardStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("closing the store to force the failure path: %v", err)
	}
	srv := httptest.NewServer(NewHandler(store))
	defer srv.Close()

	status, _ := getBoard(t, srv.URL+"/board?assignee=Grace")
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (ListFiltered failure maps to the store-failure status)", status)
	}
}
