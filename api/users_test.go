package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Card api/12 (workplans/scenarios-kanban/api/12-users-roster-contract.md) —
// GET /users is the roster contract, byte-for-byte.
// Given the server is running
// When GET /users is requested
// Then 200 answers with the five roster names in fixed order
//
// The body is byte-pinned: the five names in the contract's display order
// (workplan_api_board.md Contracts added 2026-10-07: exactly
// {"users":["Ada","Grace","Alan","Barbara","Linus"]}). The order is contract
// because every dropdown lists the cast in this one order, so the pin checks
// the sequence as written, not a set — a reordering is a contract change and
// must turn this leg red. The served names come from users.Names, so the pin
// also proves the endpoint states the roster itself owns rather than a copy
// that could drift.
func TestGetUsersAnswersRosterInFixedOrder(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openBoardStore(t)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/users")
	if err != nil {
		t.Fatalf("GET /users: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	t.Logf("GET /users body: %s", body)

	const wantBody = `{"users":["Ada","Grace","Alan","Barbara","Linus"]}` + "\n"
	if string(body) != wantBody {
		t.Errorf("body = %s, want exactly the five roster names in contract order: %s", body, wantBody)
	}
	if got, want := rawKeys(body), []string{"users"}; !equalKeys(got, want) {
		t.Errorf("top-level keys = %v, want exactly [users] (body %s)", got, body)
	}
}

// The order is fixed, not incidental: a second request answers the identical
// bytes. users.Names copies its list per call, so no caller can have sorted
// or shuffled what this endpoint states — the repetition is the contract
// (workplan_users_roster.md: the same order, every time).
// Given the roster endpoint answered once
// When GET /users is requested again
// Then the answer is byte-identical
func TestGetUsersRosterOrderIsFixedAcrossRequests(t *testing.T) {
	srv := httptest.NewServer(NewHandler(openBoardStore(t)))
	defer srv.Close()

	first, err := http.Get(srv.URL + "/users")
	if err != nil {
		t.Fatalf("first GET /users: %v", err)
	}
	defer first.Body.Close()
	firstBody, err := io.ReadAll(first.Body)
	if err != nil {
		t.Fatalf("read first body: %v", err)
	}

	second, err := http.Get(srv.URL + "/users")
	if err != nil {
		t.Fatalf("second GET /users: %v", err)
	}
	defer second.Body.Close()
	secondBody, err := io.ReadAll(second.Body)
	if err != nil {
		t.Fatalf("read second body: %v", err)
	}

	if string(secondBody) != string(firstBody) {
		t.Errorf("second body = %s, want byte-identical to the first (%s) — fixed order, every time", secondBody, firstBody)
	}
}
