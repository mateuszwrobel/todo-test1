package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"todo/todos"
)

// Card server/04 — Clean shutdown completes in-flight work.
// Given a request is being processed
// When  shutdown is signaled
// Then  in-flight requests finish their responses
//
//	And the store is closed so completed operations are durable
//	And the process exits without error
//
// A real process gets SIGTERM while a PATCH request is genuinely mid-flight:
// the request line and headers are sent, the body arrives only after the
// signal — the handler sits blocked reading r.Body when shutdown starts.
//
// The signal point is gated by a causal barrier, not a sleep: the request
// carries Expect: 100-continue, and net/http sends the 100 Continue interim
// response the instant the handler performs its first body read. Receiving
// it proves the request entered ServeHTTP and is registered ACTIVE in
// Shutdown's accounting — a connection still being read cannot be misread
// as idle and closed under the racy "signal immediately after write" form
// of this test (that race was a verified flake).
func TestCleanShutdownCompletesInFlightWork(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shutdown.db")
	boardPath := filepath.Join(t.TempDir(), "shutdown-board.db")
	seedOne(t, dbPath)

	addr := freeAddr(t)
	srv := startServer(t, addr, dbPath, boardPath)

	// Start a PATCH whose body never finishes: the handler is processing
	// (io.ReadAll on the body) while shutdown is signaled.
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	part := "{\"done\":tr"
	rest := "ue}"
	_, err = io.WriteString(conn, "PATCH /todos/1 HTTP/1.1\r\n"+
		"Host: "+addr+"\r\n"+
		"Content-Type: application/json\r\n"+
		"Expect: 100-continue\r\n"+
		"Content-Length: "+strconv.Itoa(len(part)+len(rest))+"\r\n"+
		"\r\n"+part)
	if err != nil {
		t.Fatalf("send partial request: %v", err)
	}

	// Barrier: wait for the 100 Continue interim response. Its arrival is
	// causally after handler entry (first body read), so the request is
	// active in the server by the time we signal.
	br := bufio.NewReader(conn)
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("no 100 Continue interim response: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // headers of the interim response complete
		}
		if strings.HasPrefix(line, "HTTP/") && !strings.HasPrefix(line, "HTTP/1.1 100") {
			t.Fatalf("unexpected interim response: %q", line)
		}
	}

	// Signal shutdown while the request is observably in flight.
	if err := srv.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}

	// Complete the body: the in-flight handler must still be alive to finish.
	if _, err := io.WriteString(conn, rest); err != nil {
		t.Fatalf("complete request body after signal: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("in-flight request did not get a response: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("in-flight PATCH status = %d (%s), want 200", resp.StatusCode, body)
	}

	// The process exits without error — not killed by the signal.
	waitExitZero(t, srv)

	// The completed operation is durable: reopen proves the store closed
	// after the committed write.
	store, err := todos.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen after shutdown: %v", err)
	}
	defer store.Close()
	list, err := store.List()
	if err != nil {
		t.Fatalf("List after shutdown: %v", err)
	}
	if len(list) != 1 || !list[0].Done {
		t.Fatalf("in-flight change not durable: %+v, want one done todo", list)
	}

	// The listener stopped: nothing accepts on the address anymore.
	if c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond); err == nil {
		c.Close()
		t.Fatalf("listener still accepts on %s after shutdown", addr)
	}
}

// Exit codes contract: SIGINT/SIGTERM after successful start → 0.
func TestCleanShutdownExitCodeZero(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "idle.db")
	boardPath := filepath.Join(t.TempDir(), "idle-board.db")
	seedOne(t, dbPath)

	addr := freeAddr(t)
	srv := startServer(t, addr, dbPath, boardPath)

	if err := srv.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("SIGTERM: %v", err)
	}
	waitExitZero(t, srv)

	if c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond); err == nil {
		c.Close()
		t.Fatalf("listener still accepts on %s after shutdown", addr)
	}
}

// seedOne creates the data file with a single not-done todo, using the store
// directly while no server holds the file (single-process assumption).
func seedOne(t *testing.T, dbPath string) {
	t.Helper()
	store, err := todos.Open(dbPath)
	if err != nil {
		t.Fatalf("seed Open: %v", err)
	}
	if _, err := store.Create("shut down me"); err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("seed Close: %v", err)
	}
}

// waitExitZero waits for the process to exit on its own (bounded) and
// requires exit code 0 — a signal death or a hung drain fails the test.
func waitExitZero(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			var s string
			if out, ok := cmd.Stdout.(*lockedBuffer); ok {
				s = out.String()
			}
			t.Fatalf("process did not exit cleanly: %v\noutput: %s", err, s)
		}
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("process did not exit within the drain bound")
	}
}
