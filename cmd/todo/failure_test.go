package main

import (
	"context"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runExpectFailure runs the built command, requires a non-zero exit with a
// stated reason, and returns stderr.
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

// Card server/05 — Unusable configuration fails loudly.
// Given the listen address is already in use or the data path cannot be opened
// When  the command is started
// Then  it exits with a non-zero code and a stated reason
//
//	And no half-wired server is left listening
func TestAddressInUseFailsLoudly(t *testing.T) {
	ln, err := net.Listen("tcp", freeAddr(t))
	if err != nil {
		t.Fatalf("occupy address: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	out := runExpectFailure(t, "--addr", addr, "--db", filepath.Join(t.TempDir(), "x.db"),
		"--board-db", filepath.Join(t.TempDir(), "y.db"))
	t.Logf("stated reason: %s", out)
}

func TestUnusableDataPathFailsLoudly(t *testing.T) {
	addr := freeAddr(t)
	badPath := filepath.Join(t.TempDir(), "no-such-dir", "todo.db")

	out := runExpectFailure(t, "--addr", addr, "--db", badPath)
	t.Logf("stated reason: %s", out)

	if dialable(t, addr) {
		t.Errorf("address %s is listening despite startup failure", addr)
	}
}

func TestMalformedFlagValuesFailLoudly(t *testing.T) {
	// Port out of range — a malformed --addr value.
	out := runExpectFailure(t, "--addr", "127.0.0.1:99999", "--db", filepath.Join(t.TempDir(), "x.db"),
		"--board-db", filepath.Join(t.TempDir(), "y.db"))
	t.Logf("stated reason: %s", out)

	// Unknown flag.
	out = runExpectFailure(t, "--bogus")
	t.Logf("stated reason: %s", out)
}
