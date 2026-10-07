package main

import (
	"io"
	"net/http"
	"path/filepath"
	"testing"
)

// Card api/12 (composition arm) — GET /users is mounted on the composed
// listener. The api/12 endpoint exists on the api handler, but the composed
// server reaches it only through its route line (main.go:
// mux.Handle("/users", apiHandler)); without that line the roster would 404
// through the page fallback, exactly the gap this mirrors for /cards at
// ui/04. api/users_test.go pins the endpoint in-process; this leg pins the
// composition.
// Given a freshly started server
// When  GET /users arrives on the composed listener
// Then  the JSON contract answers it (200, not 404) with the roster body
//
//	byte-exact — the five names in the one fixed display order
func TestUsersEndpointMounted(t *testing.T) {
	dir := t.TempDir()
	addr := freeAddr(t)
	startServer(t, addr, filepath.Join(dir, "kanban.db"))

	resp, err := http.Get("http://" + addr + "/users")
	if err != nil {
		t.Fatalf("GET /users: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET /users body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /users status = %d, want 200 (is /users mounted on the api handler? body %s)", resp.StatusCode, body)
	}

	// Byte-exact wire body, the same spelling api/users_test.go pins: the
	// encoder's newline included. A route to anything but the api handler
	// (the page fallback would answer HTML) turns this red.
	const wantBody = `{"users":["Ada","Grace","Alan","Barbara","Linus"]}` + "\n"
	if string(body) != wantBody {
		t.Errorf("GET /users body = %q, want %q", string(body), wantBody)
	}
}
