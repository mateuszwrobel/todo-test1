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

	"todo/board"
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

// startServer runs the built command with the given listen address and
// board data file path, and waits until it answers on /board. The board
// path is always caller-supplied and a temp path: leaving the flag out
// would write the kanban.db default into the package dir. (The --db flag
// retired with the todo store at api/10 — the board file is the only data
// file the command opens.)
func startServer(t *testing.T, addr, boardDBPath string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(buildBinary(t), "--addr", addr, "--board-db", boardDBPath)
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
		resp, err := http.Get("http://" + addr + "/board")
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

// boardColumn/boardPayload decode the GET /board wire body. The test package
// mirrors the contract's shape locally rather than importing api — the same
// own-shape convention the ui module follows toward the contract.
type boardColumn struct {
	Title string       `json:"title"`
	Cards []board.Card `json:"cards"`
}

type boardPayload struct {
	Columns []boardColumn `json:"columns"`
}

// wantColumnTitles is the contract's fixed column set in board order.
var wantColumnTitles = []string{"To Do", "In Progress", "Done"}

// getBoard reads GET /board and requires 200 with a decodable body.
func getBoard(t *testing.T, addr string) boardPayload {
	t.Helper()
	resp, err := http.Get("http://" + addr + "/board")
	if err != nil {
		t.Fatalf("GET /board: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /board status = %d, want 200 (%s)", resp.StatusCode, body)
	}
	var got boardPayload
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("GET /board body %s: %v", body, err)
	}
	return got
}

// Card server/01 — Start serves board and page.
// Given a fresh machine state — no board data file
// When  the command is started
// Then  GET /board answers 200 with the three fixed columns
//
//	And the page route serves the app page from the same process
//
// The todo surface is fully retired (api/01, api/02, api/05, api/10) — the
// command opens one data file and mounts only the board contract; the
// startup-with-no-data-file behavior the old todo-flavored fresh state pinned
// now belongs to the board store alone.
func TestStartServesBothSurfaces(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.db") // absent at start
	addr := freeAddr(t)
	startServer(t, addr, boardPath)

	// JSON contract: the board read answers the three fixed columns; on a
	// fresh machine state every column is empty.
	got := getBoard(t, addr)
	if len(got.Columns) != len(wantColumnTitles) {
		t.Fatalf("GET /board columns = %d, want %d: %+v", len(got.Columns), len(wantColumnTitles), got.Columns)
	}
	for i, want := range wantColumnTitles {
		if got.Columns[i].Title != want {
			t.Errorf("column %d title = %q, want %q", i, got.Columns[i].Title, want)
		}
		if len(got.Columns[i].Cards) != 0 {
			t.Errorf("column %q on a fresh machine state holds %d cards, want 0",
				got.Columns[i].Title, len(got.Columns[i].Cards))
		}
	}

	// Page from the same process: the app page is served on GET /.
	resp, err := http.Get("http://" + addr + "/")
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
	if len(page) == 0 {
		t.Error("GET / served an empty body")
	}
}
