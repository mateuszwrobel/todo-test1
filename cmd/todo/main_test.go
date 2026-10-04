package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"todo/todos"
)

var (
	binPath string
	binErr  error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "todo-bin")
	if err == nil {
		// Tests run with the package dir as cwd; build from the module root.
		_, thisFile, _, _ := runtime.Caller(0)
		repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
		binPath = filepath.Join(dir, "todo-bin")
		build := exec.Command("go", "build", "-o", binPath, "./cmd/todo")
		build.Dir = repoRoot
		out, berr := build.CombinedOutput()
		if berr != nil {
			binErr = berr
			binPath += ": " + string(out)
		}
	}
	code := m.Run()
	if dir != "" {
		os.RemoveAll(dir)
	}
	os.Exit(code)
}

// buildBinary returns the command binary path built once for the run.
func buildBinary(t *testing.T) string {
	t.Helper()
	if binErr != nil || binPath == "" {
		t.Fatalf("build cmd/todo: %v (%s)", binErr, binPath)
	}
	return binPath
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// startServer runs the built command with the given address and db path and
// waits until it answers on /todos.
func startServer(t *testing.T, addr, dbPath string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(buildBinary(t), "--addr", addr, "--db", dbPath)
	out := &lockedBuffer{}
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if t.Failed() {
			t.Logf("server output:\n%s", out.String())
		}
	})

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/todos")
		if err == nil {
			resp.Body.Close()
			return cmd
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("server did not become ready on %s", addr)
	return nil
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Card server/01 — Start serves both surfaces.
// Given a data file path and a listen address
// When  the command is started
// Then  the page is served at GET / on that address
//
//	And the JSON contract is served at /todos on the same address
//	And page operations work end to end
//
// (In W1 the page has no write operations yet; "end to end" here means the
// rendered page reflects real stored data, read through the api contract.)
func TestStartServesBothSurfaces(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "todo.db")
	store, err := todos.Open(dbPath)
	if err != nil {
		t.Fatalf("seed Open: %v", err)
	}
	for _, title := range []string{"write report", "water plants"} {
		if _, err := store.Create(title); err != nil {
			t.Fatalf("seed Create: %v", err)
		}
	}
	store.Close()

	addr := freeAddr(t)
	startServer(t, addr, dbPath)

	// JSON contract at /todos.
	resp, err := http.Get("http://" + addr + "/todos")
	if err != nil {
		t.Fatalf("GET /todos: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /todos status = %d, want 200", resp.StatusCode)
	}
	var got []todos.Todo
	if err := json.Unmarshal(body, &got); err != nil || len(got) != 2 {
		t.Fatalf("GET /todos body = %s (err %v), want 2 todos", body, err)
	}

	// Page at GET / on the same address.
	resp, err = http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("GET / Content-Type = %q, want text/html", ct)
	}
	for _, title := range []string{"write report", "water plants"} {
		if !strings.Contains(string(page), title) {
			t.Errorf("page does not show stored todo %q:\n%s", title, page)
		}
	}
}
