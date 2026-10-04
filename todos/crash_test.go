package todos

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

// Card todos/14 — Completed operation survives process death.
// Given Create returns successfully
// When  the process is killed immediately after and restarted on the same file
// Then  the todo is present
//
// The kill is genuine, not a Close: a child copy of this test binary performs
// Create and Change operations through the store, reports success on stdout,
// then SIGKILLs itself — no Close, no deferred cleanup, no graceful exit path
// ever runs. SQLite's committed transactions are the durability mechanism the
// card names: everything the child saw return successfully must be present
// when a fresh store opens the same file.
func TestCompletedOperationSurvivesProcessDeath(t *testing.T) {
	if helper := os.Getenv("TODO_CRASH_HELPER"); helper == "1" {
		crashHelper()
		return
	}

	dbPath := t.TempDir() + "/crash.db"

	child := exec.Command(os.Args[0], "-test.run=TestCompletedOperationSurvivesProcessDeath")
	child.Env = append(os.Environ(), "TODO_CRASH_HELPER=1", "TODO_CRASH_DB="+dbPath)
	out, err := child.CombinedOutput()

	// The child must have died BY the kill: SIGKILL, never a clean exit.
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("child should have died abnormally, err=%v output=%s", err, out)
	}
	ws, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
		t.Fatalf("child must die by SIGKILL, state=%v output=%s", exitErr.ProcessState, out)
	}
	// Proof the operations completed (returned successfully) before the kill.
	if !strings.Contains(string(out), "crash-helper: operations complete") {
		t.Fatalf("child never reported completed operations before dying: %s", out)
	}

	// Restart on the same file: every completed operation is present.
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("reopen after kill: %v", err)
	}
	defer store.Close()

	list, err := store.List()
	if err != nil {
		t.Fatalf("List after kill: %v", err)
	}

	want := map[string]Todo{
		"survivor":   {ID: 1, Title: "survivor", Done: true},    // Create + Change done
		"second one": {ID: 2, Title: "second one", Done: false}, // Create only
	}
	if len(list) != len(want) {
		t.Fatalf("List after kill = %+v, want the %d completed todos", list, len(want))
	}
	for _, got := range list {
		w, known := want[got.Title]
		if !known {
			t.Fatalf("unexpected todo after kill: %+v", got)
		}
		if got != w {
			t.Fatalf("todo %q lost state across the kill: got %+v want %+v", got.Title, got, w)
		}
	}
}

// crashHelper runs inside the child process: performs store operations, then
// kills the process without any close. It never returns.
func crashHelper() {
	store, err := Open(os.Getenv("TODO_CRASH_DB"))
	if err != nil {
		panic(err)
	}
	created, err := store.Create("survivor")
	if err != nil {
		panic(err)
	}
	done := true
	if _, err := store.Change(created.ID, ChangeFields{Done: &done}); err != nil {
		panic(err)
	}
	if _, err := store.Create("second one"); err != nil {
		panic(err)
	}
	// Operations have returned — their writes are committed. Now die like a
	// crash: SIGKILL to self, so no Close and no deferred cleanup can run.
	fmt.Println("crash-helper: operations complete")
	if err := syscall.Kill(os.Getpid(), syscall.SIGKILL); err != nil {
		panic(err)
	}
	for {
		syscall.Pause()
	}
}
