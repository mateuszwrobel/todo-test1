package main

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runExpectFailure runs the built command, requires a non-zero exit with a
// stated reason, and returns stderr. The exit itself is the "no partially
// serving process remains" evidence: the command is gone, not limping.
func runExpectFailure(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, buildBinary(t), args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("command %v exited 0, want non-zero\noutput:\n%s", args, out)
	}
	exitCode, ok := cmd.ProcessState.ExitCode(), true
	if !ok || exitCode == 0 {
		t.Fatalf("command %v exit code = %d, want non-zero", args, exitCode)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatalf("command %v failed silently, want a stated reason", args)
	}
	return string(out)
}

func dialable(t *testing.T, addr string) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Card server/06 — Unusable configuration fails loudly.
// Given the configuration names a path that cannot be opened (unreadable
// directory, uncreatable file) — or an address that cannot be used, or a
// flag value that cannot be parsed
// When  the server starts
// Then  startup fails with a stated error
//
//	And no partially serving process remains — the command exits non-zero
//	and leaves nothing listening
//	And a failed data path holds nothing written
//
// Formalized plus one new arm: KW1 already proved these failures in three
// separate tests (TestAddressInUseFailsLoudly, TestUnusableDataPathFailsLoudly,
// TestMalformedFlagValuesFailLoudly under the old server/05 numbering).
// They are merged into this card-shaped test here — same runExpectFailure /
// dialable machinery, nothing duplicated. New arms: the unreadable-directory
// data path, and the nothing-left-behind pins (the uncreatable path gains a
// nothing-written assertion, the data-path arms a nothing-listening one).
// The board data file is the command's only data file (the --db flag and the
// todo store retired at api/10).
func TestUnusableConfigurationFailsLoudly(t *testing.T) {
	t.Run("uncreatable board file path", func(t *testing.T) {
		addr := freeAddr(t)
		missingDir := filepath.Join(t.TempDir(), "no-such-dir") // absent
		badPath := filepath.Join(missingDir, "kanban.db")

		out := runExpectFailure(t, "--addr", addr, "--board-db", badPath)
		t.Logf("stated reason: %s", out)

		if dialable(t, addr) {
			t.Errorf("address %s is listening despite startup failure", addr)
		}
		// Nothing written: the store open fails before any file lands —
		// the parent directory is still absent, so nothing was created
		// along the failed path.
		if _, err := os.Stat(missingDir); !os.IsNotExist(err) {
			t.Errorf("startup wrote along the failed data path: stat(%s) = %v", missingDir, err)
		}
	})

	t.Run("unreadable directory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("permission bits are not enforced for root")
		}
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o000); err != nil {
			t.Fatalf("chmod 000: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) // restore so TempDir cleanup can remove it
		addr := freeAddr(t)

		out := runExpectFailure(t, "--addr", addr, "--board-db", filepath.Join(dir, "kanban.db"))
		t.Logf("stated reason: %s", out)

		if dialable(t, addr) {
			t.Errorf("address %s is listening despite startup failure", addr)
		}
	})

	t.Run("address already in use", func(t *testing.T) {
		ln, err := net.Listen("tcp", freeAddr(t))
		if err != nil {
			t.Fatalf("occupy address: %v", err)
		}
		defer ln.Close()
		addr := ln.Addr().String()

		out := runExpectFailure(t, "--addr", addr, "--board-db", filepath.Join(t.TempDir(), "y.db"))
		t.Logf("stated reason: %s", out)
		// The exited-and-stated failure is the no-half-start evidence:
		// the only listener on that address is this test's own.
	})

	t.Run("malformed flag values", func(t *testing.T) {
		// Port out of range — a malformed --addr value.
		out := runExpectFailure(t, "--addr", "127.0.0.1:99999", "--board-db", filepath.Join(t.TempDir(), "y.db"))
		t.Logf("stated reason: %s", out)

		// Unknown flag.
		out = runExpectFailure(t, "--bogus")
		t.Logf("stated reason: %s", out)
	})
}
